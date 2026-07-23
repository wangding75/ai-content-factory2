# CF-14-01-R3 路由迁移冻结

**状态：`frozen_cf_14_01_r3`。**

| 所有权 | 路由 | 表与 DTO | 当前消费者 |
|---|---|---|---|
| WorkflowRun Runtime | `/api/v1/workflow-runs`、`/{runId}`、`/{runId}/events`、`/{runId}/retries`、`/{runId}/cancel`、`/api/v1/projects/{projectId}/workflow-run-summary` | `workflow_run_records`、`workflow_run_events`、camelCase Runtime DTO | Runtime API、流程中心、项目摘要 |
| 旧内容生产 | `/api/v1/content-workflow-runs`、`/{workflowRunId}` | `workflow_runs`、snake_case 内容 DTO | global-lite、project-works、内容改写详情 |

禁止新旧域同时注册 `/api/v1/workflow-runs` 或 `/{id}`，禁止兼容别名与按 ID、字段、参数分流。旧表数据不迁移到新表，旧 DTO 不变；不要求历史 Migration 回滚。

当前不新增 verify、enable、disable 路由，不引入真实 n8n 调用。Connection/Configuration 的 `enabled` 与 `integrationStatus` 不作为绑定和创建 Run 的路由门槛。
