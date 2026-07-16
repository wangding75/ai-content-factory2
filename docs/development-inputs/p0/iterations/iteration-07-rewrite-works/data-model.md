# 数据模型范围

- `RewriteRequest`
- `WorkflowRun`
- `WorkflowRunStatus`
- `ContentVersion`
- `ProjectWorkReadModel`

字段与关系以 `../../baselines/p0-data-model-catalog.md` 为全局基线。

## 07.1A 增量

`MockRewriteRequest(source_content_version_id, review_report_id, expected_version, parameters)` 的来源版本固定为 frozen v1；报告必须为该版本的 completed `ReviewReport`。`ContentVersion` 新增合法来源枚举 `mock_rewrite`，只用于 v2；v2 为 `editable_draft`，并以无 ReviewReport / 无发布记录表达未提交审核和未发布。

`WorkflowRunDetail(id, project_id, provider_key=mock, workflow_key=content_mock_rewrite, status, source_content_item_id, source_content_version_id, target_content_version_id?, source_review_report_id, input_summary, output_summary?, error?, idempotency_key, request_fingerprint, started_at, finished_at)`。成功 run 指向 v2；失败 run 的 target/output 为空且错误为安全消息。所有非空关系必须属于同一项目与同一 ContentItem。

## 07.1B 查询模型增量

`ContentVersionListItem` 包含版本自身摘要、`is_current` 与 nullable 来源 ContentVersion/ReviewReport/WorkflowRun 摘要；`ContentVersionList(items,total,limit,offset)` 固定按 `version_no DESC, id DESC`。`ContentVersionDetail` 包含指定版本完整快照、精简 ContentItem、相同的 nullable 来源摘要和 `is_current`。v1 的来源摘要均为 null；07.1A v2 的来源摘要分别指向 v1、completed ReviewReport 和 succeeded `content_mock_rewrite` WorkflowRun。

## 07.1C ProjectWorkReadModel

`ProjectWorkReadModel(work_id=ContentItem.id, project_id, chapter_plan, content_item, current_version, version_count, latest_review?, latest_workflow_run?, navigation)` 只读聚合既有数据。`work_id` 是稳定 ContentItem ID 映射；不新增持久化 Work。当前版本只比较 `current_version_id`；最近审核/运行分别按 `created_at DESC,id DESC` 和 `started_at DESC,id DESC`；版本历史复用 07.1B 摘要与排序。

## Migration 000007 结论

需要 Migration 000007；本轮只冻结结论，不创建 Migration。000006 的 `content_versions.source` 检查约束尚未允许 `mock_rewrite`，须扩展该枚举，并新增约束使其仅可用于 `version_no=2` 的 `editable_draft` v2；既有 `(content_item_id, version_no)` 唯一约束继续防止重复 v2，无需新增版本表或 `ProjectWorkReadModel` 表。

`workflow_runs` 须允许 `content_mock_rewrite`，以现有 `content_version_id` 继续表示来源版本，并新增 nullable `target_content_version_id`（同一 ContentItem 的复合外键）及 `source_review_report_id`（同项目、同 ContentItem、同来源版本的 ReviewReport 关系）。成功 run 必须指向 v2；失败 run 的 target/output 为空且只保留安全错误。既有 `(project_id, content_item_id, workflow_key, idempotency_key)` 唯一约束保持不变，用于同键重放；结合 `(content_item_id, version_no)` 唯一约束阻止重复创建 v2。新增 rewrite 查询索引应覆盖 `(content_item_id, workflow_key, started_at DESC, id DESC)`；既有版本和审核索引保持不变。跨项目关系继续由既有 `project_id/content_item_id` 复合外键及新增同范围复合外键保证。
