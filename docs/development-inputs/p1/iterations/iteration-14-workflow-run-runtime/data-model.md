# Iteration 14 — WorkflowRun Runtime 与执行器抽象 — 数据模型

**状态：`frozen_cf_14_01_r3`。**

Runtime 领域归 `apps/api/internal/workflowrun`：`workflow_run_records` 保存 camelCase API 映射的 Run，`workflow_run_events` 保存仅追加的脱敏 Event。合法 `triggerSource` 为 `manual`、`retry`、`system`、`api`。Runtime 状态为 `queued`、`running`、`succeeded`、`failed`、`cancelled`。

`WorkflowConnection` 与 `WorkflowConfiguration` 是平台内部配置记录，为绑定和 Run 脱敏快照提供配置数据。`enabled`、`integrationStatus` 是未来真实集成预留元数据，当前既不表示已经验证/可执行，也不作为绑定或创建 queued Run 的资格门槛。

当前生产执行器 `UnavailableWorkflowExecutor` 在创建 queued Run 和初始 Event 后停止；不会执行外部平台。`FakeWorkflowExecutor` 仅供测试。真实 n8n、Worker、队列、回调与 verify/enable/disable 命令延后至 `CF-14-N8N-Integration`。

旧内容生产领域继续归 `apps/api/internal/contentitem`，使用 `workflow_runs`、既有 snake_case DTO 和内容改写语义；旧数据不迁移到 Runtime 表。
