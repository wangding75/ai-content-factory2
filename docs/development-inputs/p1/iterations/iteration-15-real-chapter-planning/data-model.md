# CF-15-01B — 数据模型与 Migration 设计冻结

**状态：`frozen_cf_15_01b`。** 本文件冻结 CF-15-01A 规则的数据结构、关系、约束和 Migration 终态设计；`business-rules.md` 仍是业务语义的唯一来源。CF-15-01C 才冻结完整 OpenAPI 字段 Schema 和错误码。本任务不创建 Migration 文件。

## 1. 复用边界

| 实体 / 表 | 处理 | 职责 |
|---|---|---|
| `WorkflowRun` / `workflow_run_records` | 复用并扩展索引 | 章节规划 Run 的唯一 Runtime；保存脱敏绑定快照、输入、输出、状态与版本。 |
| `WorkflowRunEvent` / `workflow_run_events` | 原样复用 | Run 的只追加脱敏事件；不承担候选消费事件或候选审计。 |
| `ProjectWorkflowBinding` / `project_workflow_bindings` | 原样复用 | 每项目每 stage 的唯一绑定；其版本进入预检和 Run 快照。 |
| `Storyline` / `storylines` | 原样复用 | 递归上下文来源；候选只保存引用快照，故事线不拥有章节。 |
| `ChapterPlan` / `chapter_plans` | 扩展来源与当前修订指针 | 当前章节的唯一模型，保持 P0 确认与正文门槛。 |
| `idempotency_records` | 复用 | 所有 CF-15 命令的唯一持久化重放记录；不新建业务幂等表。 |

不得建立第二套 WorkflowRun、WorkflowRunEvent、ChapterPlan 或章节确认模型。`mock_generation_runs` 只保留 P0 历史兼容，不参与 Iteration 15。

## 2. 实体、字段与关系

所有新增主键均为 UUID，所有时间为 `TIMESTAMPTZ`，所有可变聚合均有 `version INTEGER NOT NULL DEFAULT 1 CHECK (version >= 1)`、`created_at` 与 `updated_at`。项目删除时可级联删除其候选域；正常业务操作不物理删除 Batch、Candidate 或 Revision。

### 2.1 `chapter_plan_candidate_batches`

一次成功结果消费的项目级候选集合。字段：`id`、`project_id`（FK `projects`）、`source_workflow_run_id`（与 `workflow_run_records(project_id,id)` 的复合 FK，唯一）、`generation_mode`、`range_start`、`range_end`、`requested_chapter_count`、`input_digest CHAR(64)`、`input_snapshot JSONB`、`storyline_selection_snapshot JSONB`、`context_options JSONB`、`additional_instructions`、`workflow_binding_snapshot JSONB`、`status`、计数 `candidate_count/pending_count/stale_count/adopted_count/discarded_count`、`completed_at`、`abandoned_at`、`abandon_reason`、`created_by`、`updated_by`、审计与版本字段。

`input_snapshot` 保存生成模式、目标和安全上下文摘要；`workflow_binding_snapshot` 只保存 Run 已脱敏快照的安全摘要或版本引用。两者均须为 JSON object，均不得保存凭据、原始第三方响应或原始幂等键。状态仅为 `ready`、`partially_adopted`、`adopted`、`abandoned`。

### 2.2 `chapter_plan_candidates`

一个 Batch 内一个章节号的候选。字段：`id`、`batch_id`（与 `chapter_plan_candidate_batches(project_id,id)` 的复合 FK）、`project_id`（FK Project）、`chapter_no`、`sort_order`、`base_chapter_plan_id`（可空复合 FK ChapterPlan，`ON DELETE SET NULL`）、`base_revision_id`（可空复合 FK Revision，`ON DELETE SET NULL`）、`base_chapter_version`（可空）、`base_snapshot JSONB`、`generated_snapshot JSONB`、`current_snapshot JSONB`、`diff_type`、`status`、`adopted_chapter_plan_id`（可空复合 FK ChapterPlan，`ON DELETE RESTRICT`）、`adopted_revision_id`（可空复合 FK Revision，`ON DELETE RESTRICT`）、`adopted_at`、`discarded_at`、`discard_reason`、`created_by`、`updated_by`、`last_edited_by`、`last_edited_at`、审计与版本字段。

`generated_snapshot` 是经规范化校验后原样保留的完整候选内容，永不更新；`current_snapshot` 是用户可编辑的完整候选内容，初始与 generated 相同；`base_snapshot` 是生成时当前 ChapterPlan 的完整安全快照（新增候选为空）。三者均包含章节内容及其 Storyline/Material/Foreshadowing 有序引用快照，引用对象只含 ID、展示安全摘要、关系/位置及版本；不复制素材全文。差量不持久化为事实，只在 Compare/Recompare 时由三份完整快照计算。状态仅为 `pending`、`stale`、`adopted`、`discarded`；`edited` 绝不是状态。

### 2.3 `chapter_plan_revisions`

不可变的当前章节完整历史。字段：`id`、`chapter_plan_id`（与 `chapter_plans(project_id,id)` 的复合 FK）、`project_id`（FK Project）、`revision_no`、`snapshot JSONB`、`change_type`、`source_candidate_id`（可空复合 FK Candidate，`ON DELETE RESTRICT`）、`source_candidate_batch_id`（可空复合 FK Batch，`ON DELETE RESTRICT`）、`source_workflow_run_id`（可空复合 FK Run，`ON DELETE RESTRICT`）、`created_by`、`created_at`。

`snapshot` 是完整 ChapterPlan 内容及关系集合，不是差量；Revision 无 `updated_at`/`version`，禁止 UPDATE 和物理 DELETE。`revision_no` 在同一 ChapterPlan 内严格顺序唯一。正常 `change_type` 为 `manual_create`、`manual_edit`、`candidate_adopt`、`confirm`；Migration 回填历史可使用仅限迁移的 `legacy_backfill`。任何 Adopt 必须产生 `candidate_adopt` Revision；确认继续复用 P0 事务语义，并在同一事务追加 `confirm` Revision。

### 2.4 `chapter_plans` 扩展

保留现有主键、`project_id + chapter_no` 唯一、状态、版本、P0 关系表和确认形状约束。新增可空 `current_revision_id`（FK Revision，`ON DELETE RESTRICT`）、`source_candidate_id`、`source_candidate_batch_id`、`source_workflow_run_id`（均为可空、`ON DELETE RESTRICT`）。`source` 枚举扩展为既有 `mock_generated` 与 `candidate_adopted`；候选采用写入后者。新 Adopt 创建或替换后都写 `pending_confirmation`，即使原 ChapterPlan 已确认。`current_revision_id` 必须指向同一 `chapter_plan_id` 的 Revision，由事务和约束触发器保证；不能用普通跨表 FK 单独保证。

### 2.5 结果消费、绑定与故事线

不新增消费状态表。`chapter_plan_candidate_batches.source_workflow_run_id` 的唯一约束是成功消费的持久化事实；无 Batch 即表示该 Run 尚未成功消费。非法输出或事务失败只写既有 Run 的安全错误/Event，不写 Candidate 域。预检 Token 无数据库业务表；方案见第 5 节。ProjectWorkflowBinding 与 Storyline 不复制到候选域，只有它们的安全版本快照或引用进入 Batch/Candidate。

## 3. 唯一约束、索引与并发最终保证

| 对象 | 约束 / 索引 | 用途 |
|---|---|---|
| Batch | `UNIQUE(source_workflow_run_id)` | 同一 Run 最多消费出一个 Batch。 |
| Batch | `UNIQUE(project_id, id)` | Candidate 与来源的复合 FK 保证 Batch 项目归属。 |
| Batch | `(project_id, created_at DESC, id DESC)`；`(project_id, status, created_at DESC, id DESC)` | 项目列表、状态筛选与稳定分页。 |
| Candidate | `UNIQUE(batch_id, chapter_no)`；`UNIQUE(batch_id, sort_order)` | 每 Batch 的章节号与排序稳定唯一。 |
| Candidate | `(batch_id, status, chapter_no, id)`；`(project_id, base_chapter_plan_id, base_chapter_version)` | 列表、状态筛选、基线冲突查询。 |
| Revision | `UNIQUE(chapter_plan_id, revision_no)`；`(chapter_plan_id, revision_no DESC)` | 顺序唯一与历史读取。 |
| Revision | `UNIQUE(project_id, id)` | Candidate 与 ChapterPlan 来源的复合 FK 保证项目归属。 |
| ChapterPlan | 保留 `UNIQUE(project_id, chapter_no)`；新增来源 FKs 索引 | 当前章节唯一与追溯查询。 |
| Runtime | 新增 `UNIQUE(project_id, id)` | 供 Batch/Revision 的复合来源 FK 使用。 |
| Runtime | `UNIQUE INDEX workflow_run_records_one_active_chapter_planning_per_project_idx ON workflow_run_records(project_id) WHERE stage='chapter_planning' AND status IN ('queued','running')` | 数据库最终保证每项目仅一个活跃章节规划 Run。 |

所有命令的更新谓词都同时包含实体 `id` 与 `version`；更新成功时 `version=version+1`。单个 Adopt 还必须同时比较 Candidate `expectedVersion` 和目标 ChapterPlan `expectedVersion`；新增场景以 `UNIQUE(project_id, chapter_no)` 作为并发最终保证。PostgreSQL `23505` 的 Batch source Run、活跃 Run、章节号或 Revision 唯一冲突均映射为稳定领域冲突，不泄露约束名或 SQL。

## 4. 状态与保留规则

Candidate 迁移严格遵从 01A：`pending → stale/adopted/discarded`，`stale → pending/discarded`，`adopted` 与 `discarded` 为终态。Batch 状态从 Candidate 事实派生并在同一写事务更新；`abandoned` 永不回滚已采用 Revision 或 ChapterPlan。Candidate、Batch、Revision 与其审计记录均保留；只允许项目级级联删除。

## 5. Preflight Token 数据方案

采用**无持久化的签名 Token**，不写任何业务表、Run、Batch、Candidate 或 Revision。Token 使用服务端密钥 HMAC-SHA-256 签名并带 `kid`，负载仅含：`projectId`、`actorId`、固定 `stage=chapter_planning`、generationMode、规范化 target、`inputDigest`、`projectContextVersion`、`storylineTreeVersion`、`bindingId/bindingVersion`、`iat`、`exp` 和随机 `jti`。

`exp = iat + 10 分钟`。Token 不含任何凭据、原始配置、上游响应或章节内容。每次预检签发新的 jti/token；创建 Run 验签、检查未过期和主体/输入摘要完全一致，并重新读取项目、故事线、Binding、其版本与活跃 Run。任一版本、策略、范围、输入摘要或 actor 改变，或者 Token 过期/签名/kid 无效，即拒绝并要求重新预检。Token 允许同一有效输入重放，但创建 Run 的持久化幂等和活跃 Run唯一索引阻止重复业务结果。

## 6. Migration 终态设计

只在 CF-15-02A 创建实际 Migration，顺序固定为：

1. 创建 `chapter_plan_candidate_batches`、`chapter_plan_candidates`、`chapter_plan_revisions` 及其 CHECK、FK、唯一约束和索引；
2. 为 `chapter_plans` 新增 nullable 来源字段与 `current_revision_id`，扩展 source CHECK；
3. 为每个现存 ChapterPlan 生成 `legacy_backfill` Revision 完整快照，回填 `current_revision_id`；
4. 安装保证 `current_revision_id` 与 ChapterPlan 同属的约束触发器；
5. 创建 Runtime 活跃 Run partial unique index；
6. 在事务性迁移结束前校验每个 ChapterPlan 都有 current Revision、所有现有数据满足新约束。

新表 project FK 使用 `ON DELETE CASCADE`；所有来源、Batch、Candidate、Revision 和 ChapterPlan 关系均使用带 `project_id` 的复合 FK，禁止跨项目引用。追溯性来源 FK 使用 `RESTRICT`，可被 P0 正常删除的生成时基线指针使用 `SET NULL`，同时完整 `base_snapshot` 保留历史。所有 JSONB 快照必须为 object；默认空对象仅用于非空技术快照，业务快照必须由应用写入。当前数据库终态验收覆盖空库升级、既有 ChapterPlan 回填、唯一索引、FK/删除策略、事务回滚与并发冲突；不要求历史版本回滚。
