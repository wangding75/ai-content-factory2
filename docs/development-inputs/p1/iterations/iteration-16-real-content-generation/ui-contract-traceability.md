# CF-16-01C — UI / API 契约追踪

**状态：`review_ready_cf_16_01c`。** 11 个 Frame 归并为一个编辑器页面、一个抽屉和持久化状态变体，不是 11 条路由。所有新 P1 响应使用 `data`/`request_id`；错误使用受限 `ErrorEnvelope`。

| Frame | 组件 / 路由 | 动作与 API | 状态、错误、后续任务 |
|---|---|---|---|
| I16_D1_EDITOR_CHAPTER_GOAL | 编辑器 | GET /api/v1/works/{workId}; GET /api/v1/content-items/{contentItemId}/content-generation-summary | idle/candidate_ready；03A |
| I16_D1_EDITOR_STORY_CONTEXT | 同一编辑器右侧 Tab | 复用 ProjectWork/ChapterPlan/故事线只读查询 | loading/empty/error/success；03A |
| I16_D1_EDITOR_MATERIALS | 同一编辑器右侧 Tab | 复用项目素材只读查询 | loading/empty/error/success；03A |
| I16_D2_GENERATE_CONFIRM | 生成抽屉 | POST .../content-generation-runs/preflight | passed/blocked；03A |
| I16_D2_GENERATE_REQUIREMENTS | 生成抽屉 | preflight → POST .../content-generation-runs | input/queued；03A |
| I16_D3_RUN_QUEUED | 顶部状态条 | Summary + GET WorkflowRun/detail/events | queued；03B |
| I16_D3_RUN_RUNNING | 顶部状态条 | Summary + WorkflowRun events | running；03B |
| I16_D3_RUN_SUCCEEDED | 顶部状态条 | Summary.latestCandidateVersion | candidate_ready；03B |
| I16_D4_CANDIDATE_VERSION | 版本选择状态 | GET versions/detail; POST .../current-version | success/current_version_changed/candidate_source_stale；03C |
| I16_D3_RUN_FAILED | 顶部状态条/正文提示 | Summary + Runtime detail/events; Runtime retry or consumption retry | runtime_failed/output_validation_failed/result_consumption_failed；03B |
| I16_D5_NOT_CONFIGURED | 编辑器状态 | Summary + preflight | not_configured/workflow_not_configured；03B |


## 路由和身份

- 编辑器：`/projects/{projectId}/works/{workId}`；
- `workId == ContentItem.id`；
- 版本历史仍由 `/api/v1/content-items/{contentItemId}/versions` 提供；
- Run 详情和 Retry/Cancel 复用 Iteration 14；
- 项目工作流配置复用 Iteration 13，业务页面不选择临时配置。

## 失败映射

- Runtime `failed/cancelled`：从 Run 状态恢复，使用通用 Retry；
- `output_validation_failed`：无候选版本，必须新建 Runtime Retry；
- `result_consumption_failed`：无候选版本，使用专用结果消费重试；
- `workflow_not_configured` / `execution_integration_unavailable`：阻止创建 Run并引导设置；
- `current_version_changed` / `candidate_source_stale`：不覆盖，刷新版本或重新生成。

## 中文和图标

前端不得把 `stage`、`status`、`source`、错误 code 或 Material Symbols ligature 直接显示。技术标识可以显示 Run ID、版本号和模型名；业务状态必须映射为中文。
