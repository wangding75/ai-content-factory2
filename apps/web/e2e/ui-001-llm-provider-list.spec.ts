import { expect, test } from "@playwright/test";
import { execSync } from "child_process";
import path from "path";
import fs from "fs";

test.beforeAll(() => {
  const repoRoot = path.resolve(__dirname, "../../..");
  const scriptPath = path.join(repoRoot, "scripts/ui-acceptance/Set-UI001-LlmProviderFixture.ps1");
  execSync(`powershell.exe -NoProfile -ExecutionPolicy Bypass -File "${scriptPath}" -Mode Apply`, {
    cwd: repoRoot,
    stdio: "inherit",
  });
});

test.use({
  baseURL: process.env.E2E_BASE_URL || "http://localhost:13001",
  viewport: { width: 1440, height: 900 },
});

test("UI-001 LLM Provider List aligns with prototype and satisfies all acceptance criteria", async ({ page }) => {
  await page.goto("/settings");
  await expect(page.locator('[aria-busy="true"]')).toHaveCount(0);

  // 1. Verify Notice Banner
  const notice = page.locator(".ui001-provider-notice");
  await expect(notice).toBeVisible();
  await expect(notice).toContainText("关键连接参数修改后");
  await expect(notice).toContainText("执行资格立即失效");

  // 2. Verify Toolbar elements
  const searchInput = page.locator(".ui001-provider-list input[placeholder='搜索配置名称']");
  await expect(searchInput).toBeVisible();

  const statusSelect = page.locator(".ui001-provider-list select").nth(0);
  await expect(statusSelect).toBeVisible();
  const statusOptions = await statusSelect.locator("option").allInnerTexts();
  expect(statusOptions).toContain("全部验证状态");
  expect(statusOptions).toContain("配置已变更");

  const enabledSelect = page.locator(".ui001-provider-list select").nth(1);
  await expect(enabledSelect).toBeVisible();

  const executableSelect = page.locator(".ui001-provider-list select").nth(2);
  await expect(executableSelect).toBeVisible();
  const executableOptions = await executableSelect.locator("option").allInnerTexts();
  expect(executableOptions).toContain("全部执行资格");
  expect(executableOptions).toContain("可执行");
  expect(executableOptions).toContain("不可执行");

  const refreshBtn = page.getByRole("button", { name: "刷新" });
  await expect(refreshBtn).toBeVisible();

  // 3. Verify 10 Table Headers
  const tableHeaders = page.locator(".ui001-provider-list table th");
  await expect(tableHeaders).toHaveCount(10);
  const headerTexts = await tableHeaders.allInnerTexts();
  expect(headerTexts).toEqual([
    "配置名称",
    "服务类型",
    "Base URL",
    "可用模型",
    "默认模型",
    "验证状态",
    "启用状态",
    "执行资格",
    "最近验证",
    "操作",
  ]);

  // 4. Verify Seeded Rows and Data
  const r1 = page.locator("tr:has-text('OpenAI 主模型')");
  await expect(r1).toBeVisible();
  await expect(r1.locator(".ui001-status-verified")).toBeVisible();
  await expect(r1.locator(".ui001-status-verified")).toHaveText("验证成功");
  await expect(r1.locator(".ui001-eligibility-ready")).toHaveText("可执行");
  await expect(r1.locator("td").nth(3)).toContainText("0 个");

  const r2 = page.locator("tr:has-text('内容审核模型')");
  await expect(r2).toBeVisible();
  await expect(r2.locator(".ui001-status-failed")).toBeVisible();
  await expect(r2.locator(".ui001-status-failed")).toHaveText("验证失败");
  await expect(r2.locator(".ui001-eligibility-blocked")).toHaveText("不可执行");
  await expect(r2.locator(".ui001-status-error")).toContainText("认证失败，请更新 API Key");
  await expect(r2.locator("td").nth(3)).toContainText("0 个");

  const r3 = page.locator("tr:has-text('备用模型')");
  await expect(r3).toBeVisible();
  await expect(r3.locator(".ui001-status-stale")).toBeVisible();
  await expect(r3.locator(".ui001-status-stale")).toHaveText("配置已变更");
  await expect(r3.locator(".ui001-eligibility-blocked")).toHaveText("不可执行");
  await expect(r3.locator("td").nth(3)).toContainText("0 个");

  const r4 = page.locator("tr:has-text('本地兼容模型')");
  await expect(r4).toBeVisible();
  await expect(r4.locator(".ui001-status-unverified")).toBeVisible();
  await expect(r4.locator(".ui001-status-unverified")).toHaveText("未验证");
  await expect(r4.locator(".ui001-eligibility-blocked")).toHaveText("不可执行");
  await expect(r4.locator("td").nth(3)).toContainText("0 个");

  // 5. Interaction: Search filter
  await searchInput.fill("主模型");
  await expect(page.locator(".ui001-provider-list table tbody tr:has-text('主模型')")).toHaveCount(1);
  await expect(r1).toBeVisible();
  await searchInput.clear();

  // 6. Interaction: Status filter
  await statusSelect.selectOption("failed");
  await expect(page.locator(".ui001-provider-list table tbody tr:has-text('内容审核模型')")).toHaveCount(1);
  await expect(r2).toBeVisible();
  await statusSelect.selectOption("");

  // 7. Interaction: Executable filter
  await executableSelect.selectOption("true");
  await expect(page.locator(".ui001-provider-list table tbody tr:has-text('OpenAI 主模型')")).toHaveCount(1);
  await expect(r1).toBeVisible();
  await executableSelect.selectOption("");

  // 8. Interaction: Row action "更多" menu drawer & Escape key
  const moreBtn = r1.locator(".ui001-more-btn");
  await moreBtn.click();
  const drawer = page.locator(".ui001-more-menu");
  await expect(drawer).toBeVisible();
  await expect(drawer.getByRole("menuitem", { name: "发现模型" })).toBeVisible();

  await page.keyboard.press("Escape");
  await expect(drawer).not.toBeVisible();

  // 9. Refresh Button Reloads Data
  await refreshBtn.click();
  await expect(r1).toBeVisible();

  // 10. Capture Screenshot for Acceptance Report
  const outputDir = path.resolve(__dirname, "../../../docs/acceptance/second-loop-ui/page-fixes/UI-001");
  if (!fs.existsSync(outputDir)) {
    fs.mkdirSync(outputDir, { recursive: true });
  }

  const screenshotPath = path.join(outputDir, "UI-001_I19_01_LLM_PROVIDER_LIST_AFTER.png");
  await page.screenshot({ path: screenshotPath, fullPage: true });
  expect(fs.existsSync(screenshotPath)).toBe(true);
});
