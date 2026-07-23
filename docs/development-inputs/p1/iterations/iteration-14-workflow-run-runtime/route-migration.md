# CF-14-01-R2 路由迁移冻结

| 所有权 | 路由 | 表与 DTO | 当前消费者 | 实施任务 |
|---|---|---|---|---|
| WorkflowRun Runtime | `/api/v1/workflow-runs`、`/{runId}`、`/{runId}/events`、`/{runId}/retries`、`/{runId}/cancel`、`/api/v1/projects/{projectId}/workflow-run-summary` | `workflow_run_records`、`workflow_run_events`、camelCase Runtime DTO | Runtime API、流程中心、项目摘要 | Runtime / CF-14-02D-R1 装配 |
| 旧内容生产 | `/api/v1/content-workflow-runs`、`/{workflowRunId}` | `workflow_runs`、snake_case 内容 DTO | global-lite、project-works、内容改写详情 | CF-14-02D-R1 |

禁止新旧域同时注册 `/api/v1/workflow-runs` 或 `/{id}`，禁止兼容别名，禁止按 ID、字段或参数分流。旧表数据不迁移到新表，旧 DTO 不变；不要求历史 Migration 回滚。

CF-14-02D-R1 必须迁移 `iteration08.go` 列表、`iteration07.go` 详情、global-lite、project-works 和相应测试，装配 Iteration 14 Handler，并完成完整 Server 测试和真实 HTTP 冒烟。本任务仅冻结迁移边界，不修改这些运行时代码。
