# CF-16-01B — UI / API / 数据模型追踪

**状态：`frozen_cf_16_01b`。** 11 个 Frame 均是唯一编辑器路由 `/projects/{projectId}/works/{workId}`（`workId=ContentItem.id`）的状态；不存在截图专用、候选、失败或未配置伪路由。

| Frame | 用户操作或状态 | API | 字段 | 数据模型 | 错误码 | 开发任务 |
|---|---|---|---|---|---|---|
| I16_D1_EDITOR_CHAPTER_GOAL | 编辑器加载/章节目标 | getContentItem, getContentGenerationSummary | contentItem, currentVersion, state | ContentItem, ContentVersion | — | CF-16-03A |
| I16_D1_EDITOR_STORY_CONTEXT | 切换故事情报 | getContentItem, getContentGenerationSummary | contentItem, currentVersion | ContentItem, ContentVersion | — | CF-16-03A |
| I16_D1_EDITOR_MATERIALS | 切换素材库 | getContentItem, getContentGenerationSummary | contentItem, currentVersion | ContentItem, ContentVersion | — | CF-16-03A |
| I16_D2_GENERATE_CONFIRM | 预检通过或阻断 | preflightContentGenerationRun | status, preflightToken, checks | ContentItem, ChapterPlan, ProjectWorkflowBinding | workflow_not_configured, chapter_plan_not_confirmed, active_run_conflict | CF-16-03A |
| I16_D2_GENERATE_REQUIREMENTS | 二次确认创建 Run | createContentGenerationRun | preflightToken, subjectType, subjectId | WorkflowRun, idempotency_records | preflight_token_invalid, preflight_token_expired, current_version_changed, idempotency_key_reused_with_different_payload | CF-16-03A |
| I16_D3_RUN_QUEUED | `queued` | getContentGenerationSummary, getWorkflowRunDetail, listWorkflowRunEvents | activeRun, latestEvents | WorkflowRun, WorkflowRunEvent | — | CF-16-03B |
| I16_D3_RUN_RUNNING | `running` | getContentGenerationSummary, getWorkflowRunDetail, listWorkflowRunEvents | activeRun, latestEvents | WorkflowRun, WorkflowRunEvent | — | CF-16-03B |
| I16_D3_RUN_SUCCEEDED | `candidate_ready` | getContentGenerationSummary | latestCandidateVersion, candidateCanBecomeCurrent | ContentVersion, WorkflowRunEvent | — | CF-16-03B |
| I16_D3_RUN_FAILED | runtime/output/消费失败与 Retry | getContentGenerationSummary, retryWorkflowRun, retryContentGenerationResultConsumption | latestRun, latestEvents, latestError | WorkflowRun, WorkflowRunEvent, ContentVersion | output_validation_failed, result_consumption_failed, run_already_consumed | CF-16-03B |
| I16_D4_CANDIDATE_VERSION | 查看/比较/设为当前 | listContentVersions, getContentVersion, setCurrentContentVersion | source_content_version_id, source_content_version_version, source_workflow_run_id, expectedCurrentVersion | ContentItem, ContentVersion | candidate_source_stale, content_version_not_candidate, content_version_item_mismatch | CF-16-03B |
| I16_D5_NOT_CONFIGURED | `not_configured` 与配置入口 | getContentGenerationSummary, preflightContentGenerationRun | workflowConfigured, canGenerate, latestError | ProjectWorkflowBinding, WorkflowConfiguration | workflow_not_configured, execution_integration_unavailable | CF-16-03B |

Summary 状态只允许 `idle`、`not_configured`、`queued`、`running`、`candidate_ready`、`runtime_failed`、`output_validation_failed`、`result_consumption_failed`。`WorkflowRun.succeeded` 仅是外部执行成功；`candidate_ready` 才表示输出消费与候选创建成功。`result_consumption_failed` 保留 succeeded Run 的事实，专用 Retry 不调用外部工作流。

所有用户可见文案使用简体中文：`running` 显示“运行中”，`candidate_ready` 显示“候选版本已创建”，`workflow_not_configured` 显示“尚未配置正文生成工作流”，`candidate_source_stale` 显示候选基线过期说明。不得显示错误码、snake_case、原始枚举或 Material Symbols 名称；图标只用按钮必须有中文 `aria-label`，最终 UI 不依赖外部图标字体。
