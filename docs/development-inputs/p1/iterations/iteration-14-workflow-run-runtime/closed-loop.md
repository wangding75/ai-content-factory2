# Iteration 14 — WorkflowRun Runtime 与执行器抽象 — Closed Loop

**状态：`completed`（冻结契约：`frozen_cf_14_01_r3`）。**

当前闭环为：创建内部 `WorkflowConnection` 配置记录 → 创建内部 `WorkflowConfiguration` 配置记录 → 将 Configuration 绑定到 Project stage → 创建 `queued` Runtime WorkflowRun → 查询、取消、重试、项目摘要。

绑定仅校验 Configuration 与其 Connection 存在。创建 Run 校验 Project、stage Binding、Configuration 与 Connection 存在，并遵守冻结的请求参数、版本及幂等要求；不要求任一记录的 `enabled=true` 或 `integrationStatus=verified`。

生产默认 `UnavailableWorkflowExecutor` 只持久化 queued Run、初始脱敏 Event 和脱敏快照；不启动 Worker、不直接调用 n8n、不调用外部执行器且不伪造终态。`FakeWorkflowExecutor` 仅用于测试。

流程中心使用 Runtime `/api/v1/workflow-runs`，详情使用 `/{runId}`，项目摘要使用 `/api/v1/projects/{projectId}/workflow-run-summary`，读取 `workflow_run_records` 与 `workflow_run_events` 并返回 camelCase DTO。旧内容生产继续使用 `/api/v1/content-workflow-runs`、`workflow_runs` 与 snake_case DTO；不按 ID、字段或参数分流，也不提供兼容别名或历史 Migration 回滚。

机器验收 PASS，人工 UI 验收 PASS（2026-07-23）。最终业务代码 Commit 为 `7b6d1e8fa64cb216e3b8645b6e596b503ce8379c`；验收证据保留在本地 `.ai-dev/reports/CF-14-03D/`，不进入 Git。

`CF-14-N8N-Integration` 为后续独立工作，不属于 Iteration 14 当前完成门槛，也不是遗留缺陷或阻塞项。权威路线图的下一迭代为 Iteration 15。
