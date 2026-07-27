import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { ApiError } from "../../lib/api.ts";
import {
  abandonChapterPlanCandidateBatch,
  adoptChapterPlanCandidate,
  adoptChapterPlanCandidates,
  createChapterPlanRun,
  preflightChapterPlanRun,
  recompareChapterPlanCandidate,
  updateChapterPlanCandidate,
  type ChapterPlan,
  type ChapterPlanCandidateBatch,
  type ChapterPlanCandidateSnapshot,
} from "./chapter-plan-http-api.ts";

test("preflight sends frozen full, append, and range targets without legacy fields", async () => {
  const originalFetch = globalThis.fetch;
  const bodies: Record<string, unknown>[] = [];
  globalThis.fetch = (async (_input: RequestInfo | URL, init?: RequestInit) => {
    const body = JSON.parse(String(init?.body ?? "{}")) as Record<string, unknown>;
    bodies.push(body);
    return new Response(JSON.stringify({ data: { result: "blocked", status: "blocked", inputDigest: "digest", inputSummary: { generationMode: body.generationMode, target: { startChapterNo: 1, endChapterNo: 1, requestedChapterCount: 1 }, storylineSelection: { mode: "auto_balanced" }, contextOptions: {} }, executionConfigurationSummary: { stage: "chapter_planning", workflowBindingId: "binding", workflowBindingVersion: 1 }, checks: [], warnings: [], blockers: [{ code: "workflow_not_configured", message: "blocked", severity: "blocker", details: { safeSummary: "safe", action: "configure" } }] }, request_id: "req" }), { status: 200, headers: { "Content-Type": "application/json" } });
  }) as typeof fetch;
  const base = { storylineSelection: { mode: "auto_balanced" as const }, contextOptions: { includeProjectMaterials: true, includeUnpaidForeshadowings: true, includePriorChapterSummaries: true, coreSettingsOnly: false }, additionalInstructions: null };
  try {
    await preflightChapterPlanRun("project", { ...base, generationMode: "full", target: { targetTotalChapters: 12 } });
    await preflightChapterPlanRun("project", { ...base, generationMode: "append", target: { chapterCount: 3 } });
    await preflightChapterPlanRun("project", { ...base, generationMode: "range", target: { startChapterNo: 2, endChapterNo: 4 } });
    assert.deepEqual(bodies.map((body) => body.target), [{ targetTotalChapters: 12 }, { chapterCount: 3 }, { startChapterNo: 2, endChapterNo: 4 }]);
    for (const body of bodies) assert.equal("requestedChapterCount" in (body.target as object), false);
  } finally {
    globalThis.fetch = originalFetch;
  }
});

test("candidate edit sends expectedCandidateVersion, currentSnapshot, and Idempotency-Key", async () => {
  const originalFetch = globalThis.fetch;
  let capturedUrl = "";
  let capturedMethod = "";
  let capturedHeaders: Record<string, string> = {};
  let capturedBody: Record<string, unknown> = {};

  globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
    capturedUrl = String(input);
    capturedMethod = init?.method || "GET";
    capturedHeaders = (init?.headers as Record<string, string>) || {};
    capturedBody = JSON.parse(String(init?.body || "{}")) as Record<string, unknown>;
    return new Response(
      JSON.stringify({
        data: {
          data: { id: "cand-1", version: 2, currentSnapshot: capturedBody.currentSnapshot },
          request_id: "req-1",
        },
        request_id: "env-1",
      }),
      { status: 200, headers: { "Content-Type": "application/json" } },
    );
  }) as typeof fetch;

  try {
    const candidateId = "cand-1";
    const currentSnapshot: ChapterPlanCandidateSnapshot = {
      chapterNo: 1,
      title: "Updated Title",
      summary: "Updated Summary",
      chapterPurpose: "plot_advance",
      storylineRefs: [],
      materialRefs: [],
      foreshadowingRefs: [],
      generationBasis: {
        contextSummary: "basis",
        additionalInstructions: null,
      },
    };

    const payload = {
      expectedCandidateVersion: 1,
      currentSnapshot,
    };
    const key = "key-edit-123";

    const res = await updateChapterPlanCandidate(candidateId, payload, key);
    const bodySnapshot = capturedBody.currentSnapshot as Record<string, unknown>;

    assert.match(capturedUrl, /\/chapter-plan-candidates\/cand-1$/);
    assert.equal(capturedMethod, "PATCH");
    assert.equal(capturedHeaders["Idempotency-Key"], "key-edit-123");
    assert.equal(capturedBody.expectedCandidateVersion, 1);
    assert.equal(bodySnapshot.title, "Updated Title");
    assert.equal(res.data.id, "cand-1");
  } finally {
    globalThis.fetch = originalFetch;
  }
});

test("candidate edit propagates version conflict (409) without silent overwrite", async () => {
  const originalFetch = globalThis.fetch;
  globalThis.fetch = (async () => {
    return new Response(
      JSON.stringify({
        code: "version_conflict",
        message: "Candidate version conflict",
      }),
      { status: 409, headers: { "Content-Type": "application/json" } },
    );
  }) as typeof fetch;

  try {
    const currentSnapshot: ChapterPlanCandidateSnapshot = {
      chapterNo: 1,
      title: "Title",
      summary: "Summary",
      chapterPurpose: "plot",
      storylineRefs: [],
      materialRefs: [],
      foreshadowingRefs: [],
      generationBasis: { contextSummary: "", additionalInstructions: null },
    };
    await updateChapterPlanCandidate(
      "cand-1",
      { expectedCandidateVersion: 1, currentSnapshot },
      "key-409",
    );
    assert.fail("Should have thrown 409 ApiError");
  } catch (cause) {
    assert.ok(cause instanceof ApiError);
    assert.equal((cause as ApiError).status, 409);
  } finally {
    globalThis.fetch = originalFetch;
  }
});

test("recompare sends POST to /recompare with expectedCandidateVersion and does NOT call /adopt", async () => {
  const originalFetch = globalThis.fetch;
  let capturedUrl = "";
  let capturedMethod = "";
  let capturedBody: Record<string, unknown> = {};

  globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
    capturedUrl = String(input);
    capturedMethod = init?.method || "GET";
    capturedBody = JSON.parse(String(init?.body || "{}")) as Record<string, unknown>;
    return new Response(
      JSON.stringify({
        data: {
          data: {
            candidate: { id: "cand-1", version: 2 },
            currentChapter: null,
            diff: { baseRevisionId: null, candidateVersion: 2, entries: [], stale: false },
          },
          request_id: "req-rec",
        },
        request_id: "env-rec",
      }),
      { status: 200, headers: { "Content-Type": "application/json" } },
    );
  }) as typeof fetch;

  try {
    const res = await recompareChapterPlanCandidate(
      "cand-1",
      { expectedCandidateVersion: 1 },
      "key-rec-1",
    );

    assert.match(capturedUrl, /\/chapter-plan-candidates\/cand-1\/recompare$/);
    assert.doesNotMatch(capturedUrl, /\/adopt/);
    assert.equal(capturedMethod, "POST");
    assert.equal(capturedBody.expectedCandidateVersion, 1);
    assert.equal(res.data.candidate.id, "cand-1");
  } finally {
    globalThis.fetch = originalFetch;
  }
});

test("single adopt sends expectedCandidateVersion, expectedChapterPlanVersion, and Idempotency-Key", async () => {
  const originalFetch = globalThis.fetch;
  let capturedUrl = "";
  let capturedHeaders: Record<string, string> = {};
  let capturedBody: Record<string, unknown> = {};

  globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
    capturedUrl = String(input);
    capturedHeaders = (init?.headers as Record<string, string>) || {};
    capturedBody = JSON.parse(String(init?.body || "{}")) as Record<string, unknown>;
    return new Response(
      JSON.stringify({
        data: {
          data: {
            outcome: "adopted",
            candidate: { id: "cand-1", status: "adopted" },
            chapterPlan: { id: "cp-1" },
            revision: { revisionNo: 1 },
          },
          request_id: "req-adopt",
        },
        request_id: "env-adopt",
      }),
      { status: 200, headers: { "Content-Type": "application/json" } },
    );
  }) as typeof fetch;

  try {
    const res = await adoptChapterPlanCandidate(
      "cand-1",
      { expectedCandidateVersion: 1, expectedChapterPlanVersion: 3 },
      "key-adopt-1",
    );

    assert.match(capturedUrl, /\/chapter-plan-candidates\/cand-1\/adopt$/);
    assert.equal(capturedHeaders["Idempotency-Key"], "key-adopt-1");
    assert.equal(capturedBody.expectedCandidateVersion, 1);
    assert.equal(capturedBody.expectedChapterPlanVersion, 3);
    assert.equal((res.data as unknown as { outcome: string }).outcome, "adopted");
  } finally {
    globalThis.fetch = originalFetch;
  }
});

test("bulk adopt preserves itemized outcomes (adopted, no_change, stale, conflict, failed)", async () => {
  const originalFetch = globalThis.fetch;
  let capturedUrl = "";
  let capturedBody: Record<string, unknown> = {};

  globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
    capturedUrl = String(input);
    capturedBody = JSON.parse(String(init?.body || "{}")) as Record<string, unknown>;
    return new Response(
      JSON.stringify({
        data: {
          data: {
            batch: { id: "batch-1", status: "partially_adopted" },
            items: [
              { candidateId: "c1", outcome: "adopted" },
              { candidateId: "c2", outcome: "no_change" },
              { candidateId: "c3", outcome: "stale" },
              { candidateId: "c4", outcome: "conflict" },
              { candidateId: "c5", outcome: "failed" },
            ],
          },
          request_id: "req-bulk",
        },
        request_id: "env-bulk",
      }),
      { status: 200, headers: { "Content-Type": "application/json" } },
    );
  }) as typeof fetch;

  try {
    const res = await adoptChapterPlanCandidates(
      "batch-1",
      {
        expectedBatchVersion: 1,
        candidates: [
          { candidateId: "c1", expectedCandidateVersion: 1, expectedChapterPlanVersion: null },
          { candidateId: "c2", expectedCandidateVersion: 1, expectedChapterPlanVersion: null },
          { candidateId: "c3", expectedCandidateVersion: 1, expectedChapterPlanVersion: null },
          { candidateId: "c4", expectedCandidateVersion: 1, expectedChapterPlanVersion: null },
          { candidateId: "c5", expectedCandidateVersion: 1, expectedChapterPlanVersion: null },
        ],
      },
      "key-bulk-1",
    );

    assert.match(capturedUrl, /\/chapter-plan-candidate-batches\/batch-1\/adoptions$/);
    assert.equal(capturedBody.expectedBatchVersion, 1);
    assert.equal(res.data.items.length, 5);
    assert.equal(res.data.items[0].outcome, "adopted");
    assert.equal(res.data.items[1].outcome, "no_change");
    assert.equal(res.data.items[2].outcome, "stale");
    assert.equal(res.data.items[3].outcome, "conflict");
    assert.equal(res.data.items[4].outcome, "failed");
  } finally {
    globalThis.fetch = originalFetch;
  }
});

test("abandon batch explicitly carries acknowledgeAdoptedChaptersRemain: true", async () => {
  const originalFetch = globalThis.fetch;
  let capturedUrl = "";
  let capturedBody: Record<string, unknown> = {};

  globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
    capturedUrl = String(input);
    capturedBody = JSON.parse(String(init?.body || "{}")) as Record<string, unknown>;
    return new Response(
      JSON.stringify({
        data: {
          data: { id: "batch-1", status: "abandoned" } as ChapterPlanCandidateBatch,
          request_id: "req-abandon",
        },
        request_id: "env-abandon",
      }),
      { status: 200, headers: { "Content-Type": "application/json" } },
    );
  }) as typeof fetch;

  try {
    const res = await abandonChapterPlanCandidateBatch(
      "batch-1",
      {
        expectedBatchVersion: 2,
        reason: "User abandoned batch",
        acknowledgeAdoptedChaptersRemain: true,
      },
      "key-abandon-1",
    );

    assert.match(capturedUrl, /\/chapter-plan-candidate-batches\/batch-1\/abandon$/);
    assert.equal(capturedBody.expectedBatchVersion, 2);
    assert.equal(capturedBody.acknowledgeAdoptedChaptersRemain, true);
    assert.equal(res.data.status, "abandoned");
  } finally {
    globalThis.fetch = originalFetch;
  }
});

test("create chapter plan run sends preflightToken and Idempotency-Key", async () => {
  const originalFetch = globalThis.fetch;
  let capturedUrl = "";
  let capturedHeaders: Record<string, string> = {};
  let capturedBody: Record<string, unknown> = {};

  globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
    capturedUrl = String(input);
    capturedHeaders = (init?.headers as Record<string, string>) || {};
    capturedBody = JSON.parse(String(init?.body || "{}")) as Record<string, unknown>;
    return new Response(
      JSON.stringify({
        data: {
          data: { id: "run-100", status: "running" },
          request_id: "req-create-run",
        },
        request_id: "env-run",
      }),
      { status: 200, headers: { "Content-Type": "application/json" } },
    );
  }) as typeof fetch;

  try {
    const res = await createChapterPlanRun(
      "proj-1",
      { preflightToken: "token-abc-123" },
      "key-run-1",
    );

    assert.match(capturedUrl, /\/projects\/proj-1\/chapter-plan-runs$/);
    assert.equal(capturedHeaders["Idempotency-Key"], "key-run-1");
    assert.equal(capturedBody.preflightToken, "token-abc-123");
    assert.equal((res as { data?: { id?: string } }).data?.id, "run-100");
  } finally {
    globalThis.fetch = originalFetch;
  }
});

test("production chapter-plans files do not import mock-only fixtures or modules", () => {
  const detailPageSource = readFileSync(
    new URL("./candidate-batch-detail-page.tsx", import.meta.url),
    "utf8",
  );

  assert.doesNotMatch(detailPageSource, /mock-data/i);
  assert.doesNotMatch(detailPageSource, /fixture/i);
});

test("ChapterPlan contract supports candidate_adopted source and nullable OpenAPI fields", () => {
  const plan: ChapterPlan = {
    id: "cp-1",
    project_id: "p-1",
    chapter_no: 1,
    title: "Chapter 1",
    summary: "Summary 1",
    status: "confirmed",
    source: "candidate_adopted",
    storyline_refs_json: [{ storyline_id: "sl-1", relation: "primary" }],
    material_refs_json: [],
    foreshadowing_refs_json: [],
    chapter_goal: null,
    creation_notes: null,
    confirmed_at: "2026-07-27T12:00:00Z",
    currentRevisionId: "rev-1",
    sourceCandidateId: "cand-1",
    sourceCandidateBatchId: "batch-1",
    sourceWorkflowRunId: "run-1",
    version: 1,
    created_at: "2026-07-27T12:00:00Z",
    updated_at: "2026-07-27T12:00:00Z",
  };

  assert.equal(plan.source, "candidate_adopted");
  assert.equal(plan.currentRevisionId, "rev-1");
  assert.equal(plan.sourceCandidateId, "cand-1");
  assert.equal(plan.sourceCandidateBatchId, "batch-1");
  assert.equal(plan.sourceWorkflowRunId, "run-1");
});
