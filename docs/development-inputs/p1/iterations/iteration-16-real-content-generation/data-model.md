# CF-16-01B — 数据模型、索引与来源追溯

**状态：`review_ready_cf_16_01b`。** 本文件冻结目标数据库终态；Migration 文件由后续数据层任务创建。本轮不要求从新终态回滚到历史版本。

## 1. 复用实体

| 实体 / 表 | 处理 | Iteration 16 职责 |
|---|---|---|
| `ContentItem` / `content_items` | 复用 | 正文聚合和唯一当前版本指针 |
| `ContentVersion` / `content_versions` | 扩展 | 真实生成候选版本和来源追溯 |
| `WorkflowRun` / `workflow_run_records` | 复用并扩展 subject | 唯一 Runtime 生命周期 |
| `WorkflowRunEvent` / `workflow_run_events` | 扩展事件类型 | 记录结果消费成功或失败 |
| 项目工作流绑定 | 复用 | `content_generation` 阶段配置来源 |
| ChapterPlan、故事线、素材、伏笔 | 只读 | 构建输入和 context snapshot |

不新增 `Work`、`ContentCandidate` 或第二套 WorkflowRun 表。

## 2. WorkflowRun subject 扩展

`workflow_run_records` 新增可空字段：

| 字段 | 类型 | 规则 |
|---|---|---|
| `subject_type` | VARCHAR(40) NULL | Iteration 16 固定为 `content_item` |
| `subject_id` | UUID NULL | 固定为 `ContentItem.id` |

约束：

- `subject_type` 与 `subject_id` 同时为空或同时非空；
- `stage='content_generation'` 的新 Run 必须为 `subject_type='content_item'`；
- 服务端校验 subject 属于 Run 的 `project_id`；
- 旧 Runtime 数据允许两字段为空，不进行虚假回填。

数据库最终唯一索引：

```sql
CREATE UNIQUE INDEX workflow_run_records_one_active_subject_stage_idx
ON workflow_run_records(project_id, stage, subject_type, subject_id)
WHERE status IN ('queued', 'running') AND subject_id IS NOT NULL;
```

该索引保证同一正文同一阶段只有一个活跃 Run，同时允许同项目不同正文并行生成。

## 3. ContentVersion 扩展

`content_versions` 新增：

| 字段 | 类型 | 规则 |
|---|---|---|
| `source_content_version_id` | UUID NULL | 生成发起时的当前版本；同一 ContentItem |
| `source_content_version_version` | INTEGER NULL | 发起生成时源版本的乐观锁版本 |
| `source_workflow_run_id` | UUID NULL | 对应 P1 `workflow_run_records.id` |

`source` CHECK 增加 `workflow_generated`。

真实生成版本必须满足：

- `source='workflow_generated'`；
- `version_no >= 2`；
- `version=1`；
- `status='editable_draft'`；
- `source_content_version_id IS NOT NULL`；
- `source_content_version_version IS NOT NULL AND source_content_version_version >= 1`；
- `source_workflow_run_id IS NOT NULL`；
- `frozen_at IS NULL`。

数据库约束：

```sql
CREATE UNIQUE INDEX content_versions_source_workflow_run_unique_idx
ON content_versions(source_workflow_run_id)
WHERE source_workflow_run_id IS NOT NULL;

CREATE UNIQUE INDEX content_versions_item_version_no_unique_idx
ON content_versions(content_item_id, version_no);
```

为保证同一正文来源，建立/复用 `(content_item_id,id)` 唯一键，并以 `(content_item_id,source_content_version_id)` 建立复合外键。`source_workflow_run_id` 指向 `workflow_run_records(id)`；项目一致性由 Service 校验并在事务内锁定。

## 4. 版本语义

- `version_no` 是正文历史序号；新候选取当前最大值加一。
- `version` 是该 ContentVersion 的乐观锁号；新候选从 1 开始。
- 候选不是新状态枚举；`is_current=false && source=workflow_generated` 即可识别。
- 设为当前只更新 `content_items.current_version_id`，不复制正文、不删除旧版本。
- 候选基线过期不删除候选，仍保留审计和比较价值。

## 5. WorkflowRun Event 扩展

在既有 Event 类型上增加：

- `result_consumed`：领域结果已原子创建，payload 仅含 `contentItemId`、`contentVersionId`、`versionNo` 等安全标识；
- `result_consumption_failed`：输出已通过协议校验但事务失败，payload 只含安全错误码和可显示摘要。

Event 仅追加。`result_consumed` 不改变已终态 Runtime status；Summary 以 Event 与 `source_workflow_run_id` 唯一关系确认领域结果。

## 6. 输入和配置快照

继续使用 Runtime 的：

- `input_payload`：ContentItem、源版本、ChapterPlan 和上下文摘要；
- `configuration_snapshot`：脱敏工作流配置快照；
- `output_payload`：经过安全过滤的规范化输出；
- `error_code/error_message/error_details`：安全错误。

不得在 ContentVersion 重复保存完整配置或原始上游响应。ContentVersion 通过源 Run 可追溯输入和配置快照。

## 7. 查询模型

`ContentGenerationSummary` 是只读聚合，不新增表。它读取：

- ContentItem 当前版本；
- 该 subject 最新/活跃 `content_generation` Run；
- Run Events；
- 最新非当前 `workflow_generated` ContentVersion；
- 项目 `content_generation` 绑定可用性。

`ProjectWorkReadModel` 和现有版本列表继续复用，并扩展源 Run 摘要以支持真实 Runtime。

## 8. Migration 终态

后续 Migration 只包含：

1. `workflow_run_records` 增加 subject 字段、CHECK 和活跃唯一索引；
2. `content_versions` 增加三个来源字段；
3. 扩展 `content_versions.source` CHECK；
4. 增加来源复合 FK、Run FK 和唯一索引；
5. 扩展 WorkflowRun Event 类型约束；
6. 必要查询索引：
   - `(subject_type, subject_id, stage, created_at DESC, id DESC)`；
   - `(content_item_id, source, version_no DESC, id DESC)`。

禁止删除旧 P0 `workflow_runs`，禁止把历史 Mock Run 迁移进 P1 Runtime，禁止改写旧 ContentVersion 来源。
