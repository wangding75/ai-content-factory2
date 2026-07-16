# 闭环链路

1. D2_REVIEW 或 D3_PROJECT_WORKS → D4_CREATE_REWRITE
2. 确认创建 → 内置 Mock Provider 生成 v2 → D5_REWRITE_RESULT
3. v1 保留，v2 不自动设为当前、不自动提交审核、不自动发布
4. D5 可返回 D1、D2 或 D3 查看对应数据

## 07.1A D4/D5 契约闭环

D4 Confirm 仅发送冻结的 Mock Rewrite POST；Cancel、Close 和 ESC 均不发送请求。请求固定关联 frozen v1、其 completed ReviewReport 与 D4 参数。成功后 D5 使用响应中的 target v2、source v1、source review 和 WorkflowRun 展示关系；v1 保留 frozen，v2 不成为 current，不自动审核或发布。

WorkflowRun 在新键有效尝试开始时创建，`running -> succeeded|failed`。成功关联 v2；失败保留 failed run、目标版本为空，不保留部分 v2。GET WorkflowRun 按项目隔离，且只返回安全错误信息。

## 07.1B D3/D5 版本导航

D3 使用版本列表显示 v2、v1 的固定顺序 `version_no DESC, id DESC`，并仅根据 `current_version_id` 标记当前版本；v2 不因较新而成为当前版本。D5 的“查看”以指定 versionId 读取完整固定快照，包含来源 v1、审核报告及重写 WorkflowRun 摘要；历史版本不得被当前版本替换。

## 07.1C D3 作品聚合导航

D3 列表读取 ProjectWorkReadModel，`work_id=ContentItem.id`。每项提供当前正文、最近审核、最近运行、版本计数及 D1/D2/D4/D5 所需 ID；当前版本仍只由 `current_version_id` 判定。打开正文使用 content_item_id/current_version_id，审核使用 latest_review_report_id，重写使用 rewrite_source_version_id/rewrite_review_report_id，D5 使用 rewrite_target_version_id/latest_workflow_run_id；空 ID 表示该入口当前不可用。
