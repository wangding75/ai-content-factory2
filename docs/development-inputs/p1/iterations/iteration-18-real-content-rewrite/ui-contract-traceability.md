# Iteration 18 — UI / API / 数据模型追踪

**状态：`rebuild_candidate_2026_07_29`。** OperationId 和 Schema 名称是 CF-18-01 的候选输入，正式值由 OpenAPI 冻结。

| Frame | 用户操作/状态 | API operationId | Summary 状态 | 数据模型 |
|---|---|---|---|---|
| `I18_D2_REVIEW_REWRITE_ENTRY` | 选择可处理 Issue 并点击创建重写任务 | `getReview`, `preflightContentRewriteRun` | `idle`, `not_configured` | ReviewReport, ReviewIssue, ContentVersion |
| `I18_D4_CREATE_REWRITE` | 进入创建页并完成预检 | `preflightContentRewriteRun`, `createContentRewriteRun` | `idle`, `queued` | ReviewReport, ReviewIssue, WorkflowRun |
| `I18_D4_REWRITE_CONFIG_DRAWER` | 点击查看项目配置 | `preflightContentRewriteRun` | `idle`, `not_configured` | ProjectWorkflowBinding, WorkflowConfiguration, WorkflowConnection |
| `I18_D4_REWRITE_RUNNING` | summary=queued|running | `getContentRewriteSummary`, `getWorkflowRunDetail`, `listWorkflowRunEvents` | `queued`, `running` | workflow_run_records, workflow_run_events |
| `I18_D5_REWRITE_RESULT` | summary=candidate_ready | `getContentRewriteSummary`, `getContentVersion`, `listContentItemVersions` | `candidate_ready` | content_versions, rewrite_source_issue_links |
| `I18_D5_SET_CURRENT_CONFIRM` | 点击设为当前版本 | `setCurrentContentVersion` | `candidate_ready` | content_items, content_versions, idempotency_records |
| `I18_D5_RESULT_CONSUMPTION_FAILED` | summary=result_consumption_failed | `getContentRewriteSummary`, `retryRewriteResultConsumption` | `result_consumption_failed` | workflow_run_records, workflow_run_events |
| `I18_D4_REWRITE_FAILED` | summary=runtime_failed|output_validation_failed | `getContentRewriteSummary`, `retryWorkflowRun`, `getWorkflowRunDetail` | `runtime_failed`, `output_validation_failed` | workflow_run_records, workflow_run_events |
| `I18_D4_REWRITE_AVAILABILITY` | summary=not_configured 或无记录 | `getContentRewriteSummary`, `preflightContentRewriteRun` | `idle`, `not_configured` | ProjectWorkflowBinding, WorkflowConfiguration, WorkflowConnection |

## 强制追踪规则

1. `workId=ContentItem.id`；Run 固定 `stage=content_rewrite`、`subjectType=review_report`、`subjectId=reviewReportId`。
2. Report、Issue、来源 ContentVersion 必须属于同一 ContentItem/Project。
3. selected Issue 只允许 `open`，创建后 Issue disposition 不自动变化。
4. `candidate_ready` 必须存在同一 Run 的候选与完整 RewriteSourceIssueLink。
5. `result_consumption_failed` 只能调用专用消费 Retry，不得外呼 Runtime。
6. 设为当前复用 Iteration 16 CAS；来源 stale 时不强制覆盖。
7. 失败 UI 只显示安全错误，不展示原始上游或内部实现。
