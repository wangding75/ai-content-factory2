# 验收方案

| 用例 ID | 场景 | 通过标准 |
|---|---|---|
| I07-AC01 | 创建 v2 保留 v1 | 重写成功后 versions 数量增加 1，v1 内容和记录仍存在。 |
| I07-AC02 | 不自动切换当前版本 | 生成 v2 后 current_version_id 仍保持原值，除非用户后续明确选择。 |
| I07-AC03 | 不自动审核与发布 | v2 初始 review_status=not_submitted，publish_status=not_published。 |
| I07-AC04 | 取消无副作用 | D4 取消时不创建 WorkflowRun 或 ContentVersion。 |
| I07-AC05 | 项目作品聚合 | D3 可显示章节、当前版本、审核状态和可用版本数量。 |
| I07-AC06 | 运行可追踪 | WorkflowRun 输入、输出摘要、开始/结束时间和错误信息可查询。 |

## 门禁

- 核心用例必须可重复执行。
- 不接受仅页面打开或 HTTP 200 的冒烟结果。
- 失败分支必须验证数据库无脏数据。

## 07.1A 契约验收

`mockRewriteContentItem` 仅接受 frozen v1、其 completed ReviewReport 与匹配的 `expected_version`；201 同时返回 v2、来源 v1、来源报告和 succeeded WorkflowRun。v2 的 `version_no=2`、`source=mock_rewrite`、`status=editable_draft`；v1 仍 frozen，`current_version_id` 不变，且 v2 没有自动审核或发布记录。

验证 `invalid_uuid`、not found、`content_version_not_frozen`、`review_not_completed`、`source_version_mismatch`、`version_conflict`、必填 key、同键异 payload、重复重写、参数无效、跨项目关系、`mock_rewrite_failed` 和 `internal_error` 的统一 ErrorEnvelope 与 HTTP 状态。失败不得留下部分 v2 或关联；failed WorkflowRun 可被 `getWorkflowRun` 查询，新键重试创建新 run。D4 Cancel/Close 无请求、无副作用。

## 07.1B 查询验收

版本列表返回 v2、v1，按 `version_no DESC, id DESC` 稳定排序并返回共享分页字段。只有 `current_version_id` 相等的项为当前版本；v2 仍非当前。详情请求 v1 返回 v1、请求 v2 返回 v2 的完整固定快照，均带所需来源摘要；无来源字段明确为 null。空版本集合成功返回空列表；无效 UUID、缺失对象、无效分页、跨项目访问和内部失败均使用既有安全 ErrorEnvelope。

## 07.1C 聚合查询验收

项目作品列表按 `chapter_plan.chapter_no ASC, content_item.id ASC`，空项目返回成功空列表。`work_id` 等于 ContentItem ID，详情不产生副作用且返回同一只读聚合；当前版本、版本计数、07.1B 历史摘要、最近审核/运行和 D1/D2/D4/D5 导航 ID 一致。无效 UUID、项目/作品缺失、无效分页、跨项目访问和内部失败使用既有安全 ErrorEnvelope。
