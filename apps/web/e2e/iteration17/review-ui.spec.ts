import { expect, test, type Page } from "@playwright/test";

const projectId = "11111111-1111-4111-8111-111111111111";
const workId = "22222222-2222-4222-8222-222222222222";
const versionId = "33333333-3333-4333-8333-333333333333";
const historicalVersionId = "33333333-3333-4333-8333-333333333332";
const reportId = "44444444-4444-4444-8444-444444444444";
const issueId = "55555555-5555-4555-8555-555555555555";
const secondIssueId = "99999999-9999-4999-8999-999999999999";
const runId = "66666666-6666-4666-8666-666666666666";

const project = {
  id: projectId,
  name: "霓港纪事",
  type: "novel",
  status: "planning",
  description: "Iteration 17 真实内容审核浏览器验收",
  current_stage: "review",
  created_at: "2026-07-28T06:00:00Z",
  updated_at: "2026-07-28T06:32:00Z",
};
const version = {
  id: versionId,
  content_item_id: workId,
  version_no: 4,
  version: 7,
  status: "editable_draft",
  source: "workflow_generated",
  title: "第08章 雨夜来客",
  content:
    "夜雨瓢泼，霓港市的老城区在水雾中显得格外朦胧。\n\n突然，一道闪电撕裂夜空，李青看清了地上的那滩暗红。\n\n雨水很快冲刷掉了巷子里的痕迹。",
  summary: "雨夜中的案件线索。",
  word_count: 3842,
  frozen_at: null,
  created_at: "2026-07-28T06:00:00Z",
  updated_at: "2026-07-28T06:32:00Z",
};
const historicalVersion = {
  ...version,
  id: historicalVersionId,
  version_no: 3,
  version: 2,
  status: "frozen",
  title: "第08章 雨夜来客（V3 固定稿）",
  word_count: 3620,
  frozen_at: "2026-07-27T06:32:00Z",
  created_at: "2026-07-27T06:00:00Z",
  updated_at: "2026-07-27T06:32:00Z",
};
const content = {
  content_item: {
    id: workId,
    chapter_plan_id: "77777777-7777-4777-8777-777777777777",
    title: version.title,
    status: "draft",
    current_version_id: versionId,
    reviewed_at: null,
    created_at: "2026-07-28T06:00:00Z",
    updated_at: "2026-07-28T06:32:00Z",
  },
  current_version: version,
};
const run = {
  id: runId,
  runNumber: "RUN-REVIEW-017",
  projectId,
  stage: "review",
  subjectType: "content_version",
  subjectId: historicalVersionId,
  workflowConfigurationId: "88888888-8888-4888-8888-888888888888",
  triggerSource: "manual",
  status: "running",
  inputPayload: {},
  outputPayload: null,
  errorCode: null,
  errorMessage: null,
  errorDetails: null,
  configurationSnapshot: {},
  startedAt: "2026-07-28T06:35:00Z",
  finishedAt: null,
  cancelledAt: null,
  createdAt: "2026-07-28T06:35:00Z",
  updatedAt: "2026-07-28T06:35:20Z",
  version: 3,
};
const sourceSummary = {
  id: versionId,
  contentItemId: workId,
  versionNo: 4,
  version: 7,
  title: version.title,
  wordCount: version.word_count,
  contentHash: "a".repeat(64),
  frozen: true,
};
const issue = {
  id: issueId,
  reviewId: reportId,
  issueKey: "character-consistency-1",
  position: 1,
  categoryKey: "character_consistency",
  categoryLabel: "角色一致性",
  severity: "critical",
  title: "人物行为与前文设定冲突",
  description: "人物反应与前文建立的冷静设定不一致。",
  evidence: {
    quote: "突然，一道闪电撕裂夜空，李青看清了地上的那滩暗红。",
    sourceRefs: ["角色设定"],
  },
  location: {
    paragraphStart: 2,
    paragraphEnd: 2,
    sentenceStart: 1,
    sentenceEnd: 1,
  },
  suggestion: "调整人物反应，使其保持专业判断。",
  disposition: "open",
  version: 2,
  ignoredAt: null,
  ignoredBy: null,
  createdAt: "2026-07-28T06:37:00Z",
  updatedAt: "2026-07-28T06:37:00Z",
};
const report = {
  id: reportId,
  contentItemId: workId,
  sourceContentVersionId: versionId,
  sourceContentVersionVersion: 7,
  sourceContentHash: "a".repeat(64),
  workflowRunId: runId,
  schemaVersion: "review.output.v1",
  conclusion: "needs_changes",
  summary: "发现需要优先处理的问题，建议修改后再次审核。",
  passedRuleCount: 142,
  createdAt: "2026-07-28T06:37:00Z",
  completedAt: "2026-07-28T06:37:00Z",
};
const realDetail = {
  report,
  sourceContentVersionSummary: sourceSummary,
  issues: [
    issue,
    {
      ...issue,
      id: secondIssueId,
      issueKey: "language-1",
      position: 2,
      severity: "warning",
      categoryKey: "language_quality",
      categoryLabel: "语言表达",
      title: "句式表达可进一步精简",
      evidence: { quote: null, sourceRefs: [] },
      location: null,
    },
  ],
  recommendations: [],
  workflowRunSummary: { ...run, status: "succeeded", finishedAt: report.completedAt },
};

type FixtureState =
  | "idle"
  | "not_configured"
  | "queued"
  | "running"
  | "review_ready"
  | "runtime_failed"
  | "output_validation_failed"
  | "result_consumption_failed";

async function installFixture(
  page: Page,
  state: FixtureState,
  options: {
    uncertainOnce?: "create" | "runtimeRetry" | "consumptionRetry" | "issue";
  } = {},
) {
  const counters = {
    create: 0,
    preflight: 0,
    issue: 0,
    runtimeRetry: 0,
    consumptionRetry: 0,
    selectedVersionId: "",
    createKeys: [] as string[],
    runtimeRetryKeys: [] as string[],
    consumptionRetryKeys: [] as string[],
    issueKeys: [] as string[],
  };
  let issueValue = { ...issue };
  page.on("console", (message) => {
    if (
      message.type() === "error" &&
      !(
        options.uncertainOnce &&
        message.text().includes("status of 500")
      )
    ) {
      throw new Error(message.text());
    }
  });
  page.on("pageerror", (error) => {
    throw error;
  });
  await page.route("**/api/v1/**", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const path = url.pathname;
    const json = (data: unknown, status = 200) =>
      route.fulfill({
        status,
        contentType: "application/json",
        body: JSON.stringify({ data, request_id: "iteration17-browser" }),
      });
    const uncertain = (command: NonNullable<typeof options.uncertainOnce>) =>
      options.uncertainOnce === command
        ? route.fulfill({
            status: 500,
            contentType: "application/json",
            body: JSON.stringify({
              error: {
                code: "internal_error",
                message: "result unknown",
                details: {},
              },
              request_id: "iteration17-browser",
            }),
          })
        : null;

    if (path.endsWith(`/projects/${projectId}/workspace`))
      return json({
        project,
        progress: {
          material_count: 0,
          storyline_count: 0,
          confirmed_chapter_count: 1,
          work_count: 1,
        },
      });
    if (path.endsWith(`/content-items/${workId}`) && request.method() === "GET")
      return json(content);
    if (path.endsWith(`/content-items/${workId}/versions`))
      return json({
        items: [
          {
            ...version,
            is_current: true,
            source_content_version: null,
            source_review_report: null,
            source_workflow_run: null,
          },
          {
            ...historicalVersion,
            is_current: false,
            source_content_version: null,
            source_review_report: null,
            source_workflow_run: null,
          },
        ],
        total: 2,
        limit: 100,
        offset: 0,
      });
    if (path.includes(`/projects/${projectId}/chapter-plans`))
      return json({
        items: [
          {
            id: content.content_item.chapter_plan_id,
            project_id: projectId,
            chapter_no: 8,
            title: version.title,
            chapter_goal: "推进案件",
            creation_notes: "",
            storyline_refs_json: [],
            material_refs_json: [],
            foreshadowing_refs_json: [],
          },
        ],
        total: 1,
        limit: 100,
        offset: 0,
      });
    if (path.endsWith(`/content-items/${workId}/content-generation-summary`))
      return json({
        contentItemId: workId,
        contentItem: content.content_item,
        currentVersionId: versionId,
        currentVersion: version,
        workflowConfigured: true,
        state: "idle",
        activeRun: null,
        latestRun: null,
        latestEvents: [],
        latestCandidateVersion: null,
        latestError: null,
        canGenerate: true,
        candidateCanBecomeCurrent: false,
      });
    if (path.endsWith(`/content-items/${workId}/review-summary`)) {
      const active = state === "queued" || state === "running";
      const failed = state.endsWith("failed");
      return json({
        contentItemId: workId,
        state,
        canStartReview: state === "idle",
        activeRun: active ? { ...run, status: state } : null,
        latestRun:
          active || failed
            ? {
                ...run,
                status: active ? state : state === "runtime_failed" ? "failed" : "succeeded",
                finishedAt: active ? null : "2026-07-28T06:38:00Z",
              }
            : state === "review_ready"
              ? { ...run, status: "succeeded", finishedAt: report.completedAt }
              : null,
        latestReport:
          state === "review_ready"
            ? {
                id: reportId,
                workflowRunId: runId,
                sourceContentVersionId: versionId,
                conclusion: report.conclusion,
                summary: report.summary,
                completedAt: report.completedAt,
              }
            : null,
        issueSummary:
          state === "review_ready"
            ? { total: 2, critical: 1, warning: 1, suggestion: 0, open: 2, ignored: 0 }
            : null,
        latestError: failed
          ? {
              code: state,
              message:
                state === "result_consumption_failed"
                  ? "审核结果未能安全保存"
                  : state === "output_validation_failed"
                    ? "审核输出未通过结构校验"
                    : "审核工作流执行失败",
              correlationId: "CORR-REVIEW-17",
              attemptCount: 1,
              occurredAt: "2026-07-28T06:38:00Z",
            }
          : null,
        configurationSummary:
          state === "not_configured"
            ? null
            : {
                bindingVersion: 1,
                workflowConfigurationId: run.workflowConfigurationId,
                workflowConfigurationName: "内容质量审核 v2",
                workflowConfigurationVersion: 2,
                connectionId: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
                connectionVersion: 1,
                inputContract: "review.input.v1",
                outputContract: "review.output.v1",
              },
      });
    }
    if (
      path.endsWith(`/content-versions/${versionId}/review-runs/preflight`) ||
      path.endsWith(
        `/content-versions/${historicalVersionId}/review-runs/preflight`,
      )
    ) {
      counters.preflight += 1;
      counters.selectedVersionId = path.includes(historicalVersionId)
        ? historicalVersionId
        : versionId;
      expect(request.postDataJSON().sourceContentVersionVersion).toBe(
        counters.selectedVersionId === historicalVersionId ? 2 : 7,
      );
      return json({
        status: state === "not_configured" ? "blocked" : "passed",
        checks: [
          {
            code:
              state === "not_configured"
                ? "project_binding_available"
                : "source_version_saved",
            status: state === "not_configured" ? "blocked" : "passed",
            message:
              state === "not_configured"
                ? "项目尚未绑定审核工作流"
                : "固定来源版本已保存",
          },
        ],
        sourceContentVersionSummary: sourceSummary,
        reviewDimensions: [
          "compliance",
          "factual_consistency",
          "language_quality",
          "structural_logic",
          "character_consistency",
        ],
        configurationSummary:
          state === "not_configured"
            ? null
            : {
                bindingVersion: 1,
                workflowConfigurationId: run.workflowConfigurationId,
                workflowConfigurationName: "内容质量审核 v2",
                workflowConfigurationVersion: 2,
                connectionId: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
                connectionVersion: 1,
                inputContract: "review.input.v1",
                outputContract: "review.output.v1",
              },
        preflightToken: state === "not_configured" ? null : "preflight-token",
        expiresAt: state === "not_configured" ? null : "2026-07-28T06:45:00Z",
      });
    }
    if (
      (path.endsWith(`/content-versions/${versionId}/review-runs`) ||
        path.endsWith(
          `/content-versions/${historicalVersionId}/review-runs`,
        )) &&
      request.method() === "POST"
    ) {
      counters.create += 1;
      counters.createKeys.push(request.headers()["idempotency-key"] ?? "");
      counters.selectedVersionId = path.includes(historicalVersionId)
        ? historicalVersionId
        : versionId;
      expect(request.headers()["idempotency-key"]).toBeTruthy();
      expect(request.postDataJSON()).toEqual({ preflightToken: "preflight-token" });
      if (counters.create === 1 && options.uncertainOnce === "create")
        return uncertain("create");
      return json({ ...run, status: "queued" }, 201);
    }
    if (path.endsWith(`/workflow-runs/${runId}/events`))
      return json({
        items: [
          { id: "event-1", runId, eventType: "queued", status: "queued", payload: {}, createdAt: run.createdAt },
          { id: "event-2", runId, eventType: "worker_started", status: "running", payload: {}, createdAt: run.startedAt },
        ],
      });
    if (path.endsWith(`/reviews/${reportId}`))
      return json({ ...realDetail, issues: realDetail.issues.map((item) => item.id === issueId ? issueValue : item) });
    if (path.endsWith(`/content-versions/${versionId}`))
      return json({ content_version: { ...version, status: "frozen", frozen_at: report.completedAt }, is_current: true });
    if (path.endsWith(`/content-versions/${historicalVersionId}`))
      return json({ content_version: historicalVersion, is_current: false });
    if (path.endsWith(`/reviews/${reportId}/issues/${issueId}`)) {
      counters.issue += 1;
      counters.issueKeys.push(request.headers()["idempotency-key"] ?? "");
      expect(request.headers()["idempotency-key"]).toBeTruthy();
      expect(request.postDataJSON()).toEqual({ disposition: "ignored", expectedVersion: 2 });
      if (counters.issue === 1 && options.uncertainOnce === "issue")
        return uncertain("issue");
      issueValue = { ...issueValue, disposition: "ignored", version: 3 };
      return json(issueValue);
    }
    if (path.endsWith(`/workflow-runs/${runId}/retries`)) {
      counters.runtimeRetry += 1;
      counters.runtimeRetryKeys.push(request.headers()["idempotency-key"] ?? "");
      expect(request.postDataJSON()).toEqual({
        expectedVersion: 3,
        useCurrentConfiguration: false,
      });
      if (counters.runtimeRetry === 1 && options.uncertainOnce === "runtimeRetry")
        return uncertain("runtimeRetry");
      return json({ ...run, id: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", status: "queued" }, 201);
    }
    if (
      path.endsWith(
        `/workflow-runs/${runId}/review-result-consumption-retries`,
      )
    ) {
      counters.consumptionRetry += 1;
      counters.consumptionRetryKeys.push(request.headers()["idempotency-key"] ?? "");
      expect(request.postDataJSON()).toEqual({ expectedRunVersion: 3 });
      if (
        counters.consumptionRetry === 1 &&
        options.uncertainOnce === "consumptionRetry"
      )
        return uncertain("consumptionRetry");
      return json({
        report,
        issues: [issueValue],
        recommendations: [],
        workflowRun: { ...run, status: "succeeded" },
      });
    }
    if (path.endsWith(`/content-items/${workId}/reviews`)) {
      return json({
        items: [
          {
            workflowRun: { ...run, status: "succeeded", finishedAt: report.completedAt },
            sourceContentVersionSummary: sourceSummary,
            reportSummary: {
              id: reportId,
              workflowRunId: runId,
              sourceContentVersionId: versionId,
              conclusion: report.conclusion,
              summary: report.summary,
              completedAt: report.completedAt,
            },
            state: "review_ready",
            latestError: null,
          },
          {
            workflowRun: {
              ...run,
              id: "cccccccc-cccc-4ccc-8ccc-cccccccccccc",
              status: "failed",
              createdAt: "2026-07-27T06:00:00Z",
            },
            sourceContentVersionSummary: { ...sourceSummary, versionNo: 3 },
            reportSummary: null,
            state: "output_validation_failed",
            latestError: {
              code: "output_validation_failed",
              message: "审核输出未通过结构校验",
              correlationId: "CORR-HISTORY-17",
              attemptCount: 1,
              occurredAt: "2026-07-27T06:01:00Z",
            },
          },
          {
            id: "dddddddd-dddd-4ddd-8ddd-dddddddddddd",
            content_item_id: workId,
            content_version_id: versionId,
            provider_key: "mock",
            status: "completed",
            conclusion: "pass",
            score: 92,
            summary: "P0 模拟审核通过",
            created_at: "2026-07-26T06:00:00Z",
          },
        ],
        total: 3,
        limit: 20,
        offset: 0,
      });
    }
    return route.fulfill({
      status: 404,
      contentType: "application/json",
      body: JSON.stringify({
        error: { code: "fixture_not_found", message: path, details: {} },
        request_id: "iteration17-browser",
      }),
    });
  });
  return counters;
}

async function assertHealthyPage(page: Page) {
  const overflow = await page.evaluate(
    () =>
      document.documentElement.scrollWidth >
      document.documentElement.clientWidth + 1,
  );
  expect(overflow).toBe(false);
  await expect(page.locator(".shell-brand").first()).toContainText(
    /AI Content\s*Factory/,
  );
  await expect(
    page.locator(".shell-nav").first().getByRole("link", { name: "项目", exact: true }),
  ).toBeVisible();
}

test("I17_D1_EDITOR_REVIEW_ENTRY and D2_SUBMIT_REVIEW_DRAWER enforce saved draft, preflight, cancellation, and idempotent create", async ({ page }) => {
  const counters = await installFixture(page, "idle");
  await page.goto(`/projects/${projectId}/works/${workId}`);
  await assertHealthyPage(page);
  const submit = page.getByRole("button", { name: "提交审核" });
  await expect(submit).toBeEnabled();
  await submit.click();
  await expect(page.getByRole("dialog", { name: "发起内容审核" })).toBeVisible();
  await expect(page.getByText("预检通过，可以发起审核。")).toBeVisible();
  expect(counters.create).toBe(0);
  await page.getByRole("button", { name: "取消", exact: true }).click();
  expect(counters.create).toBe(0);

  await page.locator("textarea.content-body-input").fill("未保存的正文修改");
  await expect(submit).toBeDisabled();
  await expect(page.getByText("请先保存当前修改，再提交审核。")).toBeVisible();
  await page.reload();
  await submit.click();
  const instructions = page.getByPlaceholder("例如：重点检查人物动机与前文设定是否一致");
  await expect(page.getByText("预检通过，可以发起审核。")).toBeVisible();
  await page.getByRole("radio", { name: /V3/ }).click();
  await expect.poll(() => counters.selectedVersionId).toBe(historicalVersionId);
  await expect(page.getByRole("radio", { name: /V3/ })).toHaveAttribute(
    "aria-checked",
    "true",
  );
  const beforeChange = counters.preflight;
  await instructions.fill("变更审核说明");
  await expect(page.getByRole("button", { name: "确认发起审核" })).toBeDisabled();
  await expect.poll(() => counters.preflight).toBeGreaterThan(beforeChange);
  await expect(page.getByRole("button", { name: "确认发起审核" })).toBeEnabled();
  await page.getByRole("button", { name: "确认发起审核" }).dblclick();
  await expect.poll(() => counters.create).toBe(1);
  expect(counters.selectedVersionId).toBe(historicalVersionId);
});

test("STATE_TASK_RUNNING_BAR restores queued and running from persisted summary", async ({ page }) => {
  for (const state of ["queued", "running"] as const) {
    await installFixture(page, state);
    await page.goto(`/projects/${projectId}/works/${workId}/review`);
    await assertHealthyPage(page);
    await expect(page.getByText(state === "queued" ? "已排队" : "运行中", { exact: true }).first()).toBeVisible();
    await expect(page.getByText("任务会在后台继续运行，可以安全离开当前页面。")).toBeVisible();
    await expect(page.getByText("创建审核任务")).toBeVisible();
    await expect(page.getByText("第08章 雨夜来客（V3 固定稿）", { exact: false }).first()).toBeVisible();
    await expect(page.getByText("V3", { exact: true }).first()).toBeVisible();
    await expect(page.getByText(/3,620 字/).first()).toBeVisible();
  }
});

test("D2_REVIEW_V2 renders the fixed report and updates issue disposition with version and idempotency", async ({ page }) => {
  const counters = await installFixture(page, "review_ready");
  await page.goto(`/projects/${projectId}/works/${workId}/review?reportId=${reportId}`);
  await assertHealthyPage(page);
  await expect(page.getByText("建议修改", { exact: true }).first()).toBeVisible();
  await expect(page.getByText("142")).toBeVisible();
  await expect(page.getByText("人物行为与前文设定冲突", { exact: true }).first()).toBeVisible();
  await expect(page.getByText("固定来源版本")).toBeVisible();
  await expect(page.getByRole("button", { name: "创建重写任务（Iteration 18）" }).last()).toBeDisabled();
  await page.getByRole("button", { name: "标记为忽略" }).click();
  await expect.poll(() => counters.issue).toBe(1);
  await expect(page.getByText("已忽略", { exact: true })).toBeVisible();
});

test("I17_D2_REVIEW_ISSUE_DETAIL locates against the frozen source and keeps report query on return", async ({ page }) => {
  await installFixture(page, "review_ready");
  await page.goto(`/projects/${projectId}/works/${workId}/review?reportId=${reportId}&issueId=${issueId}&view=source`);
  await assertHealthyPage(page);
  await expect(page.getByText("来源正文只读快照 · V4")).toBeVisible();
  await expect(page.locator("mark")).toContainText("突然，一道闪电撕裂夜空");
  const back = page.getByRole("link", { name: "返回问题列表" });
  await expect(back).toHaveAttribute("href", new RegExp(`reportId=${reportId}`));
  await expect(page.getByRole("button", { name: "创建重写任务（Iteration 18）" })).toBeDisabled();
});

test("ordinary Issue selection keeps issueId independent from source view", async ({ page }) => {
  await installFixture(page, "review_ready");
  await page.goto(`/projects/${projectId}/works/${workId}/review?reportId=${reportId}`);
  await page.getByRole("button", { name: /句式表达可进一步精简/ }).click();
  await expect(page).toHaveURL(new RegExp(`issueId=${secondIssueId}`));
  await expect(page).not.toHaveURL(/view=source/);
  await expect(page.locator(".review-issue-list button.active")).toContainText(
    "句式表达可进一步精简",
  );
  await expect(page.locator(".review-issue-panel")).toContainText(
    "句式表达可进一步精简",
  );
  await page.locator(".review-issue-panel").getByRole("link", { name: "在全文中定位" }).click();
  await expect(page).toHaveURL(
    new RegExp(`issueId=${secondIssueId}.*view=source`),
  );
  await expect(page.locator(".review-source aside")).toContainText(
    "句式表达可进一步精简",
  );
});

test("Create Review Run reuses its key after an uncertain response", async ({ page }) => {
  const counters = await installFixture(page, "idle", {
    uncertainOnce: "create",
  });
  await page.goto(`/projects/${projectId}/works/${workId}`);
  await page.getByRole("button", { name: "提交审核" }).click();
  await expect(page.getByRole("button", { name: "确认发起审核" })).toBeEnabled();
  await page.getByRole("button", { name: "确认发起审核" }).click();
  await expect.poll(() => counters.create).toBe(1);
  await page.getByRole("button", { name: "确认发起审核" }).click();
  await expect.poll(() => counters.create).toBe(2);
  expect(counters.createKeys[1]).toBe(counters.createKeys[0]);
});

test("Runtime Retry reuses its key after an uncertain response", async ({ page }) => {
  const counters = await installFixture(page, "runtime_failed", {
    uncertainOnce: "runtimeRetry",
  });
  await page.goto(`/projects/${projectId}/works/${workId}/review`);
  const retry = page.getByRole("button", { name: "重新执行审核" });
  await retry.click();
  await expect.poll(() => counters.runtimeRetry).toBe(1);
  await retry.click();
  await expect.poll(() => counters.runtimeRetry).toBe(2);
  expect(counters.runtimeRetryKeys[1]).toBe(counters.runtimeRetryKeys[0]);
});

test("consumption Retry reuses its key after an uncertain response", async ({ page }) => {
  const counters = await installFixture(page, "result_consumption_failed", {
    uncertainOnce: "consumptionRetry",
  });
  await page.goto(`/projects/${projectId}/works/${workId}/review`);
  const retry = page.getByRole("button", { name: "重试保存结果" });
  await retry.click();
  await expect.poll(() => counters.consumptionRetry).toBe(1);
  await retry.click();
  await expect.poll(() => counters.consumptionRetry).toBe(2);
  expect(counters.consumptionRetryKeys[1]).toBe(
    counters.consumptionRetryKeys[0],
  );
});

test("Issue disposition reuses its key after an uncertain response", async ({ page }) => {
  const counters = await installFixture(page, "review_ready", {
    uncertainOnce: "issue",
  });
  await page.goto(`/projects/${projectId}/works/${workId}/review?reportId=${reportId}`);
  const ignore = page.getByRole("button", { name: "标记为忽略" });
  await ignore.click();
  await expect.poll(() => counters.issue).toBe(1);
  await ignore.click();
  await expect.poll(() => counters.issue).toBe(2);
  expect(counters.issueKeys[1]).toBe(counters.issueKeys[0]);
  await expect(page.getByText("已忽略", { exact: true })).toBeVisible();
});

test("STATE_TASK_FAILED_NOTICE separates Runtime Retry from result-consumption Retry", async ({ page }) => {
  for (const state of ["runtime_failed", "output_validation_failed", "result_consumption_failed"] as const) {
    const counters = await installFixture(page, state);
    await page.goto(`/projects/${projectId}/works/${workId}/review`);
    await assertHealthyPage(page);
    if (state === "result_consumption_failed") {
      await expect(page.getByRole("button", { name: "重试保存结果" })).toBeVisible();
      await expect(page.getByRole("button", { name: "重新执行审核" })).toHaveCount(0);
      await page.getByRole("button", { name: "重试保存结果" }).click();
      await expect.poll(() => counters.consumptionRetry).toBe(1);
      expect(counters.runtimeRetry).toBe(0);
    } else {
      await expect(page.getByRole("button", { name: "重新执行审核" })).toBeVisible();
      await expect(page.getByRole("button", { name: "重试保存结果" })).toHaveCount(0);
      await page.getByRole("button", { name: "重新执行审核" }).click();
      await expect.poll(() => counters.runtimeRetry).toBe(1);
      expect(counters.consumptionRetry).toBe(0);
    }
    await expect(page.getByText("CORR-REVIEW-17")).toBeVisible();
  }
});

test("STATE_NOT_CONFIGURED_EMPTY blocks create and links to project workflow settings", async ({ page }) => {
  await installFixture(page, "not_configured");
  await page.goto(`/projects/${projectId}/works/${workId}/review`);
  await assertHealthyPage(page);
  await expect(page.getByText("尚未配置内容审核工作流")).toBeVisible();
  await expect(page.getByRole("link", { name: "前往项目设置" })).toHaveAttribute(
    "href",
    `/projects/${projectId}/settings?tab=workflow-bindings`,
  );
  await expect(page.getByRole("button", { name: "发起新审核" })).toBeDisabled();
});

test("I17_D2_REVIEW_HISTORY keeps multi-version Runtime failures and P0 mock reports", async ({ page }) => {
  await installFixture(page, "review_ready");
  await page.goto(`/projects/${projectId}/works/${workId}/review/history`);
  await assertHealthyPage(page);
  await expect(page.getByRole("heading", { name: "审核历史" })).toBeVisible();
  await expect(page.getByText("V4", { exact: true }).first()).toBeVisible();
  await expect(page.getByText("V3", { exact: true })).toBeVisible();
  await expect(page.getByText("输出校验失败", { exact: true }).first()).toBeVisible();
  await expect(page.getByText("P0 模拟审核", { exact: true }).first()).toBeVisible();
  await expect(page.locator(".review-history-counts")).toContainText("严重问题");
  await page.getByText("V3", { exact: true }).click();
  await expect(page.getByText("审核输出未通过结构校验", { exact: true }).first()).toBeVisible();
});
