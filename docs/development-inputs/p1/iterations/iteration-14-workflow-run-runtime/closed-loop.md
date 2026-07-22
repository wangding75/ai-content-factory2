# Iteration 14 — n8n 适配与 WorkflowRun 异步运行 — Closed Loop

## 连接与工作流就绪

连接列表 → 验证 → Worker/服务端在受控边界使用 n8n Adapter 安全测试 → `verified` 或 `failed`（脱敏错误）→ 用户启用或停用。只有已验证连接可启用，启用新连接原子停用旧连接。

工作流配置列表 → 检查绑定连接已验证且已启用 → 验证 Workflow ID 或 Webhook Path → `verified` 或 `failed` → 用户启用或停用。配置或关联连接变化后回到待验证状态。

## 项目触发与运行

项目业务页点击“运行工作流”时：已绑定且可运行，打开 P14_08 确认弹窗；未绑定，打开 P14_09 提示弹窗，提示前往绑定且不创建独立页面。确认后服务端读取项目绑定、创建不含密钥的快照与 `WorkflowRun(queued)`，并以 Idempotency-Key 去重；创建成功后直接跳转 `/workflow-runs/{runId}`。

Worker 领取 queued 运行后进入 running，调用 n8n Webhook，校验输出，再进入 succeeded；超时、HTTP/Adapter 错误、输出校验失败进入 failed；可取消的运行进入 cancelled。错误、事件和上游响应均使用脱敏结构。页面刷新通过运行详情和事件恢复状态；不建设 callback server。

## 运行记录与项目入口

流程中心无标签页，直接在 `/workflow-runs` 展示全局运行记录；列表支持项目和环节筛选。详情统一在 `/workflow-runs/{runId}`，面包屑为“流程中心 / 运行记录 / 运行编号”，项目仅为字段和可点击链接。

项目概览展示运行摘要、最近运行和入口：“查看全部运行记录”跳转 `/workflow-runs?projectId={projectId}`，“查看环节运行记录”跳转 `/workflow-runs?projectId={projectId}&stage={stage}`。不建立第二套项目级运行列表路由。

## 重试与边界

失败详情的完整重试创建新的 WorkflowRun 并记录 `retryOfRunId`；原运行、快照和事件不可修改。Iteration 14 不包含分发平台绑定或执行，也不包含 LLM Provider 执行。
# CF-14-01-R1 clarification: CreateRun is idempotent and has no expectedVersion. Cancel/Retry expectedVersion targets runId WorkflowRun.version; Retry is failed/cancelled only, defaults to source snapshot/input, supports current configuration and complete inputOverride replacement. Shared Idempotency-Key replay returns the first response without duplicate writes. List time filters are inclusive createdAt bounds; runNumber is exact and q is runNumber contains. Summary is totalRuns, activeRuns (queued+running), recentFailedRuns (7x24h), lastRunAt, and <=3 recentRuns. Event status is a WorkflowRun status snapshot.
