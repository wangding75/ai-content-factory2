# Iteration 14 — WorkflowRun Runtime 与执行器抽象 — 数据模型

**状态：`frozen_cf_14_01_r2`。**

Runtime 领域归 `apps/api/internal/workflowrun`：`workflow_run_records` 保存 camelCase API 所映射的 Run，`workflow_run_events` 保存只增的脱敏事件。关键字段为 `runNumber`、`stage`、`triggerSource`、配置快照、输入/输出、错误、时间戳和 `version`；合法 `triggerSource` 仅为 `manual`、`retry`、`system`、`api`。Create、Cancel、Retry 使用共享、持久化幂等机制，原始 Idempotency-Key 不写入 Run、Event、Snapshot 或日志。

旧内容生产领域归 `apps/api/internal/contentitem`：继续使用 `workflow_runs`、原有 snake_case DTO 和内容改写语义。旧数据不迁移到 Runtime 表，也不改变旧 DTO 字段。

Runtime 状态为 `queued`、`running`、`succeeded`、`failed`、`cancelled`；事件状态是写入后的 Run 状态快照。当前生产执行器是 UnavailableWorkflowExecutor：创建 queued 与初始 Event 后停止，不执行外部平台；FakeWorkflowExecutor 仅供测试。真实 n8n、Worker、队列、回调和连接/配置执行命令延后至 `CF-14-N8N-Integration`。
