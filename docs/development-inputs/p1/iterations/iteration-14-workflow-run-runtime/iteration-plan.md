# Iteration 14 — WorkflowRun Runtime 与执行器抽象

## 当前状态

`frozen_cf_14_01_r2`。本迭代当前范围只交付通用 WorkflowRun Runtime、平台无关的执行器抽象及其 HTTP/UI 闭环；不因延后的真实 n8n 集成而阻塞。

## 当前目标

- WorkflowRun、WorkflowRunEvent、Repository、Application Service 与安全快照；
- `WorkflowExecutor`、FakeWorkflowExecutor 与生产默认的 UnavailableWorkflowExecutor；
- Runtime HTTP API、流程中心 UI、项目摘要、当前数据库终态和真实 API 联调；
- 运行创建只写入 queued 记录和初始事件，不启动 Worker、不直接调用外部平台，也不伪造 succeeded。

## 路由与领域边界

| 领域 | 表/DTO | 路由所有权 | 消费者 |
|---|---|---|---|
| Runtime | `workflow_run_records`、`workflow_run_events`、camelCase Runtime DTO | `/api/v1/workflow-runs`、`/{runId}`、事件、重试、取消及项目摘要 | Runtime API、流程中心、项目摘要 |
| 内容生产旧闭环 | `workflow_runs`、既有 snake_case DTO | `/api/v1/content-workflow-runs`、`/{workflowRunId}` | global-lite、project-works、内容改写详情 |

旧数据不迁移到 Runtime 新表；不提供旧 `/api/v1/workflow-runs` 的兼容别名或按字段/ID 分流。历史 Migration 不要求回滚。

## 延后范围

真实 n8n Adapter、Worker、队列、callback server、外部工作流执行以及 WorkflowConnection/WorkflowConfiguration 的 verify、enable、disable 归入独立任务 `CF-14-N8N-Integration`，不是当前完成门槛。

## 后续顺序

1. CF-14-02B-R1：修复 `triggerSource` 与持久化幂等。
2. CF-14-02D-R1：迁移旧路由、主程序装配、完整 Server 测试和真实 HTTP 冒烟。
3. 恢复 CF-14-03A；随后 CF-14-03B/C；最后联调。
4. `CF-14-N8N-Integration` 独立开发。
