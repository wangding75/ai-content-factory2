import fs from "node:fs";
import { createRequire } from "node:module";
import path from "node:path";

const requireFromWeb = createRequire(
  path.resolve("apps/web/package.json"),
);
const { chromium } = requireFromWeb("@playwright/test");

const outputDir = path.resolve(
  process.env.ACCEPTANCE_OUTPUT_DIR ??
    "report/iteration-15-18-browser-acceptance/iteration-18/capture",
);
fs.mkdirSync(outputDir, { recursive: true });

const projectId = "973d7f23-3430-4a17-96e0-c8190535a87d";
const workId = "6b7c451c-d3e2-4b75-a0e5-f67647200444";
const reviewId = "c4292128-bcc3-4ff4-aaf1-4bf8ae2cdf0a";
const source = {
  id: "30fef55f-a448-47ea-b096-2e2fc54da1e8",
  contentItemId: workId,
  versionNo: 1,
  version: 2,
  title: "待审核章节",
  wordCount: 17,
  contentHash:
    "e893b69327ce0ee050ee8f5a29907a8a4b78175aa933fda51094afe52b864cb4",
};
const configuration = {
  bindingVersion: 1,
  workflowConfigurationId: "df7f9d82-ff71-4f61-8449-f2e42efc2729",
  workflowConfigurationName:
    "rewrite-df7f9d82-ff71-4f61-8449-f2e42efc2729",
  workflowConfigurationVersion: 1,
  connectionId: "d2ec7c4f-9d2c-488d-8fd7-e8b9e55e8816",
  connectionVersion: 1,
  inputContract: "rewrite.input.v1",
  outputContract: "rewrite.output.v1",
};

const summary = {
  reviewReportId: reviewId,
  contentItemId: workId,
  state: "idle",
  canStartRewrite: true,
  activeRun: null,
  latestRun: null,
  sourceContentVersionSummary: source,
  selectedIssueSummary: null,
  candidateVersion: null,
  candidateIsCurrent: false,
  canSetCurrent: false,
  latestError: null,
  configurationSummary: configuration,
};
const available = {
  reviewReportId: reviewId,
  contentItemId: workId,
  sourceContentVersionSummary: source,
  available: true,
  reason: null,
  openIssueCount: 1,
  activeRun: null,
  configurationSummary: configuration,
};
const unavailable = {
  ...available,
  available: false,
  reason: "rewrite_not_configured",
  configurationSummary: null,
};

const browser = await chromium.launch({ headless: true });
const context = await browser.newContext({
  viewport: { width: 1440, height: 900 },
  deviceScaleFactor: 1,
  locale: "zh-CN",
  timezoneId: "Asia/Shanghai",
  colorScheme: "light",
  reducedMotion: "reduce",
});
const results = [];

async function capture(frameId, availability) {
  const page = await context.newPage();
  const errors = [];
  page.on("console", (message) => {
    if (message.type() === "error") errors.push(message.text());
  });
  page.on("pageerror", (error) => errors.push(error.message));
  await page.route(`**/api/v1/reviews/${reviewId}/rewrite-summary`, (route) =>
    route.fulfill({ json: { data: summary, request_id: "acceptance-summary" } }),
  );
  await page.route(
    `**/api/v1/reviews/${reviewId}/rewrite-availability`,
    (route) =>
      route.fulfill({
        json: { data: availability, request_id: "acceptance-availability" },
      }),
  );
  await page.goto(
    `http://127.0.0.1:13001/projects/${projectId}/works/${workId}/rewrite?reportId=${reviewId}`,
    { waitUntil: "networkidle", timeout: 45_000 },
  );
  await page.addStyleTag({
    content:
      "*,*::before,*::after{animation-duration:0s!important;animation-delay:0s!important;transition:none!important;scroll-behavior:auto!important}",
  });
  if (frameId === "I18_D4_REWRITE_AVAILABILITY") {
    await page.getByRole("heading", { name: "项目重写配置不可用" }).waitFor();
  } else {
    await page.getByRole("heading", { name: "创建正文重写" }).waitFor();
  }
  await page.screenshot({
    path: path.join(outputDir, `${frameId}.png`),
  });
  results.push({ frameId, errors });
  return page;
}

const unavailablePage = await capture(
  "I18_D4_REWRITE_AVAILABILITY",
  unavailable,
);
await unavailablePage.close();

const createPage = await capture("I18_D4_CREATE_REWRITE", available);
await createPage.getByRole("button", { name: "查看项目重写配置" }).click();
await createPage
  .getByRole("dialog", { name: "项目重写配置" })
  .waitFor();
await createPage.screenshot({
  path: path.join(outputDir, "I18_D4_REWRITE_CONFIG_DRAWER.png"),
});
results.push({ frameId: "I18_D4_REWRITE_CONFIG_DRAWER", errors: [] });
await createPage.close();

async function captureReal(frameId, url, heading) {
  const page = await context.newPage();
  const errors = [];
  page.on("console", (message) => {
    if (message.type() === "error") errors.push(message.text());
  });
  page.on("pageerror", (error) => errors.push(error.message));
  await page.goto(url, { waitUntil: "networkidle", timeout: 45_000 });
  await page.emulateMedia({ reducedMotion: "reduce", colorScheme: "light" });
  await page.addStyleTag({
    content:
      "*,*::before,*::after{animation-duration:0s!important;animation-delay:0s!important;transition:none!important;scroll-behavior:auto!important}",
  });
  await page.getByRole("heading", { name: heading }).waitFor();
  await page.screenshot({ path: path.join(outputDir, `${frameId}.png`) });
  results.push({ frameId, errors, url: page.url() });
  return page;
}

const reviewPage = await captureReal(
  "I18_D2_REVIEW_REWRITE_ENTRY",
  `http://127.0.0.1:13001/projects/${projectId}/works/${workId}/review?reportId=${reviewId}`,
  "内容审核",
);
await reviewPage.close();

const failedPage = await captureReal(
  "I18_D4_REWRITE_FAILED",
  "http://127.0.0.1:13001/projects/f82f3d9b-22e0-4259-8484-7215dd36d0ed/works/7d05c348-bdd6-4136-a7ce-e1adf27fa1c0/rewrite?workflowRunId=4bc64cf7-1fed-401c-b295-9dc5eb22a759",
  "重写任务执行失败",
);
await failedPage.close();

const queuedRun = {
  id: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
  runNumber: "WR-ACCEPTANCE",
  projectId,
  stage: "rewrite",
  subjectType: "review_report",
  subjectId: reviewId,
  workflowConfigurationId: configuration.workflowConfigurationId,
  triggerSource: "manual",
  retryOfRunId: null,
  status: "queued",
  inputPayload: { contentItemId: workId },
  outputPayload: null,
  errorCode: null,
  errorMessage: null,
  errorDetails: null,
  configurationSnapshot: {},
  startedAt: null,
  finishedAt: null,
  cancelledAt: null,
  createdAt: "2026-07-30T03:25:15.798298Z",
  updatedAt: "2026-07-30T03:25:15.798298Z",
  version: 1,
};
const runningPage = await context.newPage();
const runningErrors = [];
runningPage.on("console", (message) => {
  if (message.type() === "error") runningErrors.push(message.text());
});
runningPage.on("pageerror", (error) => runningErrors.push(error.message));
await runningPage.route(
  `**/api/v1/reviews/${reviewId}/rewrite-summary`,
  (route) =>
    route.fulfill({
      json: {
        data: {
          ...summary,
          state: "queued",
          canStartRewrite: false,
          activeRun: queuedRun,
          latestRun: queuedRun,
          selectedIssueSummary: {
            total: 1,
            items: [
              {
                reviewIssueId: "e6f4bd38-7291-4218-b449-8f382832666b",
                issueKey: "character-consistency-1",
                position: 1,
                title: "人物行为与设定不一致",
                severity: "warning",
              },
            ],
          },
        },
        request_id: "acceptance-running-summary",
      },
    }),
);
await runningPage.route(
  `**/api/v1/workflow-runs/${queuedRun.id}/events`,
  (route) =>
    route.fulfill({
      json: {
        data: {
          items: [
            {
              id: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
              runId: queuedRun.id,
              eventType: "queued",
              status: "queued",
              payload: {},
              createdAt: queuedRun.createdAt,
            },
          ],
        },
        request_id: "acceptance-running-events",
      },
    }),
);
await runningPage.goto(
  `http://127.0.0.1:13001/projects/${projectId}/works/${workId}/rewrite?reportId=${reviewId}`,
  { waitUntil: "networkidle", timeout: 45_000 },
);
await runningPage.emulateMedia({
  reducedMotion: "reduce",
  colorScheme: "light",
});
await runningPage.addStyleTag({
  content:
    "*,*::before,*::after{animation-duration:0s!important;animation-delay:0s!important;transition:none!important;scroll-behavior:auto!important}",
});
await runningPage
  .getByRole("heading", { name: "重写任务正在排队" })
  .waitFor();
await runningPage.screenshot({
  path: path.join(outputDir, "I18_D4_REWRITE_RUNNING.png"),
});
results.push({
  frameId: "I18_D4_REWRITE_RUNNING",
  errors: runningErrors,
  stateConstruction:
    "deterministic persisted-shape fixture over production UI",
});
await runningPage.close();

const consumptionPage = await captureReal(
  "I18_D5_RESULT_CONSUMPTION_FAILED",
  "http://127.0.0.1:13001/projects/f82f3d9b-22e0-4259-8484-7215dd36d0ed/works/7d05c348-bdd6-4136-a7ce-e1adf27fa1c0/rewrite?workflowRunId=9f33d131-c285-4205-ba6e-477f24ed734f",
  "重写结果提交失败",
);
await consumptionPage.close();

const resultPage = await captureReal(
  "I18_D5_REWRITE_RESULT",
  `http://127.0.0.1:13001/projects/${projectId}/works/${workId}/rewrite?workflowRunId=5f626467-8dc3-4106-ba27-1ce481ae6406`,
  "重写候选版本",
);
await resultPage.getByRole("button", { name: "设为当前版本" }).click();
await resultPage
  .getByRole("dialog", { name: "设为当前版本确认" })
  .waitFor();
await resultPage.screenshot({
  path: path.join(outputDir, "I18_D5_SET_CURRENT_CONFIRM.png"),
});
results.push({
  frameId: "I18_D5_SET_CURRENT_CONFIRM",
  errors: [],
  action: "opened and cancelled; no Set Current mutation",
});
await resultPage.getByRole("button", { name: "取消", exact: true }).click();
await resultPage.close();

await browser.close();
fs.writeFileSync(
  path.join(outputDir, "fixture-browser-check.json"),
  `${JSON.stringify(
    {
      result: results.every((item) => item.errors.length === 0)
        ? "passed"
        : "failed",
      browser: "Chromium",
      viewport: "1440x900",
      deviceScaleFactor: 1,
      locale: "zh-CN",
      timezone: "Asia/Shanghai",
      colorScheme: "light",
      reducedMotion: true,
      stateConstruction:
        "deterministic network fixture over current production UI; no DOM mutation",
      results,
    },
    null,
    2,
  )}\n`,
);

if (results.some((item) => item.errors.length > 0)) process.exitCode = 1;
