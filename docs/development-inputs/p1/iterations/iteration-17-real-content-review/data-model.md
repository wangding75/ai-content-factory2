# Iteration 17 — 真实内容审核数据模型

**状态：`frozen_cf_17_01`。** 本迭代复用现有审核与 Runtime 模型，只做 Migration 17 向前兼容扩展，不创建第二套 Report、Issue 或 Run。

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

保留 P0 字段：`id/project_id/content_item_id/content_version_id/workflow_run_id/provider_key/status/conclusion/score/summary/created_at/completed_at`。

Migration 17 审计后的终态（已有列复用，只增加缺失列）：

| 字段 | 规则 |
|---|---|
| `workflow_run_id UUID NULL` | 复用并改为可空；真实审核必填；P0 历史 Mock 可为 null；非空时唯一 |
| `schema_version VARCHAR(40) NULL` | 新增；真实审核固定 `review.output.v1` |
| `source_content_version_version INTEGER NULL` | 新增；创建 Run 时来源版本乐观锁快照 |
| `source_content_hash CHAR(64) NULL` | 新增；SHA-256 安全摘要，不保存密钥或原始 Token |
| `completed_at TIMESTAMPTZ NOT NULL` | 复用既有列；真实报告完成时间 |

约束：

- `workflow_run_id` 非空时唯一，确保一个 Run 最多一个 Report；
- `provider_key=mock` 的非空 Run 关联按 P0 `workflow_runs` 作用域校验；`provider_key=runtime` 的非空 Run 关联按 `workflow_run_records` 的 `review/content_version/succeeded` 作用域校验；
- 延迟约束触发器在事务提交时执行上述双 Runtime 归属校验，避免新增同义 Run 列并保留 P0 写入；
- 真实 Report 的 `content_version_id` 必须等于 Runtime Run subject；
- `status=completed` 的真实 Report 必须具有 schema/version/hash；
- 失败 Run 不创建 ReviewReport。

API 字段 `sourceContentVersionId` 映射现有 `content_version_id`，不得再增加重复来源列。

## 4. ReviewIssue / review_findings 扩展

P0 `ReviewFinding` 在 API/领域层统一升级为 `ReviewIssue`，表名保持 `review_findings` 以避免破坏历史数据。

Migration 17 新增或复用字段：

| 字段 | 规则 |
|---|---|
| `issue_key` | 新增；真实 Report 内稳定唯一，P0 为 null |
| `sort_order` | 复用为 API `position`；P0 从 0 开始，真实 Issue 从 1 开始 |
| `category` | 复用为 API `categoryKey`，不得新增同义列 |
| `category_label` | 新增；真实 Issue 的中文展示名 |
| `severity` | 复用并兼容 P0 `low/medium/high`；真实 Issue 使用 `critical/warning/suggestion` |
| `title/description` | 复用；必填且受长度限制 |
| `evidence_json` | 必要短引文和来源引用 |
| `location_json` | 段落/句子范围，可空但结构合法 |
| `suggestion` | 问题级修改建议 |
| `disposition` | `open|ignored`，默认 `open` |
| `version` | Issue 处置乐观锁 |
| `ignored_at/ignored_by` | disposition=ignored 时记录 |
| `updated_at` | 处置更新时间 |

索引与约束：

- `UNIQUE(review_id, issue_key)`；
- 复用 `UNIQUE(review_id, sort_order)` 保障 API position 唯一；
- 查询索引 `(review_id, severity, disposition, sort_order)`；
- ignored 字段与 disposition 保持一致；
- Issue 不允许跨 Report、ContentItem 或 Project 引用。

## 5. Report 统计

P0 `score` 整数字段保留；真实审核不伪造 P0 分数，`score` 为 null。真实 `passedRuleCount` 从已持久化且通过 `review.output.v1` 校验的 WorkflowRun 输出读取；严重级别统计从不可变 Issue 内容派生，`openCount/ignoredCount` 从 disposition 查询派生。

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

只新增 Migration 17 向前变更；因现有 Runner 要求成对文件，down 文件只会拒绝 downgrade，不包含回退 SQL。Migration 17 只包含：

- active review subject 部分唯一索引；
- ReviewReport 的可空 Run 关系、双 Runtime 延迟归属校验及真实审核约束；
- review_findings 的 Issue 扩展字段、约束和索引。

P0 Mock 数据不回填虚假 workflow_run_id，不重命名历史表，不删除 ReviewRecommendation，不修改历史 Migration，不重建数据库或 Volume。
