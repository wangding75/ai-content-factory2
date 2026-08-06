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
  baseURL: process.env.E2E_BASE_URL || "http://127.0.0.1:13001",
  viewport: { width: 1440, height: 900 },
});

test("UI-002 LLM Provider Edit Drawer aligns with prototype and satisfies all acceptance criteria", async ({ page }) => {
  // 2. 打开 /settings
  await page.goto("/settings");
  await expect(page.locator('[aria-busy="true"]')).toHaveCount(0);

  // 3. 定位“OpenAI 主模型”所在行
  const row = page.locator("tr:has-text('OpenAI 主模型')");
  await expect(row).toBeVisible();

  // 4. 点击该行“编辑”
  const editBtn = row.locator("button:has-text('编辑')");
  await expect(editBtn).toBeVisible();
  await editBtn.click();

  // 5. 等待 dialog 可见
  const drawer = page.locator("aside.ui002-provider-drawer");
  await expect(drawer).toBeVisible();

  // 6. 验证标题为“编辑 LLM 配置”
  const title = drawer.locator("h2");
  await expect(title).toHaveText("编辑 LLM 配置");

  const subtitle = drawer.locator("header p");
  await expect(subtitle).toHaveText("关键连接参数修改后将失去执行资格，重新验证成功后恢复。");

  // 7. 验证配置名称为“OpenAI 主模型”
  const nameInput = drawer.locator("label:has-text('配置名称') input");
  await expect(nameInput).toHaveValue("OpenAI 主模型");

  // 8. 验证 Provider 类型只读
  const providerSelect = drawer.locator("label:has-text('服务类型') select");
  await expect(providerSelect).toBeDisabled();

  // 9. 验证 Base URL
  const baseUrlInput = drawer.locator("label:has-text('服务地址') input");
  await expect(baseUrlInput).toHaveValue("https://api.openai.com/v1");

  // 10. 验证请求超时为 60
  const timeoutInput = drawer.locator("label:has-text('请求超时') input");
  await expect(timeoutInput).toHaveValue("60");

  // 11. 验证“API Key：已安全保存”
  const credStatus = drawer.locator(".ui002-credential-status");
  await expect(credStatus).toBeVisible();
  await expect(credStatus.locator(".ui002-credential-text")).toHaveText("API Key：已安全保存");

  // 12. 验证“更新 API Key”按钮
  const updateBtn = credStatus.locator("button:has-text('更新 API Key')");
  await expect(updateBtn).toBeVisible();

  // 13. 点击更新按钮后验证密码输入框出现
  await updateBtn.click();
  const passwordInput = drawer.locator("label:has-text('API Key') input[type='password']");
  await expect(passwordInput).toBeVisible();
  await expect(passwordInput).toHaveAttribute("placeholder", "输入新的 API Key");
  await expect(passwordInput).toHaveAttribute("autoComplete", "new-password");

  // 14. 不输入密钥，关闭输入状态或重新打开抽屉
  await page.keyboard.press("Escape");
  await expect(drawer).not.toBeVisible();

  // Re-open
  await editBtn.click();
  await expect(drawer).toBeVisible();

  // 15. 验证默认模型为 gpt-5.2
  const defaultModelInput = drawer.locator("label:has-text('默认模型') input");
  await expect(defaultModelInput).toHaveValue("gpt-5.2");

  // 16. 验证发现模型按钮存在
  const discoverBtn = drawer.locator(".ui002-model-actions button:has-text('发现模型')");
  await expect(discoverBtn).toBeVisible();
  await expect(drawer.locator(".ui002-model-count")).toHaveText(/已发现 \d+ 个模型/);

  // 17. 验证验证状态为验证成功
  const valBadge = drawer.locator(".ui002-status-badge").nth(0);
  await expect(valBadge).toHaveText("验证成功");

  // 18. 验证启用状态为已启用
  const enabledBadge = drawer.locator(".ui002-status-badge").nth(1);
  await expect(enabledBadge).toHaveText("已启用");

  // 19. 验证执行资格为可执行
  const execBadge = drawer.locator(".ui002-status-badge").nth(2);
  await expect(execBadge).toHaveText("可执行");

  // 20. 验证取消、保存、保存并验证三个按钮
  const cancelBtn = drawer.getByRole("button", { name: "取消", exact: true });
  await expect(cancelBtn).toBeVisible();

  const saveBtn = drawer.getByRole("button", { name: "保存", exact: true });
  await expect(saveBtn).toBeVisible();

  const saveVerifyBtn = drawer.getByRole("button", { name: "保存并验证", exact: true });
  await expect(saveVerifyBtn).toBeVisible();

  // 21. 验证按 Escape 可以关闭
  await page.keyboard.press("Escape");
  await expect(drawer).not.toBeVisible();

  // 22. 重新打开编辑抽屉
  await editBtn.click();
  await expect(drawer).toBeVisible();

  // 23. 清除 Console 和 Network 异常后截图
  const outputDir = path.resolve(__dirname, "../../../docs/acceptance/second-loop-ui/page-fixes/UI-002");
  if (!fs.existsSync(outputDir)) {
    fs.mkdirSync(outputDir, { recursive: true });
  }

  const screenshotPath = path.join(outputDir, "UI-002_I19_02_LLM_PROVIDER_DRAWER_AFTER.png");
  await page.screenshot({ path: screenshotPath, fullPage: false });
  expect(fs.existsSync(screenshotPath)).toBe(true);
});
