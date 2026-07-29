# Iteration 17 — 真实内容审核数据模型

**状态：`rebuild_candidate_2026_07_29`。** 本迭代复用现有审核与 Runtime 模型，只做向前兼容扩展，不创建第二套 Report、Issue 或 Run。

## 1. 模型复用

| 领域名称 | 现有持久化 | Iteration 17 责任 |
|---|---|---|
| `WorkflowRun` | `workflow_run_records` | `stage=review`、`subject_type=content_version`、固定输入/配置快照 |
| `WorkflowRunEvent` | `workflow_run_events` | queued/running/terminal、`output_validation_failed`、`result_consumed`、`result_consumption_failed` |
| `ReviewReport` | `review_reports` | 关联来源 ContentVersion 与成功 WorkflowRun |
| `ReviewIssue` | 复用并扩展 `review_findings` | 结构化问题、证据、定位、建议和处置状态 |
| `ReviewRecommendation` | `review_recommendations` | 保留 P0 兼容；仅保存报告级建议，不重复 Issue 建议 |
| `idempotency_records` | 既有表 | 创建 Run、消费 Retry、Issue 处置命令回放 |

禁止新增 `review_issues`、第二套 `review_reports` 或第二套 Runtime 表。

## 2. WorkflowRun subject 与 active 约束

真实审核 Run：

```text
stage = review
subject_type = content_version
subject_id = source ContentVersion.id
```

服务层必须验证 ContentVersion → ContentItem → Project 归属。新增部分唯一索引，保证同一固定版本最多一个 active review Run：

```sql
UNIQUE (project_id, stage, subject_type, subject_id)
WHERE stage='review'
  AND status IN ('queued','running')
  AND subject_type='content_version'
  AND subject_id IS NOT NULL
```

不同 ContentVersion 可以并行；同一版本的历史终态 Run 不受限制。

## 3. ReviewReport 扩展

保留 P0 字段：`id/content_item_id/content_version_id/provider_key/status/conclusion/score_json/summary/created_at`。

建议向前增加：

| 字段 | 规则 |
|---|---|
| `workflow_run_id UUID NULL` | 真实审核必填；P0 历史 Mock 可为 null；FK 到 WorkflowRun |
| `schema_version VARCHAR(...) NULL` | 真实审核固定 `review.output.v1` |
| `source_content_version_version INTEGER NULL` | 创建 Run 时来源版本乐观锁快照 |
| `source_content_hash VARCHAR(...) NULL` | 安全摘要，不保存密钥或原始 Token |
| `completed_at TIMESTAMPTZ NULL` | 真实报告完成时间 |

约束：

- `workflow_run_id` 非空时唯一，确保一个 Run 最多一个 Report；
- 真实 Report 的 `content_version_id` 必须等于 Run subject；
- `status=completed` 的真实 Report 必须具有 schema/version/hash；
- 失败 Run 不创建 ReviewReport。

API 字段 `sourceContentVersionId` 映射现有 `content_version_id`，不得再增加重复来源列。

## 4. ReviewIssue / review_findings 扩展

P0 `ReviewFinding` 在 API/领域层统一升级为 `ReviewIssue`，表名保持 `review_findings` 以避免破坏历史数据。

新增或明确字段：

| 字段 | 规则 |
|---|---|
| `issue_key` | Report 内稳定唯一 |
| `position` | 显示顺序，从 1 开始 |
| `category_key/category_label` | 配置键与中文展示名 |
| `severity` | `critical|warning|suggestion` |
| `title/description` | 必填，受长度限制 |
| `evidence_json` | 必要短引文和来源引用 |
| `location_json` | 段落/句子范围，可空但结构合法 |
| `suggestion` | 问题级修改建议 |
| `disposition` | `open|ignored`，默认 `open` |
| `version` | Issue 处置乐观锁 |
| `ignored_at/ignored_by` | disposition=ignored 时记录 |
| `updated_at` | 处置更新时间 |

索引与约束：

- `UNIQUE(review_id, issue_key)`；
- `UNIQUE(review_id, position)`；
- 查询索引 `(review_id, severity, disposition, position)`；
- ignored 字段与 disposition 保持一致；
- Issue 不允许跨 Report、ContentItem 或 Project 引用。

## 5. Report 统计

`score_json` 存储契约化统计：

```json
{
  "criticalCount": 0,
  "warningCount": 0,
  "suggestionCount": 0,
  "passedRuleCount": 0
}
```

原始统计在 Report 创建后不可变。`openCount/ignoredCount` 由 Issue disposition 查询计算，不改写 Provider 统计。

## 6. Summary 与历史读取

`ContentReviewSummary` 是只读聚合，不新增表。它读取：

- 当前/选中的 ContentVersion；
- 同 subject 最新 active/terminal review Run；
- 同 Run Events；
- 同 Run ReviewReport；
- Report Issue 摘要；
- 当前 review Binding 可用性；
- 安全 `latestError`。

审核历史固定按 `WorkflowRun.created_at DESC, id DESC` 排序；成功行关联 Report，失败/运行中行的 report 为 null。

## 7. Migration 与兼容

只新增一个向前 Migration，实际编号以仓库当前最新 Migration 为准。Migration 只包含：

- active review subject 部分唯一索引；
- ReviewReport 与 WorkflowRun 的可空关系及真实审核约束；
- review_findings 的 Issue 扩展字段、约束和索引。

P0 Mock 数据不回填虚假 workflow_run_id，不重命名历史表，不删除 ReviewRecommendation，不修改历史 Migration，不重建数据库或 Volume。
