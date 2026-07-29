# Iteration 17 — UI / API / 数据模型追踪

**状态：`frozen_cf_17_01`。**

| Frame | 用户操作/状态 | OpenAPI operationId | 最终 Schema / 状态 | 数据模型 | 开发任务 |
|---|---|---|---|---|---|
| `I17_D1_EDITOR_REVIEW_ENTRY` | 保存正文、打开抽屉 | `getContentItem`, `getContentReviewSummary` | `ContentReviewSummary`：`idle/not_configured` | ContentItem, ContentVersion | CF-17-03 |
| `D2_SUBMIT_REVIEW_DRAWER` | 选固定版本、预检、创建 Run | `preflightContentReviewRun`, `createContentReviewRun` | `ContentReviewPreflightReport`, `CreateContentReviewRunRequest`, `ReviewRuntimeInputV1` | ContentVersion, ProjectWorkflowBinding, WorkflowRun | CF-17-03 |
| `STATE_TASK_RUNNING_BAR` | queued/running | `getContentReviewSummary`, `getWorkflowRunDetail`, `listWorkflowRunEvents` | `ContentReviewSummary`：`queued/running`, `Iteration14WorkflowRun`, `WorkflowRunEvent` | workflow_run_records, workflow_run_events | CF-17-03 |
| `D2_REVIEW_V2` | 查看报告、过滤 Issue、标记忽略 | `getReview`, `updateReviewIssue` | `RealReviewDetail`, `RealReviewReport`, `RealReviewIssue`, `ReviewIssueDisposition` | review_reports, review_findings | CF-17-03 |
| `I17_D2_REVIEW_ISSUE_DETAIL` | 全文定位 | `getReview`, `getContentVersion`, `updateReviewIssue` | `ContentReviewSourceVersionSummary`, `ReviewRuntimeEvidenceV1`, `ReviewRuntimeLocationV1` | content_versions, review_findings | CF-17-03 |
| `STATE_TASK_FAILED_NOTICE` | Runtime Retry / 仅消费 Retry | `getContentReviewSummary`, `retryWorkflowRun`, `retryReviewResultConsumption` | `runtime_failed/output_validation_failed/result_consumption_failed`, `ContentReviewSafeError` | workflow_run_records, workflow_run_events | CF-17-03 |
| `STATE_NOT_CONFIGURED_EMPTY` | 查看配置入口 | `getContentReviewSummary`, `preflightContentReviewRun` | `not_configured`, `ContentReviewConfigurationSummary` | ProjectWorkflowBinding, WorkflowConfiguration, WorkflowConnection | CF-17-03 |
| `I17_D2_REVIEW_HISTORY` | 查看历次 Run/Report | `listContentItemReviews`, `getReview`, `getWorkflowRunDetail` | `ContentReviewHistoryItem`, `ReviewListEnvelope` | workflow_run_records, review_reports | CF-17-03 |

## 强制追踪规则

1. `workId=ContentItem.id`；审核来源固定 `subjectType=content_version`、`subjectId=sourceContentVersionId`。
2. Runtime Stage 固定 `review`，客户端不能覆盖。
3. Runtime 使用 `ReviewRuntimeInputV1`（`review.input.v1`）与 `ReviewRuntimeOutputV1`（`review.output.v1`）。
4. `review_ready` 必须存在绑定同一 WorkflowRun 的 ReviewReport；succeeded 且无 Report 时不得返回。
5. 失败 Frame 只显示 `ContentReviewSafeError`，不得显示原始上游响应或内部实现。
6. 重写按钮在 Iteration 17 不对应任何 OpenAPI operation。
