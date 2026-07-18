import { expect, test } from "@playwright/test";

test("09.F6C loads real project types, filters projects, and persists a created project", async ({ page }) => {
  const name = `F6C-验收项目-${Date.now()}`;
  const listRequests: string[] = [];
  page.on("request", (request) => { if (request.url().includes("/api/v1/projects?")) listRequests.push(request.url()); });

  await page.goto("/projects");
  await expect(page.locator('[aria-busy="true"]')).toHaveCount(0);
  await expect(page.locator('a[href="/projects/new"]').first()).toBeVisible();
  await page.locator(".projects-filter-controls button").nth(1).click();
  await expect.poll(() => listRequests.some((url) => url.includes("status=planning"))).toBe(true);

  await page.goto("/projects/new");
  await expect(page.locator('input[name="type"]')).toHaveCount(5);
  const types = page.locator('input[name="type"]');
  await expect(types).toHaveCount(5);
  await expect(page.locator('input[name="type"][value="short_film"]')).not.toBeChecked();
  await page.locator('.create-type-card:has(input[value="short_film"])').click();
  await page.locator("#name").fill(name);
  await page.locator("#description").fill("F6C Chromium acceptance project.");
  const create = page.waitForResponse((response) => response.url().includes("/api/v1/projects") && response.request().method() === "POST");
  await page.locator('button[type="submit"]').click();
  expect((await create).status()).toBe(201);
  await expect(page).toHaveURL(/\/projects\/[0-9a-f-]{36}$/);
  await page.goto("/projects");
  await expect(page.getByRole("heading", { name })).toBeVisible();
  await page.reload();
  await expect(page.getByRole("heading", { name })).toBeVisible();
});

test("09.F6C displays retryable list and type loading failures in the DOM", async ({ page }) => {
  await page.route("**/api/v1/projects?*", async (route) => route.fulfill({ status: 500, contentType: "application/json", body: JSON.stringify({ error: { code: "internal_error", message: "hidden", details: {} }, request_id: "req_error" }) }));
  await page.goto("/projects");
  await expect(page.locator(".projects-error")).toBeVisible();
  await expect(page.locator(".projects-error button")).toBeVisible();
  await page.unroute("**/api/v1/projects?*");

  await page.route("**/api/v1/project-types", async (route) => route.fulfill({ status: 500, contentType: "application/json", body: JSON.stringify({ error: { code: "internal_error", message: "hidden", details: {} }, request_id: "req_error" }) }));
  await page.goto("/projects/new");
  await expect(page.locator(".create-type-state[role=alert]")).toBeVisible();
  await expect(page.locator(".create-type-state button")).toBeVisible();
});

test("09.F6D searches project names with the selected status as an AND filter", async ({ page }) => {
  const name = `F6D-搜索项目-${Date.now()}`;
  const requests: string[] = [];
  page.on("request", (request) => { if (request.url().includes("/api/v1/projects?")) requests.push(request.url()); });

  await page.goto("/projects/new");
  await page.locator("#name").fill(name);
  await page.locator('button[type="submit"]').click();
  await expect(page).toHaveURL(/\/projects\/[0-9a-f-]{36}$/);

  await page.goto("/projects");
  const search = page.getByRole("textbox", { name: "搜索项目名称" });
  await search.fill(name);
  await search.press("Enter");
  await page.locator(".projects-filter-controls button").nth(1).click();
  await expect.poll(() => requests.some((url) => url.includes("q=") && url.includes("status=planning"))).toBe(true);
  await expect(page.getByRole("heading", { name })).toBeVisible();

  await search.fill("不存在的项目名称");
  await search.press("Enter");
  await expect(page.getByRole("heading", { name: "暂无匹配项目" })).toBeVisible();
  await page.getByRole("button", { name: "清空项目搜索" }).click();
  await expect(page.getByRole("heading", { name })).toBeVisible();
});
