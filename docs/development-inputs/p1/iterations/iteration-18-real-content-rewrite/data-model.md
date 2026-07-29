# Iteration 18 — 真实正文重写数据模型

**状态：`rebuild_candidate_2026_07_29`。** 本文件描述 CF-18-01 的候选终态，不创建第二套正文、审核或 Runtime 模型。

## 1. 模型复用

| 领域名称 | 持久化 | Iteration 18 责任 |
|---|---|---|
| WorkflowRun | `workflow_run_records` | `stage=content_rewrite`、`subject_type=review_report`、输入/配置/输出快照 |
| WorkflowRunEvent | `workflow_run_events` | Runtime 生命周期、output_validation_failed、result_consumed、result_consumption_failed |
| ReviewReport | `review_reports` | 重写输入报告，只读 |
| ReviewIssue | `review_findings` | selected Issue，只读；处置必须为 open |
| ContentVersion | `content_versions` | 创建 `workflow_rewrite` 非当前候选并保存来源版本/Run |
| RewriteSourceIssueLink | 新增单一关联表 | 候选/Run/Report/Issue 的可追踪关系 |
| IdempotencyRecord | `idempotency_records` | 创建 Run、消费 Retry、设为当前命令回放 |

禁止新建 Work、RewriteCandidate、第二套 ReviewIssue、第二套 WorkflowRun 或独立消费状态表。

## 2. WorkflowRun subject 与 active 约束

真实重写 Run：

```text
stage = content_rewrite
subject_type = review_report
subject_id = ReviewReport.id
```

同一 Report 最多一个 queued/running Run；终态后允许再次重写。不同 Report 可以并行。CF-18-01 应通过部分唯一索引提供最终并发保证，并增加按 subject 查询的倒序索引。

## 3. ContentVersion 候选

复用 Iteration 16 已存在的来源字段：

- source_content_version_id；
- source_content_version_version；
- source_workflow_run_id。

真实重写候选要求：

- `source=workflow_rewrite`；
- `status=editable_draft`；
- source 三项均非空；
- sourceContentVersion 与候选属于同一 ContentItem；
- sourceWorkflowRun 为 `content_rewrite` 且 subject 指向输入 Report；
- 一个 Run 最多一个候选；
- 候选创建时不得更新 ContentItem.current_version_id。

历史 `mock_rewrite`、`workflow_generated` 和其他来源保持兼容。

## 4. RewriteSourceIssueLink

建议表名：`rewrite_source_issue_links`。最小字段：

| 字段 | 规则 |
|---|---|
| id | UUID 主键 |
| project_id | 与 Report/ContentItem/Run 同项目 |
| content_item_id | 来源和候选共同 ContentItem |
| workflow_run_id | 对应 content_rewrite Run |
| candidate_content_version_id | 成功创建的候选 |
| review_report_id | 输入 Report |
| review_issue_id | 选中的 review_findings.id |
| issue_key | 输入时的稳定 issueKey 快照 |
| resolution_status | addressed/partially_addressed/not_addressed |
| resolution_summary | 安全的处理说明 |
| created_at | 创建时间 |

约束：

- `(workflow_run_id, review_issue_id)` 唯一；
- `(candidate_content_version_id, review_issue_id)` 唯一；
- Issue 必须属于 Report；Report 必须绑定 source ContentVersion；
- Link 数量必须等于冻结输入的 selected Issue 数量；
- Link 与候选同事务创建，失败时全部回滚。

## 5. Summary

ContentRewriteSummary 是只读聚合，不新增表。读取：

- Report、来源版本和选中 Issue 摘要；
- 最新 active content_rewrite Run；
- 最新任意 Run 与 Events；
- 同 Run 候选 ContentVersion；
- RewriteSourceIssueLink；
- Binding/Configuration/Connection 可用性。

`candidate_ready` 必须存在绑定同一 Run 的候选和完整 IssueLink；succeeded 但无候选不得返回 ready。

## 6. Migration 原则

CF-18-01 必须先审计当前数据库终态，再冻结一个最小向前 Migration。允许的候选变更仅限：

- `workflow_rewrite` ContentVersion source；
- content_rewrite active subject 索引；
- RewriteSourceIssueLink 表与必要约束/索引；
- 若现有 Event CHECK 尚未允许所需 Event，则做最小兼容扩展。

不得修改历史 Migration、伪造历史数据、清空数据库或创建平行模型。
