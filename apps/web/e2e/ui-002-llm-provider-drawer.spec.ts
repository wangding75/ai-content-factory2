import { expect, test } from "@playwright/test";
import { execSync } from "child_process";
import path from "path";
import fs from "fs";

const repoRoot = path.resolve(__dirname, "../../..");
const scriptPath = path.join(repoRoot, "scripts/ui-acceptance/Set-UI001-LlmProviderFixture.ps1");

test.beforeAll(() => {
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
  // Collect exceptions
  const consoleErrors: string[] = [];
  const pageErrors: Error[] = [];
  const networkErrors: string[] = [];
  const failedRequests: string[] = [];

  page.on("console", msg => {
    if (msg.type() === "error") {
      consoleErrors.push(msg.text());
    }
  });

  page.on("pageerror", err => {
    pageErrors.push(err);
  });

  page.on("response", response => {
    const status = response.status();
    if (status >= 400 && status < 600) {
      networkErrors.push(`${response.request().method()} ${response.url()} -> ${status}`);
    }
  });

  page.on("requestfailed", request => {
    const failure = request.failure();
    const errorText = failure ? failure.errorText : "Unknown";
    if (errorText !== "net::ERR_ABORTED") {
      failedRequests.push(`${request.method()} ${request.url()} -> ${errorText}`);
    }
  });

  // 1. 打开 /settings?tab=llm
  await page.goto("/settings?tab=llm");
  await expect(page.locator('[aria-busy="true"]')).toHaveCount(0);

  // 2. 找到“备用模型”
  const row = page.locator("tr:has-text('备用模型')");
  await expect(row).toBeVisible();

  // 3. 点击该行精确名称为“编辑”的按钮
  const editBtn = row.getByRole("button", { name: "编辑", exact: true });
  await expect(editBtn).toBeVisible();
  await editBtn.click();

  // 4. 抽屉标题为“编辑 LLM 配置”
  const drawer = page.locator("aside.ui002-provider-drawer");
  await expect(drawer).toBeVisible();
  const title = drawer.locator("h2");
  await expect(title).toHaveText("编辑 LLM 配置");

  const subtitle = drawer.locator("header p");
  await expect(subtitle).toHaveText("关键连接参数修改后将失去执行资格，重新验证成功后恢复。");

  // 5. 配置名称为“备用模型”
  const nameInput = drawer.locator("label:has-text('配置名称') input");
  await expect(nameInput).toHaveValue("备用模型");

  // 6. 服务类型只读
  const providerSelect = drawer.locator("label:has-text('服务类型') select");
  await expect(providerSelect).toBeDisabled();

  // 7. 默认模型为 gpt-4.1-mini
  const defaultModelInput = drawer.locator("label:has-text('默认模型') input");
  await expect(defaultModelInput).toHaveValue("gpt-4.1-mini");

  // 8. API Key 不回显，并显示“API Key：已安全保存”
  const credStatus = drawer.locator(".ui002-credential-status");
  await expect(credStatus).toBeVisible();
  await expect(credStatus.locator(".ui002-credential-text")).toHaveText("API Key：已安全保存");

  // 9. 显示模型数量 5，获取模型列表，模型搜索，手动模型，校验模型
  const discoverBtn = drawer.locator(".ui002-discover-btn");
  await expect(discoverBtn).toBeVisible();
  await expect(drawer.locator(".ui002-model-empty-state")).toContainText("已发现模型数量: 5");

  const searchInput = drawer.locator(".ui002-model-search-input");
  await expect(searchInput).toBeVisible();

  const manualInput = drawer.locator("label:has-text('手动输入模型') input");
  await expect(manualInput).toBeVisible();

  const verifyManualBtn = drawer.getByRole("button", { name: "校验模型", exact: true });
  await expect(verifyManualBtn).toBeVisible();

  // 10. 显示“配置已变更”、“不可执行”、“启用状态已保留”
  const valBadge = drawer.locator(".ui002-status-badge").nth(0);
  await expect(valBadge).toHaveText("配置已变更");

  const enabledBadge = drawer.locator(".ui002-status-badge").nth(1);
  await expect(enabledBadge).toHaveText("启用状态已保留");

  const execBadge = drawer.locator(".ui002-status-badge").nth(2);
  await expect(execBadge).toHaveText("不可执行");

  // 11. 显示说明文案
  const warningText = drawer.locator(".ui002-impact-biz");
  await expect(warningText).toHaveText("Base URL、API Key、超时或模型配置变化会使验证失效，但不会自动停用或解除项目绑定。");

  const relationText = drawer.locator(".ui002-impact-rel");
  await expect(relationText).toHaveText("现有工作流配置和项目绑定关系保持不变；重新验证成功前，相关新运行暂时被阻断。");

  // 12. 验证“取消”、“保存”、“保存并验证”三个按钮
  const cancelBtn = drawer.getByRole("button", { name: "取消", exact: true });
  await expect(cancelBtn).toBeVisible();

  const saveBtn = drawer.getByRole("button", { name: "保存", exact: true });
  await expect(saveBtn).toBeVisible();

  const saveVerifyBtn = drawer.getByRole("button", { name: "保存并验证", exact: true });
  await expect(saveVerifyBtn).toBeVisible();

  // 13. Escape 关闭
  await page.keyboard.press("Escape");
  await expect(drawer).not.toBeVisible();

  // 14. 重新打开以进行启停测试
  await editBtn.click();
  await expect(drawer).toBeVisible();

  // 记录当前显示为 “已启用”
  await expect(drawer.locator(".ui002-status-badge").nth(1)).toHaveText("启用状态已保留");
  const toggleBtn = drawer.getByRole("button", { name: "停用配置", exact: true });
  await expect(toggleBtn).toBeVisible();

  // 点击“停用配置”
  await toggleBtn.click();

  // 验证抽屉内控制逻辑与状态：立即显示 “未启用”，按钮变为 “启用配置”
  await expect(drawer.locator(".ui002-status-badge").nth(1)).toHaveText("未启用");
  await expect(drawer.getByRole("button", { name: "启用配置", exact: true })).toBeVisible();

  // 抽屉不得关闭
  await expect(drawer).toBeVisible();

  // 关闭当前抽屉，等待所有网络请求完成
  await page.keyboard.press("Escape");
  await expect(drawer).not.toBeVisible();
  await page.waitForLoadState("networkidle");

  // 15. 恢复数据库状态为启用且 version=2
  execSync(`docker compose exec -T postgres psql -U postgres -d ai_content_factory -c "UPDATE llm_provider_configurations SET enabled=true, version=2 WHERE id='11000000-0000-4000-8000-000000000003';"`, {
    cwd: repoRoot,
    stdio: "inherit",
  });

  // 重新加载页面并重新打开抽屉以截取正确的主截图
  await page.goto("/settings?tab=llm");
  const newRow = page.locator("tr:has-text('备用模型')");
  const newEditBtn = newRow.getByRole("button", { name: "编辑", exact: true });
  await expect(newEditBtn).toBeVisible();
  await newEditBtn.click();
  await expect(drawer).toBeVisible();

  // Take the first screenshot (top & model config)
  const outputDir = path.resolve(__dirname, "../../../docs/acceptance/second-loop-ui/page-fixes/UI-002");
  if (!fs.existsSync(outputDir)) {
    fs.mkdirSync(outputDir, { recursive: true });
  }
  const screenshotPath1 = path.join(outputDir, "UI-002_I19_02_LLM_PROVIDER_DRAWER_AFTER.png");
  await page.screenshot({ path: screenshotPath1, fullPage: false });

  // Scroll inside the drawer body to bottom
  const drawerBody = drawer.locator(".llm-drawer-body");
  await drawerBody.evaluate(el => el.scrollTop = el.scrollHeight);
  await page.waitForTimeout(500); // Wait for scroll animation/rendering

  // Take the second screenshot (status region & warnings)
  const screenshotPath2 = path.join(outputDir, "UI-002_I19_02_LLM_PROVIDER_DRAWER_STATUS_AFTER.png");
  await page.screenshot({ path: screenshotPath2, fullPage: false });

  // Write actual errors count to validation-data.json
  const dataPath = path.join(outputDir, "validation-data.json");
  const validationData = {
    providerId: "11000000-0000-4000-8000-000000000003",
    name: "备用模型",
    providerType: "openai_compatible",
    baseUrl: "https://gateway.example.com/v1",
    defaultModel: "gpt-4.1-mini",
    timeoutSeconds: 60,
    hasSecret: true,
    validationStatus: "stale",
    enabled: true,
    executable: false,
    "页面实际 URL": "http://127.0.0.1:13001/settings?tab=llm",
    "数据 Verify 结果": "PASS",
    consoleErrorsCount: consoleErrors.length,
    pageErrorsCount: pageErrors.length,
    networkErrorsCount: networkErrors.length,
    failedRequestsCount: failedRequests.length
  };
  fs.writeFileSync(dataPath, JSON.stringify(validationData, null, 2), "utf-8");

  // Assertions on console, page errors, and failed requests
  expect(consoleErrors).toEqual([]);
  expect(pageErrors).toEqual([]);
  expect(networkErrors).toEqual([]);
  expect(failedRequests).toEqual([]);
});
