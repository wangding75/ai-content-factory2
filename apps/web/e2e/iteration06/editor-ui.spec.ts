import { expect, test, type Page, type Route } from "@playwright/test";
import { loadIteration06Fixtures } from "./qa-fixtures";

function attachAudit(page: Page) {
  const consoleErrors: string[] = [];
  const pageErrors: string[] = [];
  const failedRequests: string[] = [];
  const unexpectedResponses: string[] = [];
  const allowed = new Set<string>();
  const requests: { method: string; url: string; headers: Record<string, string> }[] = [];
  page.on("console", message => {
    const expectedInjectedResponse = message.type() === "error" && /^Failed to load resource: the server responded with a status of (503|409)/.test(message.text());
    if (message.type() === "error" && !expectedInjectedResponse) consoleErrors.push(message.text());
  });
  page.on("pageerror", error => pageErrors.push(error.message));
  page.on("requestfailed", request => {
    const cancelledRsc = request.method() === "GET" && request.url().includes("_rsc=") && request.failure()?.errorText === "net::ERR_ABORTED";
    if (!cancelledRsc) failedRequests.push(`${request.method()} ${request.url()}: ${request.failure()?.errorText}`);
  });
  page.on("request", request => requests.push({ method: request.method(), url: request.url(), headers: request.headers() }));
  page.on("response", response => {
    if (response.url().includes("/api/v1/") && response.status() >= 400 && !allowed.has(`${response.request().method()} ${response.url()} ${response.status()}`)) {
      unexpectedResponses.push(`${response.request().method()} ${response.status()} ${response.url()}`);
    }
    if (response.url().includes("/_next/static/") && response.status() >= 400) unexpectedResponses.push(`static ${response.status()} ${response.url()}`);
  });
  return { consoleErrors, pageErrors, failedRequests, unexpectedResponses, allowed, requests };
}

async function openGenerator(page: Page) {
  await page.getByRole("button", { name: "模拟生成正文" }).click();
  await expect(page.getByRole("dialog", { name: "模拟生成正文" })).toBeVisible();
}

test("Iteration 06 D1 editor UI states preserve real server semantics", async ({ page }) => {
  const fixtures = loadIteration06Fixtures();
  const editorPath = `/projects/${fixtures.project_a_id}/chapter-plans/${fixtures.confirmed_chapter_plan_id}/content`;
  const audit = attachAudit(page);
  let initialGet = 0;
  let releaseInitial: (() => void) | undefined;
  const initialGate = new Promise<void>(resolve => { releaseInitial = resolve; });
  await page.route("**/api/v1/content-items/**", async route => {
    if (route.request().method() !== "GET" || initialGet > 0) return route.fallback();
    initialGet += 1;
    await initialGate;
    audit.allowed.add(`GET ${route.request().url()} 503`);
    await route.fulfill({ status: 503, contentType: "application/json", body: JSON.stringify({ error: { code: "qa_unavailable", message: "QA initial failure", details: {} }, request_id: "qa-503" }) });
  });

  await page.goto(editorPath);
  await expect(page.getByRole("heading", { name: "正在打开正文" })).toBeVisible();
  releaseInitial?.();
  await expect(page.getByRole("heading", { name: "无法打开正文" })).toBeVisible();
  await expect(page.getByRole("button", { name: "重试" })).toBeVisible();
  await page.unroute("**/api/v1/content-items/**");
  await page.getByRole("button", { name: "重试" }).click();
  await expect(page.getByRole("button", { name: "保存草稿" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "无法打开正文" })).toHaveCount(0);

  const generatePostsBeforeDismiss = audit.requests.filter(request => request.method === "POST" && request.url.includes("/mock-generate")).length;
  await openGenerator(page);
  await page.getByRole("textbox", { name: "章节目标" }).fill("cancelled goal");
  await page.getByRole("button", { name: "取消" }).click();
  await expect(page.getByRole("dialog", { name: "模拟生成正文" })).toHaveCount(0);
  await openGenerator(page);
  await expect(page.getByRole("textbox", { name: "章节目标" })).toHaveValue("Exercise D1 UI states");
  await page.getByRole("button", { name: "关闭生成对话框" }).click();
  await expect(page.getByRole("dialog", { name: "模拟生成正文" })).toHaveCount(0);
  expect(audit.requests.filter(request => request.method === "POST" && request.url.includes("/mock-generate")).length).toBe(generatePostsBeforeDismiss);

  await page.getByLabel("标题").fill("saved D1 title");
  await page.getByLabel("正文").fill("one two three four");
  let releaseSave: (() => void) | undefined;
  const saveGate = new Promise<void>(resolve => { releaseSave = resolve; });
  let saveCount = 0;
  await page.route("**/api/v1/content-items/*/draft", async route => {
    saveCount += 1;
    const response = await route.fetch();
    await saveGate;
    await route.fulfill({ response });
  });
  const saveButton = page.getByRole("button", { name: "保存草稿" });
  await saveButton.click();
  const savingButton = page.getByRole("button", { name: "保存中…" });
  await expect(savingButton).toBeDisabled();
  await savingButton.click({ force: true });
  expect(saveCount).toBe(1);
  releaseSave?.();
  await expect(page.getByText("已保存")).toBeVisible();
  await expect(page.getByRole("button", { name: "保存草稿" })).toBeDisabled();
  await expect(page.getByText("4 字", { exact: true }).first()).toBeVisible();
  await page.getByLabel("标题").fill("saved D1 title ready");
  await expect(page.getByRole("button", { name: "保存草稿" })).toBeEnabled();
  await page.unroute("**/api/v1/content-items/*/draft");

  await openGenerator(page);
  let releaseGenerate: (() => void) | undefined;
  const generateGate = new Promise<void>(resolve => { releaseGenerate = resolve; });
  let generateCount = 0;
  await page.route("**/api/v1/content-items/*/mock-generate", async route => {
    generateCount += 1;
    const response = await route.fetch();
    await generateGate;
    await route.fulfill({ response });
  });
  const submitGenerate = page.getByRole("button", { name: "开始生成" });
  await submitGenerate.click();
  const generatingButton = page.getByRole("button", { name: "生成中…" });
  await expect(generatingButton).toBeDisabled();
  await generatingButton.click({ force: true });
  expect(generateCount).toBe(1);
  releaseGenerate?.();
  await expect(page.getByRole("dialog", { name: "模拟生成正文" })).toHaveCount(0);
  await expect(page.getByText("当前正文由模拟生成产生。您可以直接修改或通过左侧章节规划重新生成。", { exact: true })).toBeVisible();
  await expect(page.getByText("v1", { exact: true }).first()).toBeVisible();
  await page.unroute("**/api/v1/content-items/*/mock-generate");

  await page.getByLabel("标题").fill("stale title retained");
  await page.getByLabel("正文").fill("stale body retained");
  await page.getByLabel("章节摘要").fill("stale summary retained");
  const itemEnvelope = await (await page.request.post(`/api/v1/chapter-plans/${fixtures.confirmed_chapter_plan_id}/content`)).json();
  const itemId = itemEnvelope.data.content_item.id as string;
  const detail = await (await page.request.get(`/api/v1/content-items/${itemId}`)).json();
  const current = detail.data.current_version;
  await page.request.put(`/api/v1/content-items/${itemId}/draft`, { data: { expected_version: current.version, title: "concurrent server title" } });
  audit.allowed.add(`PUT ${page.url().replace(/\/projects.*/, "")}/api/v1/content-items/${itemId}/draft 409`);
  await page.getByRole("button", { name: "保存草稿" }).click();
  await expect(page.getByText("保存失败")).toBeVisible();
  await expect(page.getByLabel("标题")).toHaveValue("stale title retained");
  await expect(page.getByLabel("正文")).toHaveValue("stale body retained");
  await expect(page.getByLabel("章节摘要")).toHaveValue("stale summary retained");
  const afterConflict = await (await page.request.get(`/api/v1/content-items/${itemId}`)).json();
  expect(afterConflict.data.current_version.title).toBe("concurrent server title");
  expect(afterConflict.data.current_version.content).not.toBe("stale body retained");

  await page.reload();
  await expect(page.getByRole("button", { name: "保存草稿" })).toBeVisible();
  await openGenerator(page);
  await page.getByRole("textbox", { name: "章节目标" }).fill("retry goal retained");
  const idempotencyKeys: string[] = [];
  let failedGenerateCount = 0;
  await page.route("**/api/v1/content-items/*/mock-generate", async route => {
    failedGenerateCount += 1;
    idempotencyKeys.push(route.request().headers()["idempotency-key"] ?? "");
    audit.allowed.add(`POST ${route.request().url()} 409`);
    await route.fulfill({ status: 409, contentType: "application/json", body: JSON.stringify({ error: { code: "version_conflict", message: "QA frozen conflict", details: {} }, request_id: "qa-409" }) });
  });
  const beforeFailure = await (await page.request.get(`/api/v1/content-items/${itemId}`)).json();
  await page.getByRole("button", { name: "开始生成" }).click();
  await expect(page.getByText("版本或幂等请求发生冲突。请检查后重试。")).toBeVisible();
  await expect(page.getByRole("textbox", { name: "章节目标" })).toHaveValue("retry goal retained");
  await page.getByRole("button", { name: "开始生成" }).click();
  expect(failedGenerateCount).toBe(2);
  expect(idempotencyKeys[0]).toBeTruthy();
  expect(idempotencyKeys[1]).toBe(idempotencyKeys[0]);
  const afterFailure = await (await page.request.get(`/api/v1/content-items/${itemId}`)).json();
  expect(afterFailure.data.current_version.version).toBe(beforeFailure.data.current_version.version);
  expect(afterFailure.data.current_version.content).toBe(beforeFailure.data.current_version.content);
  await page.unroute("**/api/v1/content-items/*/mock-generate");

  expect(audit.consoleErrors).toEqual([]);
  expect(audit.pageErrors).toEqual([]);
  expect(audit.failedRequests).toEqual([]);
  expect(audit.unexpectedResponses).toEqual([]);
});
