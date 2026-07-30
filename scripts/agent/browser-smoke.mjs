import fs from "node:fs";
import { createRequire } from "node:module";
import path from "node:path";
import process from "node:process";

function readArg(name) {
  const index = process.argv.indexOf(name);
  return index >= 0 ? process.argv[index + 1] : undefined;
}

const configPath = readArg("--config");
const defaultContentRoute = "/projects/13362446-e528-4071-a9d1-5260cc1fcd78/works/9ea091d1-b63e-409c-92dd-9b1271c508c7";
const config = configPath
  ? JSON.parse(fs.readFileSync(path.resolve(configPath), "utf8"))
  : {
      baseUrl: process.env.ACF_WEB_BASE_URL ?? "http://127.0.0.1:13001",
      routes: ["/", defaultContentRoute],
      playwrightPackageJson: path.resolve("apps/web/package.json"),
      allowedFailurePatterns: ["favicon\\.ico", "net::ERR_ABORTED", "fonts\\.googleapis\\.com", "fonts\\.gstatic\\.com", "net::ERR_CONNECTION_CLOSED"],
      contentGenerationEvidence: { route: defaultContentRoute },
    };

let chromium;
try {
  if (!config.playwrightPackageJson) {
    throw new Error("Missing playwrightPackageJson in smoke configuration.");
  }
  const requireFromWeb = createRequire(path.resolve(config.playwrightPackageJson));
  ({ chromium } = requireFromWeb("@playwright/test"));
} catch (error) {
  console.error("Unable to load @playwright/test from apps/web.");
  console.error(error);
  process.exit(2);
}

const browser = await chromium.launch({ headless: true });
const results = [];
const exactPageOptions = {
  viewport: {
    width: config.viewport?.width ?? 1440,
    height: config.viewport?.height ?? 900,
  },
  deviceScaleFactor: 1,
  locale: "zh-CN",
  timezoneId: "Asia/Shanghai",
  colorScheme: "light",
  reducedMotion: "reduce",
};
const disableMotion = async (page) => {
  await page.addStyleTag({
    content:
      "*,*::before,*::after{animation-duration:0s!important;animation-delay:0s!important;transition:none!important;scroll-behavior:auto!important}",
  });
};
const captureEvidence = async (page, outputDir, frameId, options = {}) => {
  if (!outputDir) return;
  fs.mkdirSync(path.resolve(outputDir), { recursive: true });
  await page.screenshot({
    path: path.join(path.resolve(outputDir), `${frameId}.png`),
    fullPage: options.fullPage ?? false,
  });
};

const assertVisible = async (page, text) => {
  const locator = page.getByText(text, { exact: false }).first();
  await locator.waitFor({ state: "visible", timeout: 5000 });
  return locator;
};

const installContentGenerationFixture = async (page, itemId, state, counters) => {
  let currentVersion = null;
  await page.route("**/*", async (route) => {
    const request = route.request();
    const url = request.url();
    if (!url.includes(`/content-items/${itemId}`)) return route.continue();
    if (url.includes("/content-generation-runs/preflight")) {
      const sourceVersion = currentVersion ?? { version_no: 1 };
      return route.fulfill({ json: { data: { status: "passed", preflightToken: "browser-fixture-token", expiresAt: "2030-01-01T00:00:00Z", sourceVersion, targetVersionNo: (sourceVersion?.version_no ?? 1) + 1, workflow: { name: "浏览器 Fixture 工作流", inputContractVersion: "v1", outputContractVersion: "v1", configurationVersion: 1 }, contextSummary: { chapterGoalCount: 1, keyPlotCount: 0, priorChapterCount: 0, materialCount: 0, storylineCount: 0, foreshadowingCount: 0 }, checks: [{ code: "fixture", status: "passed", message: "浏览器 UI 状态验证" }] }, request_id: "browser-fixture-preflight" } });
    }
    if (url.endsWith("/current-version") && request.method() === "POST") {
      counters.cas += 1;
      return route.fulfill({ json: { data: {}, request_id: "browser-fixture-cas" } });
    }
    const response = await route.fetch();
    const body = await response.json();
    const data = body.data;
    if (url.includes("/content-generation-summary")) {
      const current = data.currentVersion;
      currentVersion = current;
      const run = ["queued", "running"].includes(state) ? { id: "browser-fixture-run", runNumber: "Fixture Run", status: state, version: 1, startedAt: null, finishedAt: null, createdAt: "2030-01-01T00:00:00Z", errorMessage: null } : state.endsWith("failed") ? { id: "browser-fixture-run", runNumber: "Fixture Run", status: "failed", version: 1, startedAt: null, finishedAt: "2030-01-01T00:00:00Z", errorMessage: "安全失败说明" } : null;
      data.state = state;
      data.workflowConfigured = state !== "not_configured";
      data.canGenerate = !["queued", "running"].includes(state);
      data.activeRun = ["queued", "running"].includes(state) ? run : null;
      data.latestRun = run;
      data.latestError = state.endsWith("failed") ? { code: "fixture", message: "安全失败说明", details: {} } : null;
      if (state === "candidate_ready" || state === "stale_candidate") {
        data.state = "candidate_ready";
        data.latestCandidateVersion = { ...current, id: "browser-fixture-candidate", version_no: current.version_no + 1, version: current.version + 1, source: "workflow_generated", title: "浏览器 Fixture 候选正文", content: "候选正文用于浏览器 UI 状态验证。" };
        data.candidateCanBecomeCurrent = state !== "stale_candidate";
      } else data.latestCandidateVersion = null;
      return route.fulfill({ json: body });
    }
    if (url.endsWith(`/content-items/${itemId}`) && request.method() === "GET") {
      data.content_item.status = "draft";
      data.content_item.reviewed_at = null;
      data.current_version.status = "editable_draft";
      currentVersion = data.current_version;
      return route.fulfill({ json: body });
    }
    return route.fulfill({ response, body: JSON.stringify(body), contentType: "application/json" });
  });
};

const runContentGenerationEvidence = async (baseUrl, evidence) => {
  const itemId = evidence.route.split("/").at(-1);
  const requestedState = readArg("--content-generation-state");
  const states = requestedState ? [requestedState] : ["not_configured", "queued", "running", "runtime_failed", "output_validation_failed", "result_consumption_failed", "candidate_ready", "stale_candidate"];
  for (const state of states) {
    const page = await browser.newPage(exactPageOptions);
    const counters = { cas: 0 };
    const result = { route: evidence.route, evidenceState: state, browserFixture: state !== "not_configured", assertions: [], errors: [] };
    try {
      if (state !== "not_configured") await installContentGenerationFixture(page, itemId, state, counters);
      await page.goto(new URL(evidence.route, baseUrl).toString(), { waitUntil: "networkidle", timeout: 45000 });
      await disableMotion(page);
      if (state === "not_configured") await assertVisible(page, "尚未配置正文生成工作流");
      if (state === "queued") await assertVisible(page, "排队中");
      if (state === "running") await assertVisible(page, "运行中");
      if (state.endsWith("failed")) { await assertVisible(page, "安全失败说明"); await assertVisible(page, state === "result_consumption_failed" ? "重试结果消费" : "重新执行 Runtime"); }
      if (state === "candidate_ready" && evidence.visualOutputDir) {
        await captureEvidence(page, evidence.visualOutputDir, "I16_D3_RUN_SUCCEEDED");
        await captureEvidence(page, evidence.visualOutputDir, "I16_D1_EDITOR_CHAPTER_GOAL");
        await page.getByRole("tab", { name: "故事情报" }).click();
        await captureEvidence(page, evidence.visualOutputDir, "I16_D1_EDITOR_STORY_CONTEXT");
        await page.getByRole("tab", { name: "素材库" }).click();
        await captureEvidence(page, evidence.visualOutputDir, "I16_D1_EDITOR_MATERIALS");
        await page.getByRole("button", { name: "生成正文" }).click();
        await assertVisible(page, "预检通过");
        await captureEvidence(page, evidence.visualOutputDir, "I16_D2_GENERATE_CONFIRM", { fullPage: true });
        await page.getByPlaceholder("可选：补充本次创作要求").fill("保持叙事视角并强化人物动机");
        await captureEvidence(page, evidence.visualOutputDir, "I16_D2_GENERATE_REQUIREMENTS", { fullPage: true });
        await page.getByRole("button", { name: "关闭生成正文抽屉" }).click();
      }
      if (state === "candidate_ready" || state === "stale_candidate") {
        await (await assertVisible(page, "查看候选")).click();
        await assertVisible(page, "候选正文用于浏览器 UI 状态验证");
        if (state === "candidate_ready") {
          await captureEvidence(page, evidence.visualOutputDir, "I16_D4_CANDIDATE_VERSION", { fullPage: true });
        }
        const apply = await assertVisible(page, "设为当前版本");
        if (state === "stale_candidate") {
          if (await apply.isEnabled()) throw new Error("Stale candidate must not be applicable");
          await assertVisible(page, "当前正文已变化");
        } else {
          await apply.click(); await assertVisible(page, "确认将候选版本设为当前正文吗");
          await (await assertVisible(page, "取消切换")).click();
          if (counters.cas !== 0) throw new Error("Cancel sent CAS request");
          await apply.click(); await (await assertVisible(page, "确认设为当前版本")).click(); await page.waitForTimeout(300);
          if (counters.cas !== 1) throw new Error(`Expected one CAS request, received ${counters.cas}`);
        }
      }
      if (state === "queued") {
        const body = page.locator("textarea.content-body-input");
        await body.fill("未保存草稿在轮询后仍保留");
        await page.waitForTimeout(5500);
        if ((await body.inputValue()) !== "未保存草稿在轮询后仍保留") throw new Error("Polling overwrote unsaved draft");
      }
      if (state === "candidate_ready") {
        await (await assertVisible(page, "生成正文")).click();
        await assertVisible(page, "预检通过");
        await page.getByPlaceholder("可选：补充本次创作要求").fill("变更后的补充要求");
        if (await page.getByText("确认创建生成任务", { exact: true }).isVisible()) throw new Error("Instruction change did not invalidate preflight");
        await (await assertVisible(page, "重新预检")).click(); await assertVisible(page, "预检通过");
        const options = page.locator(".content-generation-drawer input[type=checkbox]");
        for (let index = 0; index < 4; index += 1) { await options.nth(index).click(); if (await page.getByText("确认创建生成任务", { exact: true }).isVisible()) throw new Error(`Context option ${index + 1} did not invalidate preflight`); await (await assertVisible(page, "重新预检")).click(); await assertVisible(page, "预检通过"); }
      }
      const stateFrame = {
        not_configured: "I16_D5_NOT_CONFIGURED",
        queued: "I16_D3_RUN_QUEUED",
        running: "I16_D3_RUN_RUNNING",
        runtime_failed: "I16_D3_RUN_FAILED",
        candidate_ready: "I16_D3_RUN_SUCCEEDED",
      }[state];
      if (stateFrame && state !== "candidate_ready") {
        await captureEvidence(page, evidence.visualOutputDir, stateFrame, {
          fullPage: state === "not_configured",
        });
      }
      result.assertions.push("passed");
    } catch (error) { result.errors.push(String(error)); }
    results.push(result);
    await page.close();
  }
};

try {
  for (const route of config.routes) {
    const page = await browser.newPage({
      ...exactPageOptions,
    });

    const consoleErrors = [];
    const pageErrors = [];
    const failedResponses = [];
    const failedRequests = [];

    page.on("console", (message) => {
      if (message.type() === "error") consoleErrors.push(message.text());
    });
    page.on("pageerror", (error) => pageErrors.push(String(error)));
    page.on("response", (response) => {
      if (response.status() >= 400) {
        failedResponses.push(`${response.status()} ${response.url()}`);
      }
    });
    page.on("requestfailed", (request) => {
      failedRequests.push(`${request.failure()?.errorText ?? "failed"} ${request.url()}`);
    });

    const url = new URL(route, config.baseUrl).toString();
    const response = await page.goto(url, {
      waitUntil: "networkidle",
      timeout: config.timeoutMs ?? 45000,
    });

    if (!response || response.status() >= 400) {
      throw new Error(`Route failed: ${url}, status=${response?.status() ?? "none"}`);
    }

    await page.waitForTimeout(config.settleMs ?? 500);
    await disableMotion(page);

    const pageState = await page.evaluate(() => ({
      text: document.body?.innerText ?? "",
      scrollWidth: document.documentElement.scrollWidth,
      clientWidth: document.documentElement.clientWidth,
      title: document.title,
    }));

    const forbiddenMatches = [];
    for (const patternText of config.forbiddenPatterns ?? []) {
      const pattern = new RegExp(patternText, "i");
      if (pattern.test(pageState.text)) forbiddenMatches.push(patternText);
    }

    const allowedFailurePatterns = (config.allowedFailurePatterns ?? []).map(
      (value) => new RegExp(value, "i"),
    );

    const filterAllowed = (items) =>
      items.filter(
        (item) => !allowedFailurePatterns.some((pattern) => pattern.test(item)),
      );

    const routeResult = {
      route,
      url,
      title: pageState.title,
      horizontalOverflow: pageState.scrollWidth > pageState.clientWidth + 1,
      consoleErrors: filterAllowed(consoleErrors),
      pageErrors,
      failedResponses: filterAllowed(failedResponses),
      failedRequests: filterAllowed(failedRequests),
      forbiddenMatches,
    };

    results.push(routeResult);
    await page.close();
  }
  if (config.contentGenerationEvidence) await runContentGenerationEvidence(config.baseUrl, config.contentGenerationEvidence);
} finally {
  await browser.close();
}

const failures = results.filter(
  (item) =>
    item.horizontalOverflow ||
    item.consoleErrors?.length ||
    item.pageErrors?.length ||
    item.failedResponses?.length ||
    item.failedRequests?.length ||
    item.forbiddenMatches?.length ||
    item.errors?.length,
);

const output = {
  result: failures.length === 0 ? "passed" : "failed",
  checkedAt: new Date().toISOString(),
  results,
};

if (config.outputPath) {
  fs.mkdirSync(path.dirname(path.resolve(config.outputPath)), { recursive: true });
  fs.writeFileSync(
    path.resolve(config.outputPath),
    `${JSON.stringify(output, null, 2)}\n`,
    "utf8",
  );
}

console.log(JSON.stringify(output, null, 2));
if (failures.length > 0) process.exit(1);
