# CF-16-01A — 数据模型、索引与来源追溯

**状态：`frozen_cf_16_01a`。** 本文件描述唯一开发数据库的 Iteration 16 最终结构；CF-16-02A 才创建新增 Migration。它不创建第二套正文或 Runtime 模型。

## 1. 复用与关系

| 实体 / 表 | Iteration 16 责任 |
|---|---|
| `ContentItem` / `content_items` | 正文逻辑聚合、`current_version_id` 和乐观锁 `version`。 |
| `ContentVersion` / `content_versions` | 不可变历史；扩展工作流候选来源。 |
| `WorkflowRun` / `workflow_run_records` | 唯一 Runtime 生命周期；扩展 subject。 |
| `WorkflowRunEvent` / `workflow_run_events` | 仅追加脱敏事件；增加消费成功/失败事件值。 |
| `idempotency_records` | 复用持久化命令重放，不新建业务幂等表。 |

`workId = ContentItem.id`。source ContentVersion 是输入基线，candidate ContentVersion 是输出，source WorkflowRun 是候选来源。不得新增 Work、ContentCandidate、第二套 Run 或消费状态表。

## 2. Runtime subject 与 active Run

`workflow_run_records` 新增 nullable `subject_type VARCHAR(40)`、`subject_id UUID`；两者须同时为空或同时非空。新建 `content_generation` Run 必须是 `subject_type='content_item'`、`subject_id=ContentItem.id`，并经服务层校验 ContentItem 属于 `project_id`。历史 Runtime 记录保持两列为空，不做虚假回填。

```sql
CREATE UNIQUE INDEX workflow_run_records_active_content_generation_subject_idx
ON workflow_run_records (project_id, stage, subject_type, subject_id)
WHERE stage = 'content_generation'
  AND status IN ('queued', 'running')
  AND subject_type = 'content_item'
  AND subject_id IS NOT NULL;
```

该部分唯一索引最终保证同一正文一个活跃生成 Run，且不阻止不同正文并行。另建查询索引 `(project_id, subject_type, subject_id, stage, created_at DESC, id DESC)`。

## 3. ContentVersion 候选来源

`content_versions` 新增 nullable：

| 字段 | 类型 | 规则 |
|---|---|---|
| `source_content_version_id` | UUID | 生成时当前版本；同一 ContentItem。 |
| `source_content_version_version` | INTEGER | 生成时源版本乐观锁版本，至少 1。 |
| `source_workflow_run_id` | UUID | 来源 `workflow_run_records` Run。 |

扩展 `content_versions_source_check` 以允许 `workflow_generated`。该来源的版本必须为 `editable_draft`、`version=1`、三项来源字段均非空、`frozen_at IS NULL`；其他来源保持兼容，允许新增列为 null。利用现有 `UNIQUE(content_item_id,id)` 建立 `(content_item_id, source_content_version_id)` 复合 FK，保证源版本同 Item；`source_workflow_run_id` 以现有 Run 主键 FK 指向 `workflow_run_records(id)`，项目归属由锁定的 ContentItem、subject 和服务层校验保证。一个 Run 一个候选：

```sql
CREATE UNIQUE INDEX content_versions_source_workflow_run_unique_idx
ON content_versions (source_workflow_run_id)
WHERE source_workflow_run_id IS NOT NULL;
```

保留现有 `UNIQUE(content_item_id, version_no)`；新增 `(content_item_id, source, version_no DESC, id DESC)` 索引。候选通过 `source=workflow_generated AND id<>current_version_id` 识别，不增加候选状态枚举。

## 4. Event、快照与 Summary

既有 `workflow_run_events.event_type` 增加 `result_consumed` 与 `result_consumption_failed` 合法值。成功 Event 只记录安全的 ContentItem/ContentVersion/序号标识；失败 Event 仅保存安全错误码和摘要。它们不改变 Runtime 已终态 status。

继续由 Run 存储脱敏 `input_payload`、`configuration_snapshot`、过滤后的 `output_payload` 和安全错误；候选不重复完整配置或原始上游响应。`ContentGenerationSummary` 是只读聚合，不新增表，读取当前版本、subject 最新/活跃 Run、Events、最新非当前工作流候选和绑定可用性。

## 5. 向前 Migration 与兼容规则

当前项目只有一个开发数据库。CF-16-02A 只能新增 Iteration 16 Migration，在该数据库持续向前演进并验收本迭代完成后的最终状态：新增 subject/约束/索引、三项来源字段/FK/唯一索引、source CHECK 和 Event 合法值。历史 Migration 文件保持不变；不修改历史业务数据以绕过问题。

不要求空库验证、历史数据库回滚、downgrade、删除 Volume、`docker compose down -v` 或重建数据库。历史 ContentVersion 与 Runtime 数据以 nullable 新列兼容，保持原来源和空 subject。
