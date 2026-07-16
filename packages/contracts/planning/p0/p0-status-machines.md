# P0 状态机基线

## Project

```text
draft → planning → producing → archived
```

P0 默认创建后为 `planning`，不实现删除和归档入口。

## ChapterPlan

```text
pending_confirmation → confirmed
```

规则：

- 模拟生成只创建 `pending_confirmation`。
- 只有 `pending_confirmation` 可编辑、删除、确认。
- `confirmed` 才能创建 ContentItem。
- 确认不自动生成正文。

## ContentItem / ContentVersion

```text
ContentItem: draft → in_review → reviewed
ContentVersion: draft → saved
```

P0 版本规则：

- 首次生成或创建正文形成 v1。
- 审核只创建 ReviewReport，不修改版本正文。
- 重写形成 v2，必须保留 v1。
- v2 不自动成为 current_version。
- v2 不自动提交审核、不自动发布。

## Review

```text
not_submitted → completed
```

P0 仅实现模拟审核完成结果，不实现多人协作、审批与发布。

## Iteration 07.1A Mock Rewrite

`frozen v1 + completed ReviewReport -> editable_draft v2` is the only rewrite transition. v2 has `version_no=2` and `source=mock_rewrite`; v1 remains frozen, `ContentItem.current_version_id` does not change, and v2 has no ReviewReport or publication record. No v3, repeated rewrite, automatic review, or automatic publication transition exists.

## WorkflowRun

```text
queued → running → succeeded
                 └→ failed
```

P0 只允许 `provider_key=mock`。

For `content_mock_rewrite`, a new valid attempt starts `running`; it ends `succeeded` with target v2 or `failed` with safe error data and no target v2. Same-key same-payload retry returns the original run result; a new key is a new run.

## Foreshadowing

```text
planned → planted → paid_off
```

P0 可先实现 planned/planted，paid_off 作为可更新状态保留。
