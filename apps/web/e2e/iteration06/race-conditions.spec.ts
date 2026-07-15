import { expect, test, type Page } from "@playwright/test";
import { loadIteration06Fixtures, requireProjectBFixtures } from "./qa-fixtures";
type Detail = { content_item: { id: string; status: string; chapter_plan_id: string }; current_version: { id: string; version: number; version_no: number; status: string; source: string; title: string; content: string; summary: string | null } };
const editor = (project: string, plan: string) => `/projects/${project}/chapter-plans/${plan}/content`;
const review = (project: string, plan: string) => `${editor(project, plan)}/review`;

function attachAudit(page: Page) {
  const consoleErrors: string[] = [], pageErrors: string[] = [], failed: string[] = [], responses: string[] = [], requests: string[] = [];
  page.on("console", m => { if (m.type() === "error") consoleErrors.push(m.text()); });
  page.on("pageerror", e => pageErrors.push(e.message));
  page.on("requestfailed", r => {
    const allowedAbort = r.failure()?.errorText === "net::ERR_ABORTED";
    if (!allowedAbort) failed.push(`${r.method()} ${r.url()}: ${r.failure()?.errorText}`);
  });
  page.on("request", r => requests.push(`${r.method()} ${r.url()}`));
  page.on("response", r => {
    if (r.status() >= 400 || (r.url().includes("/_next/static/") && r.status() >= 400)) responses.push(`${r.request().method()} ${r.status()} ${r.url()}`);
  });
  return { consoleErrors, pageErrors, failed, responses, requests };
}

async function createOrGet(page: Page, planId: string): Promise<Detail> {
  return (await (await page.request.post(`/api/v1/chapter-plans/${planId}/content`)).json()).data;
}
async function getDetail(page: Page, itemId: string): Promise<Detail> {
  return (await (await page.request.get(`/api/v1/content-items/${itemId}`)).json()).data;
}
async function waitEditor(page: Page) { await expect(page.getByRole("button", { name: "保存草稿" })).toBeVisible(); }
async function waitReview(page: Page) { await expect(page.getByRole("button", { name: "创建重写版本" })).toBeVisible(); }

test("Iteration 06 D1/D2 late responses cannot cross project or resource boundaries", async ({ page }) => {
  const fixtures = loadIteration06Fixtures();
  const projectB = requireProjectBFixtures(fixtures);
  const events = attachAudit(page);
  const a0 = await createOrGet(page, fixtures.confirmed_chapter_plan_id);
  const b0 = await createOrGet(page, projectB.confirmed_chapter_plan_b_id);
  const aItem = a0.content_item.id, bItem = b0.content_item.id;
  const aEditor = editor(fixtures.project_a_id, fixtures.confirmed_chapter_plan_id);
  const bEditor = editor(projectB.project_b_id, projectB.confirmed_chapter_plan_b_id);
  const aReview = review(fixtures.project_a_id, fixtures.confirmed_chapter_plan_id);
  const bReview = review(projectB.project_b_id, projectB.confirmed_chapter_plan_b_id);

  let releaseSave: (() => void) | undefined, saveReached = false;
  const saveGate = new Promise<void>(resolve => { releaseSave = resolve; });
  await page.goto(aEditor); await waitEditor(page);
  await page.getByLabel("标题").fill("A late save title");
  await page.getByLabel("正文").fill("A late save body");
  await page.getByLabel("章节摘要").fill("A late save summary");
  await page.route(`**/api/v1/content-items/${aItem}/draft`, async route => { const response = await route.fetch(); saveReached = true; await saveGate; await route.fulfill({ response }); });
  await page.getByRole("button", { name: "保存草稿" }).click();
  await expect.poll(() => saveReached).toBe(true);
  const saveSubmitUrl = new URL(page.url()).pathname;
  await page.goto(bEditor); await waitEditor(page);
  const saveSwitchUrl = new URL(page.url()).pathname;
  await expect(page.getByLabel("标题")).toHaveValue("I06 confirmed chapter B");
  releaseSave?.(); await page.waitForTimeout(250);
  expect(new URL(page.url()).pathname).toBe(bEditor);
  await expect(page.getByLabel("标题")).toHaveValue("I06 confirmed chapter B");
  expect((await getDetail(page, aItem)).current_version.title).toBe("A late save title");
  expect((await getDetail(page, bItem)).current_version.title).toBe("I06 confirmed chapter B");
  await page.unroute(`**/api/v1/content-items/${aItem}/draft`);

  let releaseGenerate: (() => void) | undefined, generateReached = false;
  const generateGate = new Promise<void>(resolve => { releaseGenerate = resolve; });
  await page.goto(aEditor); await waitEditor(page);
  await page.getByRole("button", { name: "模拟生成正文" }).click();
  await page.getByRole("textbox", { name: "章节目标" }).fill("A delayed generate goal");
  await page.route(`**/api/v1/content-items/${aItem}/mock-generate`, async route => { const response = await route.fetch(); generateReached = true; await generateGate; await route.fulfill({ response }); });
  await page.getByRole("button", { name: "开始生成" }).click();
  await expect.poll(() => generateReached).toBe(true);
  const generateSubmitUrl = new URL(page.url()).pathname;
  await page.goto(bEditor); await waitEditor(page);
  const generateSwitchUrl = new URL(page.url()).pathname;
  releaseGenerate?.(); await page.waitForTimeout(250);
  expect(new URL(page.url()).pathname).toBe(bEditor);
  await expect(page.getByText("手动创建", { exact: true })).toBeVisible();
  await expect(page.getByRole("dialog", { name: "模拟生成正文" })).toHaveCount(0);
  expect((await getDetail(page, aItem)).current_version.source).toBe("mock_generated");
  expect((await getDetail(page, bItem)).current_version.source).toBe("manual_created");
  await page.unroute(`**/api/v1/content-items/${aItem}/mock-generate`);

  let releaseReview: (() => void) | undefined, reviewReached = false;
  const reviewGate = new Promise<void>(resolve => { releaseReview = resolve; });
  await page.goto(aReview); await waitReview(page);
  await page.getByRole("button", { name: "发起模拟审核" }).click();
  await page.route(`**/api/v1/content-items/${aItem}/reviews/mock`, async route => { const response = await route.fetch(); reviewReached = true; await reviewGate; await route.fulfill({ response }); });
  await page.getByRole("button", { name: "确认发起审核" }).click();
  await expect.poll(() => reviewReached).toBe(true);
  const reviewSubmitUrl = new URL(page.url()).pathname;
  await page.goto(bReview); await waitReview(page);
  const reviewSwitchUrl = new URL(page.url()).pathname;
  await expect(page.getByRole("button", { name: "发起模拟审核" })).toBeVisible();
  releaseReview?.(); await page.waitForTimeout(250);
  expect(new URL(page.url()).pathname).toBe(bReview);
  await expect(page.getByRole("button", { name: "发起模拟审核" })).toBeVisible();
  expect((await getDetail(page, aItem)).content_item.status).toBe("reviewed");
  expect((await getDetail(page, bItem)).content_item.status).toBe("draft");
  await page.unroute(`**/api/v1/content-items/${aItem}/reviews/mock`);

  const bBeforeReview = await getDetail(page, bItem);
  const bReviewResponse = await (await page.request.post(`/api/v1/content-items/${bItem}/reviews/mock`, { data: { content_version_id: bBeforeReview.current_version.id, expected_version: bBeforeReview.current_version.version }, headers: { "Idempotency-Key": "i06-race-b-review" } })).json();
  const aReviews = await (await page.request.get(`/api/v1/content-items/${aItem}/reviews?limit=10&offset=0`)).json();
  const aReviewId = aReviews.data.items[0].id as string;
  const bReviewId = bReviewResponse.data.review.id as string;

  let releaseADetail: (() => void) | undefined, aDetailReached = false;
  const aDetailGate = new Promise<void>(resolve => { releaseADetail = resolve; });
  await page.route(`**/api/v1/reviews/${aReviewId}`, async route => { const response = await route.fetch(); aDetailReached = true; await aDetailGate; await route.fulfill({ response }); });
  await page.goto(aReview); await waitReview(page);
  await expect.poll(() => aDetailReached).toBe(true);
  const detailSubmitUrl = new URL(page.url()).pathname;
  await page.goto(bReview); await waitReview(page);
  await expect(page.getByText("审核时固定正文", { exact: true })).toBeVisible();
  const detailSwitchUrl = new URL(page.url()).pathname;
  await expect(page.getByRole("heading", { name: /I06 confirmed chapter B/ })).toBeVisible();
  releaseADetail?.(); await page.waitForTimeout(250);
  expect(new URL(page.url()).pathname).toBe(bReview);
  await expect(page.getByRole("heading", { name: /I06 confirmed chapter B/ })).toBeVisible();
  expect(page.url()).not.toContain(aReviewId);
  await page.unroute(`**/api/v1/reviews/${aReviewId}`);

  let releaseAbort: (() => void) | undefined, abortReached = false;
  const abortGate = new Promise<void>(resolve => { releaseAbort = resolve; });
  await page.route(`**/api/v1/content-items/${aItem}`, async route => { const response = await route.fetch(); abortReached = true; await abortGate; await route.fulfill({ response }); });
  await page.goto(aEditor);
  await expect.poll(() => abortReached).toBe(true);
  const abortSubmitUrl = new URL(page.url()).pathname;
  await page.goto(bEditor); await waitEditor(page);
  const abortSwitchUrl = new URL(page.url()).pathname;
  releaseAbort?.(); await page.waitForTimeout(250);
  expect(new URL(page.url()).pathname).toBe(bEditor);
  await expect(page.getByLabel("标题")).toHaveValue("I06 confirmed chapter B");
  await page.unroute(`**/api/v1/content-items/${aItem}`);

  const aPlan = await (await page.request.get(`/api/v1/chapter-plans/${fixtures.confirmed_chapter_plan_id}`)).json();
  const bPlan = await (await page.request.get(`/api/v1/chapter-plans/${projectB.confirmed_chapter_plan_b_id}`)).json();
  expect(aPlan.data.project_id).toBe(fixtures.project_a_id);
  expect(bPlan.data.project_id).toBe(projectB.project_b_id);
  expect((await getDetail(page, aItem)).content_item.chapter_plan_id).toBe(fixtures.confirmed_chapter_plan_id);
  expect((await getDetail(page, bItem)).content_item.chapter_plan_id).toBe(projectB.confirmed_chapter_plan_b_id);
  expect(events.requests.filter(value => value.includes(`/content-items/${bItem}/mock-generate`))).toHaveLength(0);
  expect(events.requests.filter(value => value.includes(`/content-items/${bItem}/reviews/mock`))).toHaveLength(0);
  expect(events.consoleErrors).toEqual([]);
  expect(events.pageErrors).toEqual([]);
  expect(events.failed).toEqual([]);
  expect(events.responses).toEqual([]);
  expect({ saveSubmitUrl, saveSwitchUrl, generateSubmitUrl, generateSwitchUrl, reviewSubmitUrl, reviewSwitchUrl, detailSubmitUrl, detailSwitchUrl, abortSubmitUrl, abortSwitchUrl }).toEqual({ saveSubmitUrl: aEditor, saveSwitchUrl: bEditor, generateSubmitUrl: aEditor, generateSwitchUrl: bEditor, reviewSubmitUrl: aReview, reviewSwitchUrl: bReview, detailSubmitUrl: aReview, detailSwitchUrl: bReview, abortSubmitUrl: aEditor, abortSwitchUrl: bEditor });
  expect(bReviewId).toBeTruthy();
});
