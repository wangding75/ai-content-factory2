# Agent 自动化脚本

将压缩包解压到仓库根目录。

## 目标

这些脚本只沉淀重复执行动作，不替代业务任务中的特殊验收：

- 仓库基线预检
- Web 定向测试与固定门禁
- Docker Engine 稳定性判断与 production 验证
- Chromium 路由冒烟
- 安全提交、push 和机器可读报告

报告默认写入（已由 `.gitignore` 忽略）：

```text
.ai-dev/reports/
```

建议将执行报告作为本地或 CI artifact，不纳入 Git。

## 1. 任务开始

工作区必须 clean：

```powershell
.\scripts\agent\preflight.ps1 `
  -TaskId "09.F6C" `
  -ExpectedHead "<上一提交完整Hash>"
```

继续已有修改时，仅允许明确范围：

```powershell
.\scripts\agent\preflight.ps1 `
  -TaskId "09.F6C" `
  -ExpectedHead "<上一提交完整Hash>" `
  -AllowDirty `
  -AllowedDirtyPath "apps/web/src/features/example/*"
```

## 2. Web 门禁

```powershell
.\scripts\agent\verify-web.ps1 `
  -TaskId "09.F6C" `
  -TargetTest "src/features/example/example.test.ts" `
  -LintPath "src/features/example/page.tsx","src/features/example/example.test.ts"
```

脚本执行：

- 定向测试
- `apps/web` 全部测试
- typecheck
- 修改文件定向 lint
- production build
- `git diff --check`

失败后先修复根因，再重新运行对应局部步骤；不得无分析反复跑完整门禁。

## 3. Docker production

默认优先复用已经健康且可访问的现有服务，避免每次完整重建：

```powershell
.\scripts\agent\verify-production.ps1 -TaskId "09.F6C"
```

需要 API 健康检查时：

```powershell
.\scripts\agent\verify-production.ps1 `
  -TaskId "09.F6C" `
  -ApiHealthUrl "http://127.0.0.1:18080/<实际健康路径>"
```

仅在服务不可用时构建；构建最多尝试两次。第一次遇到 Docker 命名管道中断时，脚本会等待 Engine 连续稳定后再重试一次。

## 4. Chromium 冒烟

```powershell
.\scripts\agent\verify-routes.ps1 `
  -TaskId "09.F6C" `
  -Route "/workflows","/settings" `
  -ForbiddenPattern `
    "\bprovider\b", `
    "\b[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}\b", `
    "\d{4}-\d{2}-\d{2}T\d{2}:\d{2}"
```

自动检查：

- HTTP 可访问
- Console error
- page error
- 资源或接口失败
- 横向溢出
- 指定禁止文本

页面特有结构、Tab、按钮和状态仍由任务测试或特殊验收项定义。

脚本从 `apps/web` 工作区加载现有的 `@playwright/test`，不依赖根目录提升的依赖。

## 5. Commit 与 push

默认只暂存明确批准的路径：

```powershell
.\scripts\agent\finalize.ps1 `
  -TaskId "09.F6C" `
  -CommitMessage "fix: ..." `
  -IncludePath "apps/web/src/features/example" `
  -Push
```

确实需要提交全部已审查差异时才使用：

```powershell
.\scripts\agent\finalize.ps1 `
  -TaskId "09.F6C" `
  -CommitMessage "fix: ..." `
  -StageAll `
  -Push
```

脚本禁止隐式暂存：必须传 `-IncludePath` 或 `-StageAll`。
