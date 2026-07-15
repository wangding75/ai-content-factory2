import { expect, test, type Page } from "@playwright/test";
import { loadIteration06Fixtures } from "./qa-fixtures";
type RequestRecord = { method: string; url: string; headers: Record<string, string>; body?: string | null };

function audit(page: Page) {
  const consoleErrors: string[] = [], pageErrors: string[] = [], failedRequests: string[] = [], unexpectedResponses: string[] = [];
  const expectedStatuses = new Set<number>();
  const requests: RequestRecord[] = [];
  page.on("console", message => {
    const expected = message.type() === "error" && /^Failed to load resource: the server responded with a status of (503|409|404)/.test(message.text());
    if (message.type() === "error" && !expected) consoleErrors.push(message.text());
  });
  page.on("pageerror", error => pageErrors.push(error.message));
  page.on("requestfailed", request => {
    const rscAbort = request.method() === "GET" && request.url().includes("_rsc=") && request.failure()?.errorText === "net::ERR_ABORTED";
    if (!rscAbort) failedRequests.push(`${request.method()} ${request.url()}: ${request.failure()?.errorText}`);
  });
  page.on("request", request => requests.push({ method: request.method(), url: request.url(), headers: request.headers(), body: request.postData() }));
  page.on("response", response => {
    if (response.url().includes("/_next/static/") && response.status() >= 400) unexpectedResponses.push(`static ${response.status()} ${response.url()}`);
    if (response.url().includes("/api/v1/") && response.status() >= 400 && !expectedStatuses.has(response.status())) unexpectedResponses.push(`${response.request().method()} ${response.status()} ${response.url()}`);
  });
  return { consoleErrors, pageErrors, failedRequests, unexpectedResponses, expectedStatuses, requests };
}

async function contentDetail(page: Page, itemId: string) {
  return (await (await page.request.get(`/api/v1/content-items/${itemId}`)).json()).data;
}

test("Iteration 06 D2 review UI preserves review state and frozen constraints", async ({ page }) => {
  const fixtures = loadIteration06Fixtures();
  const reviewPath = `/projects/${fixtures.project_a_id}/chapter-plans/${fixtures.confirmed_chapter_plan_id}/content/review`;
  const events = audit(page);
  const bootstrap = await (await page.request.post(`/api/v1/chapter-plans/${fixtures.confirmed_chapter_plan_id}/content`)).json();
  const itemId = bootstrap.data.content_item.id as string;
  const versionId = bootstrap.data.current_version.id as string;
  const originalContent = bootstrap.data.current_version.content as string;

  let releaseHistory: (() => void) | undefined;
  const historyGate = new Promise<void>(resolve => { releaseHistory = resolve; });
  let historyIntercepts = 0;
  await page.route("**/api/v1/content-items/*/reviews?*", async route => {
    if (historyIntercepts++ > 0) return route.fallback();
    await historyGate;
    events.expectedStatuses.add(503);
    await route.fulfill({ status: 503, contentType: "application/json", body: JSON.stringify({ error: { code: "qa_history_unavailable", message: "QA history unavailable", details: {} }, request_id: "qa-history-503" }) });
  });

  await page.goto(reviewPath);
  await expect(page.getByRole("heading", { name: "正在加载审核" })).toBeVisible();
  releaseHistory?.();
  await expect(page.getByRole("heading", { name: "审核历史加载失败" })).toBeVisible();
  await expect(page.getByRole("button", { name: "重试" })).toBeVisible();
  await page.unroute("**/api/v1/content-items/*/reviews?*");
  await page.getByRole("button", { name: "重试" }).click();
  await expect(page.getByText("尚无审核记录。请发起模拟审核。", { exact: true })).toBeVisible();
  await expect(page.getByRole("heading", { name: "审核历史加载失败" })).toHaveCount(0);

  const reviewPostsBeforeCancel = () => events.requests.filter(request => request.method === "POST" && request.url.includes("/reviews/mock"));
  await page.getByRole("button", { name: "发起模拟审核" }).click();
  await expect(page.getByRole("dialog", { name: "发起模拟审核" })).toBeVisible();
  await page.getByRole("button", { name: "取消" }).click();
  await expect(page.getByRole("dialog", { name: "发起模拟审核" })).toHaveCount(0);
  await page.getByRole("button", { name: "发起模拟审核" }).click();
  await page.getByRole("button", { name: "关闭" }).click();
  expect(reviewPostsBeforeCancel()).toHaveLength(0);
  const beforeFailure = await contentDetail(page, itemId);
  expect(beforeFailure.content_item.status).toBe("draft");
  expect(beforeFailure.current_version.status).toBe("editable_draft");
  const initialHistory = await (await page.request.get(`/api/v1/content-items/${itemId}/reviews?limit=10&offset=0`)).json();
  expect(initialHistory.data.total).toBe(0);

  const failedKeys: string[] = [];
  let failedPosts = 0;
  await page.route("**/api/v1/content-items/*/reviews/mock", async route => {
    failedPosts += 1;
    failedKeys.push(route.request().headers()["idempotency-key"] ?? "");
    events.expectedStatuses.add(409);
    await route.fulfill({ status: 409, contentType: "application/json", body: JSON.stringify({ error: { code: "version_conflict", message: "QA review conflict", details: {} }, request_id: "qa-review-409" }) });
  });
  await page.getByRole("button", { name: "发起模拟审核" }).click();
  await page.getByRole("button", { name: "确认发起审核" }).click();
  await expect(page.getByText("正文版本已发生变化，请返回编辑器刷新后重试。", { exact: true })).toBeVisible();
  await expect(page.getByRole("dialog", { name: "发起模拟审核" })).toBeVisible();
  await page.getByRole("button", { name: "确认发起审核" }).click();
  expect(failedPosts).toBe(2);
  expect(failedKeys[0]).toBeTruthy();
  expect(failedKeys[1]).toBe(failedKeys[0]);
  const afterFailure = await contentDetail(page, itemId);
  expect(afterFailure.content_item.status).toBe("draft");
  expect(afterFailure.current_version.status).toBe("editable_draft");
  expect((await (await page.request.get(`/api/v1/content-items/${itemId}/reviews?limit=10&offset=0`)).json()).data.total).toBe(0);
  await page.unroute("**/api/v1/content-items/*/reviews/mock");

  let releaseReview: (() => void) | undefined;
  const reviewGate = new Promise<void>(resolve => { releaseReview = resolve; });
  let reviewPosts = 0;
  let reviewPayload: { review?: { id: string; content_version_id: string }; findings?: { title: string }[]; recommendations?: { title: string }[]; workflow_run?: { id: string; status: string } } | undefined;
  await page.route("**/api/v1/content-items/*/reviews/mock", async route => {
    reviewPosts += 1;
    const response = await route.fetch();
    reviewPayload = (await response.json()).data;
    await reviewGate;
    await route.fulfill({ response });
  });
  let releaseDetail: (() => void) | undefined;
  const detailGate = new Promise<void>(resolve => { releaseDetail = resolve; });
  let detailIntercepts = 0;
  await page.route("**/api/v1/reviews/*", async route => {
    if (detailIntercepts++ > 0) return route.fallback();
    await detailGate;
    events.expectedStatuses.add(503);
    await route.fulfill({ status: 503, contentType: "application/json", body: JSON.stringify({ error: { code: "qa_detail_unavailable", message: "QA detail unavailable", details: {} }, request_id: "qa-detail-503" }) });
  });
  const confirm = page.getByRole("button", { name: "确认发起审核" });
  await confirm.click();
  const submitting = page.getByRole("button", { name: "提交中…" });
  await expect(submitting).toBeDisabled();
  await submitting.click({ force: true });
  expect(reviewPosts).toBe(1);
  const submitted = reviewPostsBeforeCancel()[reviewPostsBeforeCancel().length - 1];
  expect(JSON.parse(submitted.body ?? "{}")).toEqual({ content_version_id: versionId, expected_version: beforeFailure.current_version.version });
  expect(submitted.headers["idempotency-key"]).toBeTruthy();
  releaseReview?.();
  await expect(page.getByRole("dialog", { name: "发起模拟审核" })).toHaveCount(0);
  await expect(page.getByText("内容状态：已审核", { exact: false })).toBeVisible();
  await expect(page.getByText("正在加载审核详情…", { exact: true })).toBeVisible();
  releaseDetail?.();
  await expect(page.getByRole("heading", { name: "审核详情不可用" })).toBeVisible();
  await page.unroute("**/api/v1/reviews/*");
  await page.getByRole("button", { name: "重试" }).click();
  await expect(page.getByText("审核时固定正文", { exact: true })).toBeVisible();

  const afterSuccess = await contentDetail(page, itemId);
  expect(afterSuccess.content_item.status).toBe("reviewed");
  expect(afterSuccess.current_version.status).toBe("frozen");
  expect(afterSuccess.current_version.content).toBe(originalContent);
  expect(afterSuccess.current_version.version_no).toBe(1);
  expect(reviewPayload?.review?.content_version_id).toBe(versionId);
  expect(reviewPayload?.workflow_run?.status).toBe("succeeded");
  await expect(page.getByText("审核历史（1）", { exact: true })).toBeVisible();
  await expect(page.getByText("共 1 条", { exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "上一页" })).toBeDisabled();
  await expect(page.getByRole("button", { name: "下一页" })).toBeDisabled();
  await expect(page.getByRole("heading", { name: "问题清单（2）" })).toBeVisible();
  const findingTitles = await page.locator(".content-review-findings h4").allTextContents();
  const recommendationTitles = await page.locator(".content-review-recommendations li b").allTextContents();
  expect(findingTitles).toEqual(reviewPayload?.findings?.map(finding => finding.title));
  expect(recommendationTitles).toEqual(reviewPayload?.recommendations?.map(recommendation => recommendation.title));

  const reviewId = reviewPayload?.review?.id;
  if (!reviewId) throw new Error("Mock review response did not include a review ID.");
  await page.route(`**/api/v1/reviews/${reviewId}`, async route => {
    events.expectedStatuses.add(404);
    await route.fulfill({ status: 404, contentType: "application/json", body: JSON.stringify({ error: { code: "review_not_found", message: "QA safe missing review", details: {} }, request_id: "qa-detail-404" }) });
  });
  await page.reload();
  await expect(page.getByRole("heading", { name: "审核详情不可用" })).toBeVisible();
  await expect(page.getByText("所请求的审核记录不存在或已不可用。", { exact: true })).toBeVisible();
  await expect(page.locator("text=internal")).toHaveCount(0);
  await page.unroute(`**/api/v1/reviews/${reviewId}`);
  await page.getByRole("button", { name: "重试" }).click();
  await expect(page.getByText("审核时固定正文", { exact: true })).toBeVisible();

  const listRequests = events.requests.filter(request => request.method === "GET" && request.url.includes(`/api/v1/content-items/${itemId}/reviews?`) && request.url.includes("limit=10") && request.url.includes("offset=0"));
  const detailRequests = events.requests.filter(request => request.method === "GET" && request.url.includes(`/api/v1/reviews/${reviewId}`));
  expect(listRequests.length).toBeGreaterThanOrEqual(3);
  expect(detailRequests.length).toBe(4); // injected 503 + retry + injected 404 + retry; never per-list N+1.

  await page.getByRole("link", { name: "返回编辑" }).click();
  await expect(page.getByLabel("标题")).toBeDisabled();
  await expect(page.getByLabel("正文")).toBeDisabled();
  await expect(page.getByLabel("章节摘要")).toBeDisabled();
  await expect(page.getByRole("button", { name: "保存草稿" })).toBeDisabled();
  await expect(page.getByRole("button", { name: "模拟生成正文" })).toBeDisabled();
  await page.getByRole("link", { name: "查看审核结果" }).click();
  await expect(page.getByRole("button", { name: "发起模拟审核" })).toHaveCount(0);
  const rewrite = page.getByRole("button", { name: "创建重写版本" });
  const postsBeforeRewrite = reviewPostsBeforeCancel().length;
  await expect(rewrite).toBeDisabled();
  await rewrite.click({ force: true });
  expect(reviewPostsBeforeCancel()).toHaveLength(postsBeforeRewrite);
  await expect(page).toHaveURL(/\/content\/review$/);

  expect(events.consoleErrors).toEqual([]);
  expect(events.pageErrors).toEqual([]);
  expect(events.failedRequests).toEqual([]);
  expect(events.unexpectedResponses).toEqual([]);
});
