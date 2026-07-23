# Iteration 14 — WorkflowRun Runtime 与执行器抽象 — 验收

**状态：`completed`（冻结契约：`frozen_cf_14_01_r3`）。** 不要求历史 Migration 回滚，只验证当前数据库与业务终态。

## 当前验收项

- Connection、WorkflowConfiguration 是内部配置记录；不代表真实 n8n 集成、外部连通性验证、可调用工作流、启用执行或完成鉴权。
- `enabled` 与 `integrationStatus` 是未来集成预留元数据，不是绑定或创建 Run 的门槛。
- 绑定要求 Configuration 及其 Connection 存在；创建 Run 要求 Project、stage Binding、Configuration 和 Connection 存在，并满足冻结的参数、版本、幂等要求。
- 闭环为创建 Connection、创建 WorkflowConfiguration、项目绑定、创建 queued WorkflowRun、查询、取消、重试和摘要。
- `UnavailableWorkflowExecutor` 只持久化 queued Run、初始 Event 与脱敏快照；不启动 Worker、不调用 n8n、不伪造运行或成功终态。Fake 仅用于测试。
- Runtime 维持 `/api/v1/workflow-runs` 及 runId、events、retries、cancel 和项目摘要路由，使用 camelCase DTO；旧内容路由维持 `/api/v1/content-workflow-runs` 与 snake_case DTO。
- `triggerSource` 仅为 `manual`、`retry`、`system`、`api`；ErrorEnvelope 使用 `request_id`。

## 延后项

真实 n8n Adapter、鉴权、连接/工作流验证、verify、enable、disable、Worker、队列、callback、外部执行与回写都由 `CF-14-N8N-Integration` 独立处理，不是当前完成门槛。

## 最终验收记录

- 机器验收：PASS。
- 人工 UI 验收：PASS（2026-07-23）。
- 最终业务代码 Commit：`7b6d1e8fa64cb216e3b8645b6e596b503ce8379c`。
- 验收证据：`.ai-dev/reports/CF-14-03D/`（本地忽略文件，不进入 Git）。

| 验收项 | 结果 |
| --- | --- |
| Contract | PASS |
| Data Model | PASS |
| Migration / 当前数据库终态 | PASS |
| Repository | PASS |
| Service | PASS |
| HTTP API | PASS |
| Persistent Idempotency | PASS |
| Route Migration | PASS |
| Frontend List | PASS |
| Frontend Detail / Event | PASS |
| Cancel / Retry | PASS |
| Project Summary / Run Entry | PASS |
| Real API / PostgreSQL | PASS |
| Browser QA | PASS |
| Security Review | PASS |
| Manual UI Acceptance | PASS |
