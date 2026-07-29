import assert from "node:assert/strict";
import test from "node:test";
import {
  createContentReviewRun,
  getContentReviewSummary,
  getReview,
  isRealReviewDetail,
  isRealReviewHistoryItem,
  listContentReviewHistory,
  preflightContentReview,
  retryReviewResultConsumption,
  updateReviewIssue,
} from "./content-review-api.ts";

const originalFetch = global.fetch;
test.after(() => {
  global.fetch = originalFetch;
});

const run = {
  id: "run/id",
  runNumber: "RUN-17",
  projectId: "project",
  stage: "review",
  workflowConfigurationId: "configuration",
  triggerSource: "manual",
  status: "queued",
  inputPayload: {},
  outputPayload: null,
  errorCode: null,
  errorMessage: null,
  errorDetails: null,
  configurationSnapshot: {},
  startedAt: null,
  finishedAt: null,
  cancelledAt: null,
  createdAt: "2026-07-29T00:00:00Z",
  updatedAt: "2026-07-29T00:00:00Z",
  version: 3,
};

test("review preflight and create use the frozen version endpoints without creating during preflight", async () => {
  const calls: Array<{ url: string; init?: RequestInit }> = [];
  global.fetch = async (url, init) => {
    calls.push({ url: String(url), init });
    const data = String(url).endsWith("/preflight")
      ? {
          status: "passed",
          checks: [],
          sourceContentVersionSummary: {},
          reviewDimensions: [],
          configurationSummary: null,
          preflightToken: "token-17",
          expiresAt: "2026-07-29T01:00:00Z",
        }
      : run;
    return new Response(JSON.stringify({ data, request_id: "req" }), {
      status: String(url).endsWith("/preflight") ? 200 : 201,
    });
  };

  await preflightContentReview("version/id", {
    sourceContentVersionVersion: 4,
    optionalInstructions: "检查角色设定",
  });
  assert.equal(calls.length, 1);
  assert.match(
    calls[0].url,
    /content-versions\/version%2Fid\/review-runs\/preflight$/,
  );
  assert.deepEqual(JSON.parse(String(calls[0].init?.body)), {
    sourceContentVersionVersion: 4,
    optionalInstructions: "检查角色设定",
  });
  assert.equal(
    (calls[0].init?.headers as Record<string, string>)["Idempotency-Key"],
    undefined,
  );

  await createContentReviewRun(
    "version/id",
    "token-17",
    "create-review-key",
  );
  assert.equal(calls.length, 2);
  assert.match(
    calls[1].url,
    /content-versions\/version%2Fid\/review-runs$/,
  );
  assert.deepEqual(JSON.parse(String(calls[1].init?.body)), {
    preflightToken: "token-17",
  });
  assert.equal(
    (calls[1].init?.headers as Record<string, string>)["Idempotency-Key"],
    "create-review-key",
  );
});

test("summary, history, and detail preserve the frozen persisted recovery endpoints", async () => {
  const calls: string[] = [];
  global.fetch = async (url) => {
    calls.push(String(url));
    const current = String(url);
    const data = current.includes("review-summary")
      ? {
          contentItemId: "item",
          state: "result_consumption_failed",
          canStartReview: false,
          activeRun: null,
          latestRun: run,
          latestReport: null,
          issueSummary: null,
          latestError: null,
          configurationSummary: null,
        }
      : current.includes("/reviews?")
        ? { items: [], total: 0, limit: 20, offset: 0 }
        : {
            report: { id: "report" },
            sourceContentVersionSummary: {},
            issues: [],
            recommendations: [],
            workflowRunSummary: run,
          };
    return new Response(JSON.stringify({ data, request_id: "req" }), {
      status: 200,
    });
  };

  const summary = await getContentReviewSummary("item/id");
  const history = await listContentReviewHistory("item/id");
  const detail = await getReview("review/id");
  assert.equal(summary.state, "result_consumption_failed");
  assert.equal(history.total, 0);
  assert.equal(isRealReviewDetail(detail), true);
  assert.match(calls[0], /content-items\/item%2Fid\/review-summary$/);
  assert.match(calls[1], /content-items\/item%2Fid\/reviews\?limit=20&offset=0$/);
  assert.match(calls[2], /reviews\/review%2Fid$/);
});

test("issue disposition and result-consumption retry send expected versions and independent idempotency keys", async () => {
  const calls: Array<{ url: string; init?: RequestInit }> = [];
  global.fetch = async (url, init) => {
    calls.push({ url: String(url), init });
    const data = String(url).includes("/issues/")
      ? { id: "issue", disposition: "ignored", version: 5 }
      : { report: { id: "report" }, issues: [], recommendations: [], workflowRun: run };
    return new Response(JSON.stringify({ data, request_id: "req" }), {
      status: 200,
    });
  };

  await updateReviewIssue(
    "review/id",
    "issue/id",
    { disposition: "ignored", expectedVersion: 4 },
    "issue-key",
  );
  await retryReviewResultConsumption("run/id", 3, "consume-key");

  assert.match(calls[0].url, /reviews\/review%2Fid\/issues\/issue%2Fid$/);
  assert.deepEqual(JSON.parse(String(calls[0].init?.body)), {
    disposition: "ignored",
    expectedVersion: 4,
  });
  assert.equal(
    (calls[0].init?.headers as Record<string, string>)["Idempotency-Key"],
    "issue-key",
  );
  assert.match(
    calls[1].url,
    /workflow-runs\/run%2Fid\/review-result-consumption-retries$/,
  );
  assert.deepEqual(JSON.parse(String(calls[1].init?.body)), {
    expectedRunVersion: 3,
  });
  assert.equal(
    (calls[1].init?.headers as Record<string, string>)["Idempotency-Key"],
    "consume-key",
  );
});

test("history discriminates runtime rows from compatible P0 mock reports", () => {
  assert.equal(
    isRealReviewHistoryItem({
      workflowRun: run,
      sourceContentVersionSummary: {
        id: "version",
        contentItemId: "item",
        versionNo: 4,
        version: 2,
        title: "正文",
        wordCount: 20,
        contentHash: "a".repeat(64),
        frozen: true,
      },
      reportSummary: null,
    }),
    true,
  );
  assert.equal(
    isRealReviewHistoryItem({
      id: "mock",
      content_item_id: "item",
      content_version_id: "version",
      provider_key: "mock",
      status: "completed",
      conclusion: "pass",
      score: 90,
      summary: "通过",
      created_at: "2026-07-29T00:00:00Z",
    }),
    false,
  );
});
