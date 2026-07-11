import { expect, test, type Page } from "@playwright/test";

const missingProjectId = "00000000-0000-4000-8000-000000000000";

async function forceInputValue(page: Page, selector: string, value: string) {
  await page.locator(selector).evaluate((element, nextValue) => {
    const prototype =
      element instanceof HTMLTextAreaElement
        ? HTMLTextAreaElement.prototype
        : HTMLInputElement.prototype;
    const setter = Object.getOwnPropertyDescriptor(prototype, "value")?.set;
    setter?.call(element, nextValue);
    element.dispatchEvent(new Event("input", { bubbles: true }));
  }, value);
}

async function expectNoHorizontalOverflow(page: Page) {
  await expect
    .poll(() =>
      page.evaluate(
        () => document.documentElement.scrollWidth <= window.innerWidth,
      ),
    )
    .toBe(true);
}

function appAlert(page: Page) {
  return page.locator('[role="alert"]:not(#__next-route-announcer__)');
}

test("Iteration 02 project creation persists across the complete desktop and mobile flow", async ({
  page,
}) => {
  const projectName = `Iteration-02-QA-${Date.now()}`;
  const description =
    "Persistent Novel project created by the Iteration 02 browser acceptance test.";
  const consoleErrors: string[] = [];
  const pageErrors: string[] = [];
  const failedRequests: string[] = [];
  const unexpectedApiResponses: string[] = [];
  const expectedApiErrors = new Set<string>();
  let createRequestCount = 0;

  page.on("console", (message) => {
    if (message.type() === "error") {
      consoleErrors.push(message.text());
    }
  });

  page.on("pageerror", (error) => {
    pageErrors.push(error.message);
  });

  page.on("requestfailed", (request) => {
    const url = request.url();
    const errorText = request.failure()?.errorText ?? "failed";
    const isExpectedNextRscAbort =
      request.method() === "GET" &&
      url.includes("_rsc=") &&
      errorText === "net::ERR_ABORTED";

    if (isExpectedNextRscAbort) {
      return;
    }

    failedRequests.push(
      `${request.method()} ${url}: ${errorText}`,
    );
  });

  page.on("request", (request) => {
    if (
      request.method() === "POST" &&
      request.url().includes("/api/v1/projects")
    ) {
      createRequestCount += 1;
    }
  });

  page.on("response", (response) => {
    const url = response.url();
    if (
      url.includes("/api/v1/") &&
      response.status() >= 400 &&
      !expectedApiErrors.has(url)
    ) {
      unexpectedApiResponses.push(`${response.status()} ${url}`);
    }
  });

  await page.goto("/");
  await expect(
    page.getByRole("heading", { name: "No projects yet" }),
  ).toBeVisible();
  await expect(
    page.getByRole("link", { name: "View all projects" }),
  ).toBeVisible();

  await page.getByRole("link", { name: "View all projects" }).click();
  await expect(page).toHaveURL(/\/projects$/);
  await expect(
    page.getByRole("heading", { name: "No matching projects" }),
  ).toBeVisible();

  await page.goto("/");
  await page.locator('a[href="/projects/new"]').first().click();
  await expect(page).toHaveURL(/\/projects\/new$/);

  await page.getByRole("link", { name: "Back to projects" }).click();
  await page.locator('a[href="/projects/new"]').last().click();
  await expect(page).toHaveURL(/\/projects\/new$/);

  const submit = page.getByRole("button", { name: "Create project" });

  await submit.click();
  await expect(appAlert(page)).toHaveText("Project name is required.");
  expect(createRequestCount).toBe(0);

  await forceInputValue(page, "#name", "x".repeat(121));
  await submit.click();
  await expect(appAlert(page)).toHaveText(
    "Project name must be 120 characters or fewer.",
  );
  expect(createRequestCount).toBe(0);

  await page.locator("#name").fill(projectName);
  await forceInputValue(page, "#description", "x".repeat(5001));
  await submit.click();
  await expect(appAlert(page)).toHaveText(
    "Description must be 5,000 characters or fewer.",
  );
  expect(createRequestCount).toBe(0);

  await page.locator("#description").fill(description);

  const createResponse = page.waitForResponse(
    (response) =>
      response.url().includes("/api/v1/projects") &&
      response.request().method() === "POST",
  );

  await submit.dblclick();
  await expect(
    page.getByRole("button", { name: /Creating project/ }),
  ).toBeDisabled();

  expect((await createResponse).status()).toBe(201);
  await expect(page).toHaveURL(/\/projects\/[0-9a-f-]{36}$/);
  expect(createRequestCount).toBe(1);

  const overviewUrl = page.url();

  await expect(
    page.getByRole("heading", { name: projectName }),
  ).toBeVisible();
  await expect(page.getByText(description)).toBeVisible();

  const typeAndStatus = page.locator("header > p").first();
  await expect(typeAndStatus).toContainText("novel");
  await expect(typeAndStatus).toContainText("planning");

  await expect(
    page.getByText("project setup", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByText(
      "This workspace has no materials, storylines, chapters, or content yet.",
    ),
  ).toBeVisible();

  for (const label of ["Created", "Last updated"]) {
    const value = page
      .locator("dt", { hasText: label })
      .locator("xpath=following-sibling::dd");

    await expect(value).not.toHaveText("");
    expect(Number.isNaN(Date.parse(await value.innerText()))).toBe(false);
  }

  await Promise.all([
    page.waitForURL(/\/projects$/),
    page.getByRole("link", { name: "Back to projects" }).click(),
  ]);
  await expect(page).toHaveURL(/\/projects$/);
  await expect(
    page.getByRole("heading", { name: projectName }),
  ).toBeVisible();

  await page.reload();
  await expect(
    page.getByRole("heading", { name: projectName }),
  ).toBeVisible();

  const createdProjectCard = page
    .locator("article")
    .filter({ hasText: projectName });

  await Promise.all([
    page.waitForURL(overviewUrl),
    createdProjectCard.getByRole("link", { name: "Open project" }).click(),
  ]);

  await expect(page).toHaveURL(overviewUrl);
  await expect(
    page.getByRole("heading", { name: projectName, level: 1 }),
  ).toBeVisible();

  await page.reload();
  await expect(page).toHaveURL(overviewUrl);
  await expect(
    page.getByRole("heading", { name: projectName, level: 1 }),
  ).toBeVisible();

  await page.goto("/projects?status=planning");
  await expect(
    page.getByRole("heading", { name: projectName }),
  ).toBeVisible();

  await page.selectOption("#status", "producing");
  await page.getByRole("button", { name: "Filter" }).click();

  await expect(
    page.getByRole("heading", { name: "No matching projects" }),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: projectName }),
  ).toHaveCount(0);

  await page.goto("/");
  await expect(
    page.getByRole("heading", { name: projectName }),
  ).toBeVisible();

  const invalidUrl = new URL(
    "/api/v1/projects/not-a-uuid/workspace",
    page.url(),
  ).toString();
  expectedApiErrors.add(invalidUrl);

  await page.goto("/projects/not-a-uuid");
  await expect(
    page.getByRole("heading", { name: "Invalid project address" }),
  ).toBeVisible();
  await expect(appAlert(page)).toBeVisible();

  const missingApiUrl = new URL(
    `/api/v1/projects/${missingProjectId}/workspace`,
    page.url(),
  ).toString();
  expectedApiErrors.add(missingApiUrl);

  await page.goto(`/projects/${missingProjectId}`);
  await expect(
    page.getByRole("heading", { name: "Project not found" }),
  ).toBeVisible();
  await expect(appAlert(page)).toBeVisible();

  await page.setViewportSize({ width: 375, height: 812 });

  for (const routePath of [
    "/",
    "/projects",
    "/projects/new",
    new URL(overviewUrl).pathname,
  ]) {
    await page.goto(routePath);
    await expect(page.locator("main")).toBeVisible();
    await expectNoHorizontalOverflow(page);
  }

  await expect(
    page.getByRole("link", { name: "Back to projects" }),
  ).toBeVisible();
  await page.getByRole("link", { name: "Back to projects" }).click();
  await expect(
    page.getByRole("link", { name: "New project" }),
  ).toBeVisible();

  expect(consoleErrors).toEqual([]);
  expect(pageErrors).toEqual([]);
  expect(failedRequests).toEqual([]);
  expect(unexpectedApiResponses).toEqual([]);
});
