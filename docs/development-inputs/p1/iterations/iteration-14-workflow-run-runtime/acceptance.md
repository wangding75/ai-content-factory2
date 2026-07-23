# Iteration 14 — WorkflowRun Runtime 与执行器抽象 — 验收

**状态：`frozen_cf_14_01_r2`。** 当前验收不要求历史 Migration 回滚。

## 当前 Iteration 14 验收项

- [ ] Runtime 数据模型、WorkflowRunEvent、Repository 与 Service 正确。
- [ ] WorkflowExecutor 抽象、FakeWorkflowExecutor 与 UnavailableWorkflowExecutor 正确；生产创建仅写 queued 与初始事件。
- [ ] Runtime HTTP、持久化幂等、路由迁移、流程中心 UI、项目摘要、真实 API 与当前数据库终态正确。
- [ ] Runtime 保有 `/api/v1/workflow-runs` 及其 runId、events、retries、cancel 和项目摘要路由，使用 camelCase DTO。
- [ ] 旧内容生产 Run 仅使用 `/api/v1/content-workflow-runs` 及 `/{workflowRunId}`，保持 snake_case DTO 与 `workflow_runs` 表；不得迁移旧数据。
- [ ] `triggerSource` 仅为 `manual`、`retry`、`system`、`api`；Create、Cancel、Retry 使用持久化共享幂等。

## 延后项（不是当前完成门槛）

- [ ] 真实 n8n Adapter、Worker、队列、callback server 和真实外部工作流执行。
- [ ] n8n 成功、失败、超时、取消联调。
- [ ] WorkflowConnection 与 WorkflowConfiguration 的 verify、enable、disable。

延后项在 `CF-14-N8N-Integration` 独立开发；不得声明当前 Iteration 已完成真实工作流执行。
