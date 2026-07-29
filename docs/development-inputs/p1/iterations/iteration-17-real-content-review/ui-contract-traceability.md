# Iteration 17 — UI / API / 数据模型追踪

| Frame | 用户操作/状态 | API | 关键字段 | 数据模型 | 主要错误 | 开发任务 |
|---|---|---|---|---|---|---|
| `I17_D1_EDITOR_REVIEW_ENTRY` | 保存正文、打开抽屉 | getContentItem, getContentReviewSummary | currentVersion, hasUnsavedChanges, canStartReview | ContentItem, ContentVersion | content_version_not_reviewable | CF-17-03A |
| `D2_SUBMIT_REVIEW_DRAWER` | 选固定版本、预检、创建 Run | preflightContentReviewRun, createContentReviewRun | sourceVersion, dimensions, preflightToken | ContentVersion, Binding, WorkflowRun | review_not_configured, active_review_conflict, preflight_token_* | CF-17-03A |
| `STATE_TASK_RUNNING_BAR` | queued/running | getContentReviewSummary, getWorkflowRunDetail, listWorkflowRunEvents | activeRun, latestEvents | WorkflowRun, WorkflowRunEvent | — | CF-17-03B |
| `D2_REVIEW_V2` | 查看报告、过滤 Issue、标记忽略 | getReviewDetail, updateReviewIssue | report, score, issues, disposition | ReviewReport, ReviewFinding/Issue | review_issue_version_conflict | CF-17-03B |
| `I17_D2_REVIEW_ISSUE_DETAIL` | 全文定位 | getReviewDetail, getContentVersion | sourceVersion, location, evidence | ContentVersion, ReviewIssue | review_source_version_missing | CF-17-03B |
| `STATE_TASK_FAILED_NOTICE` | Runtime Retry / 仅消费 Retry | getContentReviewSummary, retryWorkflowRun, retryReviewResultConsumption | state, latestError, canRetry* | Run, Event | run_not_retryable, review_result_not_retryable | CF-17-03B |
| `STATE_NOT_CONFIGURED_EMPTY` | 查看配置入口 | getContentReviewSummary, preflightContentReviewRun | configurationSummary | Binding, Configuration, Connection | review_not_configured | CF-17-03B |
| `I17_D2_REVIEW_HISTORY` | 查看历次 Run/Report | listContentItemReviews, getReviewDetail, getWorkflowRunDetail | runStatus, reportSummary, sourceVersion | Run, Report | — | CF-17-03B |

## 强制追踪规则

1. `workId=ContentItem.id`；审核来源使用明确 `ContentVersion.id`。
2. Stage 固定 `review`，禁止客户端覆盖。
3. `review_ready` 必须同时存在同一 Run 的 ReviewReport。
4. 失败 Frame 不得显示原始上游响应或内部实现。
5. 重写按钮在 Iteration 17 不对应任何可调用 API。
