# Iteration 06 数据模型

`ContentItem(id, project_id, chapter_plan_id UNIQUE, title, status, current_version_id, reviewed_at, created_at, updated_at)` 聚合正文；状态为 `draft|in_review|reviewed`。

`ContentVersion(id, content_item_id, version_no, version, status, source, title, content, summary, word_count, frozen_at, created_at, updated_at)` 是正文快照。本轮只存在 v1：`version_no=1`，初始 `source=manual_created`、`status=editable_draft`。`version` 是乐观锁号，更新后递增；`expected_version` 仅为请求条件。

`ReviewReport(id, content_item_id, content_version_id, provider_key=mock, status=completed, conclusion, score, summary, created_at)` 永久关联已冻结的固定版本。`ReviewFinding(id, review_id, category, severity, title, description, location)` 与 `ReviewRecommendation(id, review_id, priority, title, description, created_at)` 从属报告。

`WorkflowRun(id, provider_key=mock, workflow_key, subject_type, subject_id, status, idempotency_key, input_json, output_json, error_code, started_at, finished_at)` 记录生成或审核；状态 `running|succeeded|failed`。相同操作作用域、Idempotency-Key 和 payload 返回相同业务结果；键与 payload 不一致为冲突。

可空语义：所有 API 所述可空字段均明确区分 omitted（不更新/不适用）、null（清空）与 `""`（保留空字符串）。实体细节以全局目录和 Novel Schema 为准。
