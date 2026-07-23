# Iteration 14 — WorkflowRun Runtime 与执行器抽象 — Closed Loop

项目入口创建 Runtime WorkflowRun 时，生产默认 `UnavailableWorkflowExecutor` 只持久化 `queued` Run 与脱敏初始 Event，不启动 Worker、不直接调用 n8n、不伪造终态。FakeWorkflowExecutor 仅用于测试。

流程中心使用 Runtime `/api/v1/workflow-runs`，详情使用 `/{runId}`；项目摘要使用 `/api/v1/projects/{projectId}/workflow-run-summary`。Runtime 读取 `workflow_run_records` 和 `workflow_run_events`，返回 camelCase DTO。

旧内容生产全局列表、内容改写详情继续由 contentitem、`workflow_runs` 和 snake_case DTO 提供，并冻结迁至 `/api/v1/content-workflow-runs` 与 `/{workflowRunId}`。global-lite、project-works 和对应测试的实际迁移由 CF-14-02D-R1 完成；本任务不改运行时代码或前端调用。

不按 ID、字段或查询参数在新旧 Handler 间分流，也不提供旧路径别名。旧数据不迁移；不要求历史 Migration 回滚。

后续：CF-14-02B-R1 修复 triggerSource 与持久化幂等；CF-14-02D-R1 完成旧路由迁移、主程序装配、Server 测试与真实 HTTP 冒烟；随后恢复 CF-14-03A、执行 CF-14-03B/C 和最终联调。真实 n8n/Worker/连接命令由 CF-14-N8N-Integration 独立处理。
