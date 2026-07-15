# P0 状态机基线

## Project

`draft → planning → producing → archived`。P0 创建后为 `planning`。

## ChapterPlan

`pending_confirmation → confirmed`。只有 pending_confirmation 可编辑/删除/确认；只有 confirmed 可创建 ContentItem，确认不自动创建正文。

## ContentItem / ContentVersion

`ContentItem: draft → in_review → reviewed`

`ContentVersion: editable_draft → frozen`

confirmed ChapterPlan 首次创建 ContentItem 时原子创建空白 v1；每个 ChapterPlan 最多一个。v1 的 `version_no=1`，`version` 是乐观锁，`expected_version` 是客户端条件。保存草稿和 Mock Generate 只更新 editable v1，成功递增 version 且 item 仍为 draft。Mock Review 可内部进入 in_review；同步成功响应前原子冻结 v1、创建 completed ReviewReport、写 reviewed_at 并使 item=reviewed；失败回到 draft。已审核 v1 不可再审，新键返回 `content_version_already_reviewed`。v2/重写属于 Iteration 07。

## ReviewReport

`completed`。P0 只实现同步 Mock 审核结果，不实现人工审批或发布。

## WorkflowRun

`running → succeeded | failed`。P0 仅允许 `provider_key=mock`。

## Foreshadowing

`planned → planted → paid_off`。
