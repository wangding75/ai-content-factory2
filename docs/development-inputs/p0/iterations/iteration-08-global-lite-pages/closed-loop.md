# 闭环链路

## Frozen route and action contract

`/materials` (E1) lists existing global materials through `GET /api/v1/materials?scope=global`; list loading/error/empty/success are explicit. A selected material reads `GET /api/v1/materials/{materialId}` and may navigate only to an existing project material route when a reference provides `projectId` and `materialId`.

`/works` (E2) lists the read-only, cross-project `ProjectWorkReadModel` through `GET /api/v1/works`. `work_id` remains `ContentItem.id`; `current_version` is selected only by `current_version_id`. It may navigate only to `/projects/{projectId}/works`.

`/workflows` (E3) reads built-ins and run summaries. It never starts or edits a workflow. A result link is enabled only with the returned `project_id`, and targets `/projects/{projectId}/works`; run detail remains the existing `GET /api/v1/workflow-runs/{workflowRunId}` read route.

`/settings` (E4) reads capability and integration descriptors. It is read-only; its only cross-page action is `/workflows` for an advertised built-in workflow capability.

1. 全局导航 → E1_GLOBAL_MATERIALS，查看跨项目素材及引用
2. 全局导航 → E2_GLOBAL_WORKS，打开源项目、正文或审核
3. 全局导航 → E3_WORKFLOWS，查看内置模拟流程及产物
4. 全局导航 → E4_SETTINGS，查看模拟能力、真实 AI 与外部集成状态
