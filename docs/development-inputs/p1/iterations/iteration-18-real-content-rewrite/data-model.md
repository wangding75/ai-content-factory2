# CF-18-01B — Iteration 18 真实正文重写数据模型

**状态：`frozen_cf_18_01b`。** 本文件在 CF-18-01A 已冻结业务/API 契约之下，冻结数据表复用、来源追溯、数据库约束与 Migration 18 终态。不得据此建立第二套正文、Candidate、Review、WorkflowRun、幂等或 Summary 系统。

## 1. Migration 17 实际基线

唯一开发数据库在执行前为 Migration 17。Iteration 18 直接复用：

| 领域事实 | 持久化依据 | Iteration 18 用法 |
|---|---|---|
| ContentItem / CAS | `content_items` | `current_version_id` 是唯一当前版本指针；`version` 是聚合乐观锁 |
| ContentVersion / Candidate | `content_versions` | Rewrite Candidate 仍是一条非当前 ContentVersion |
| WorkflowRun | `workflow_run_records` | `stage=rewrite`、`subject_type=review_report`、`subject_id=reviewId` |
| WorkflowRunEvent | `workflow_run_events` | Runtime 生命周期、输出校验、消费成功/失败事实 |
| Preflight Token 消费事实 | `idempotency_records` | 复用现有 nonce 消费标记；Token 原文不落库 |
| 命令幂等 | `idempotency_records` | Create、Runtime Retry、Consumption Retry、Set Current 的首次响应与回放 |
| ReviewReport | `review_reports` | 已完成真实审核报告，只读输入 |
| ReviewIssue | `review_findings` | 同 Report 的 `open` Issue，只读输入 |

现有 `content_versions` 已有 `source_content_version_id`、`source_content_version_version`、`source_workflow_run_id`，现有部分唯一索引 `content_versions_source_workflow_run_unique_idx` 已保证任意非空来源 Run 最多关联一个 ContentVersion。现有 `(content_item_id, source, version_no DESC, id DESC)` 与 WorkflowRun subject 查询索引满足 Result、Summary 与 History 的查询依据，不创建重复索引。

## 2. 唯一来源链

真实 Rewrite 的持久化关系固定为：

```text
review_reports.id
  ├─ content_version_id ───────────────→ 固定 source content_versions.id
  └─ id ───────────────────────────────→ workflow_run_records.subject_id
                                           stage = rewrite
                                           subject_type = review_report
                                           input_payload = rewrite.input.v1
                                           │
                                           └─ id ─────────────────────────→ content_versions.source_workflow_run_id
                                                                               source = workflow_rewrite
                                                                               source_content_version_id = Report source
                                                                               │
content_items.current_version_id ──────────────────────────────────────────────┘（仅 Set Current/CAS 后）
```

Candidate 到 ReviewReport 的关系通过既有 `source_workflow_run_id → WorkflowRun.subject_id` 唯一派生；Candidate 到来源版本通过既有 `source_content_version_id` 直接关联。不得增加 `rewrite_candidates`、`rewrite_source_issue_links`、重复 `review_report_id` 列或其他平行来源表。

## 3. Rewrite WorkflowRun

真实 Rewrite Run 必须同时满足：

- `stage='rewrite'`；
- `subject_type='review_report'`；
- `subject_id=review_reports.id`；
- Report 为 `provider_key='runtime'`、`status='completed'`、`schema_version='review.output.v1'`；
- Run、Report、ContentItem 与 source ContentVersion 属于同一 Project/ContentItem；
- `input_payload.schemaVersion='rewrite.input.v1'`；
- `input_payload.reviewReportId=subject_id`；
- `input_payload.sourceContentVersionId=review_reports.content_version_id`；
- `input_payload.selectedIssues` 为 1～50 个对象，`reviewIssueId` 唯一，每项属于同一 Report，且快照中的 `reviewReportId` 与 `disposition='open'` 正确；
- `configuration_snapshot.stage='rewrite'`，并冻结 `rewrite.input.v1` / `rewrite.output.v1`。

Migration 18 使用 CHECK 加延迟约束触发器落实上述可由数据库表达的形状、Report 归属、Issue 归属与输入快照约束。Issue 的实时 `version/disposition` 漂移仍必须由 Create 事务的锁定复核保证；数据库触发器验证的是写入 Run 的不可变安全快照和同 Report 归属，不把后续 Issue 处置反向写入历史 Run。

### Active Rewrite Run

同一冻结业务 subject 最多一个 active Run：

```sql
UNIQUE (project_id, stage, subject_type, subject_id)
WHERE stage='rewrite'
  AND status IN ('queued','running')
  AND subject_type='review_report'
  AND subject_id IS NOT NULL
```

该部分唯一索引不影响 generation、review、终态 Rewrite Run、不同 Report/来源版本或不同 Project。服务层先检查并返回 `active_rewrite_run_conflict`，数据库索引是并发最终保护。

## 4. selectedIssueIds 的唯一持久化位置

不新增 Issue 关联表。客户端的 `selectedIssueIds` 在 Preflight/Create 中按 `position ASC, reviewIssueId ASC` 规范化，并作为 `RewriteRuntimeInputV1.selectedIssues` 的完整安全快照持久化到 `workflow_run_records.input_payload`。每个元素已有：

- `reviewIssueId`、`reviewReportId`；
- `issueKey`、`position`、`version`、`disposition=open`；
- category/severity/title/description/evidence/location/suggestion。

因此：

- selected ID 集合由 `input_payload.selectedIssues[*].reviewIssueId` 唯一派生；
- Result Consumption Retry 重用同一 Run 的该输入快照；
- Runtime Retry 继承原 Run 的 Report/Issue 业务快照，只替换新 Run 自身的 `workflowRunId/correlationId`；
- Summary/Result 从 Run 输入快照恢复选中项，不读取后来变化的 disposition 冒充原始输入；
- addressed/unresolved/warnings/metadata 继续读取严格校验后的 `output_payload`，不重复持久化。

## 5. Rewrite Candidate ContentVersion

Migration 18 只扩展 `content_versions.source` 允许 `workflow_rewrite`，并增加其专用形状与延迟来源校验。

真实 Rewrite Candidate 必须满足：

- `source='workflow_rewrite'`；
- `status='editable_draft'`、`version=1`、`frozen_at IS NULL`；
- `source_content_version_id`、`source_content_version_version`、`source_workflow_run_id` 均非空；
- Candidate `id` 与来源 ContentVersion `id` 不同，且二者属于同一 ContentItem；
- `source_content_version_version` 是来源 ContentVersion 的乐观锁 `version`，不是 `version_no`；
- source WorkflowRun 为 `stage=rewrite/status=succeeded/subject_type=review_report`；
- Run 的 ReviewReport、source ContentVersion、Project、ContentItem 与 Candidate 一致；
- Candidate 创建时不更新 `content_items.current_version_id`。

同 Run 最多一个 Candidate 继续由既有 `content_versions_source_workflow_run_unique_idx` 保证，Result Consumption 与 Retry 查找已有 Candidate 的唯一依据固定为：

```sql
SELECT ...
FROM content_versions
WHERE source_workflow_run_id = :runId
  AND source = 'workflow_rewrite';
```

不得只依赖“先查后写”。`UNIQUE(content_item_id, version_no)` 继续保护同 Item 的版本序号；在锁定 ContentItem 后计算下一个 `version_no`。

## 6. Preflight Token 与幂等

Preflight Token 继续采用现有签名 Token + nonce 消费事实：

- Preflight 本身只读，不创建 Token 行、Run、Event、Candidate 或命令幂等记录；
- Token 原文永不写入 Run、Event、幂等响应、日志或错误；
- Token Claims 必须绑定 actor/作用域、Project、ContentItem、source ID/version/hash、ReviewReport、规范化 selected Issue 快照摘要、optionalInstructions、rewriteOptions、Binding/Configuration/Connection ID/version、input/output Contract、请求摘要、nonce、issuedAt/expiresAt；
- Create 以事务级 advisory lock 锁定 nonce，并使用 `idempotency_records` 保存单次消费标记；
- 命令幂等继续使用 `(scope,idempotency_key)` 唯一约束、规范化 `request_hash` 与首次 `response_status/response_body`。

不创建 Rewrite 专用 Token 表或幂等表。

## 7. Summary 与可查询状态

`ContentRewriteSummary` 是只读聚合，不新增表或状态列：

- active/latest Run：`workflow_run_records`；
- selected Issue 摘要：Run `input_payload.selectedIssues`；
- Candidate：`content_versions.source_workflow_run_id`；
- `candidate_ready`：同一 succeeded Rewrite Run 已有完整 Candidate 且存在 `result_consumed` Event；
- `output_validation_failed` / `result_consumption_failed`：`workflow_run_events`；
- `candidateIsCurrent`：`content_items.current_version_id = candidate.id`；
- `canSetCurrent`：Candidate 已消费、非当前且请求时 CAS 基线仍可比较。

Set Current 成功后不新增“已采用”字段：Summary 仍为 `candidate_ready`，`candidateIsCurrent=true`、`canSetCurrent=false`，ContentItem 为 `draft` 且 `reviewed_at=NULL`。

## 8. Migration 18 最小增量

Migration 18 只增加：

1. `workflow_rewrite` ContentVersion source 枚举值与专用 shape CHECK；
2. Rewrite Run subject shape CHECK；
3. active Rewrite Run 部分唯一索引；
4. Rewrite Run → Report/Issue/input/configuration 的延迟约束触发器；
5. Rewrite Candidate → Run/Report/source Version 的延迟约束触发器。

不新增列、业务表、状态表或虚假回填。Down 只撤销上述对象并恢复 Migration 17 的 source CHECK；不得删除既有字段、索引或历史数据。

## 9. 兼容性结论

- `workflow_generated` shape、同 Run 唯一索引与 Iteration 16 Set Current/CAS 字段不变；
- `review_reports/review_findings/review_recommendations`、Iteration 17 `open/ignored` 与八状态不变；
- `workflow_run_records` 五个 Runtime 状态和既有 Event 值不变；
- P0 `mock_rewrite`、历史 Mock/Generation ContentVersion 与 nullable 来源列保持原义；
- 历史记录不回填 Rewrite Run、ReviewReport、source ContentVersion 或 selected Issue；
- Migration 1～17 不修改。
