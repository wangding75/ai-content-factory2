# Iteration 15 — 数据模型草案

> 状态：`business_semantics_frozen_cf_15_01a`。业务状态机以 `business-rules.md` 为准；具体字段、索引和表名留给 CF-15-01B，完整 API Schema 留给 CF-15-01C。

## 1. 复用模型

### 1.1 ChapterPlan

继续表示“当前章节”，不表示工作流原始候选。

保留核心语义：

- `pending_confirmation → confirmed`
- 只有允许状态可编辑、删除和确认；
- 只有 confirmed 可进入正文生产；
- `version` 用于乐观锁。

计划扩展来源追溯：

- `currentRevisionId`
- `sourceCandidateId`，可空
- `sourceCandidateBatchId`，可空
- `sourceWorkflowRunId`，可空

### 1.2 WorkflowRun / WorkflowRunEvent

直接复用 Iteration 14：

- `stage=chapter_planning`
- queued/running/succeeded/failed/cancelled
- 绑定、配置和连接安全快照
- Idempotency-Key 持久化
- Cancel / Retry
- Event 时间线

不得复制新的章节规划 Run 表。

## 2. 新增模型

### 2.1 ChapterPlanCandidateBatch

表示一次成功章节规划输出形成的候选集合。

建议字段：

- `id`
- `projectId`
- `sourceWorkflowRunId`，唯一
- `generationMode`: `full | append | range`
- `rangeStart`，可空
- `rangeEnd`
- `requestedChapterCount`
- `storylineSelectionSnapshot`
- `contextOptions`
- `additionalInstructions`
- `inputDigest`
- `status`: `ready | partially_adopted | adopted | abandoned`
- `candidateCount`
- `pendingCount`
- `adoptedCount`
- `discardedCount`
- `conflictCount`
- `completedAt`
- `abandonedAt`，可空
- `abandonReason`，可空
- `version`
- `createdAt`
- `updatedAt`

不变量：

- 一个 WorkflowRun 最多产生一个候选批次；
- 批次只在完整 Schema 校验和事务成功后可见；
- abandoned 后不可新增采用；
- 已采用章节不因批次 abandoned 回滚。

### 2.2 ChapterPlanCandidate

表示批次中的一个候选章节。

建议字段：

- `id`
- `batchId`
- `chapterNo`
- `baseChapterPlanId`，新增章节为空
- `baseChapterVersion`，新增章节为空
- `baseSnapshot`，用于差异与冲突判断
- `title`
- `summary`
- `purpose`
- `estimatedWords`
- `storylineRefs`
- `materialRefs`
- `foreshadowingRefs`
- `generationContextSummary`
- `diffType`: `new | replace | no_change | stale_conflict`
- `status`: `pending | stale | adopted | discarded`
- `adoptedChapterPlanId`，可空
- `adoptedRevisionId`，可空
- `adoptedAt`，可空
- `discardedAt`，可空
- `discardReason`，可空
- `version`
- `createdAt`
- `updatedAt`

约束：

- `UNIQUE(batchId, chapterNo)`；
- 只有 pending/stale 可编辑；
- 只有 pending 且基线未过期可采用；
- stale 不得直接采用，且不存在强制覆盖；
- adopted/discarded 为终态。

### 2.3 ChapterPlanRevision

保存当前章节不可变修订，支持替换历史和来源追溯。

建议字段：

- `id`
- `chapterPlanId`
- `revisionNo`
- `snapshot`
- `changeType`: `manual_create | manual_edit | candidate_adopt | confirm`
- `sourceCandidateId`，可空
- `sourceCandidateBatchId`，可空
- `sourceWorkflowRunId`，可空
- `createdBy`
- `createdAt`

约束：

- `UNIQUE(chapterPlanId, revisionNo)`；
- 修订不可原地更新；
- 候选替换必须先写修订，再更新 ChapterPlan 当前指针；
- 当前章节和修订必须在同一事务内一致。

## 3. 预检值对象

预检默认不要求持久化业务表，但创建 Run 必须绑定同一输入。

建议响应包含：

- `preflightToken`
- `inputDigest`
- `expiresAt`
- `summary`
- `checks[]`
- `blockers[]`
- `warnings[]`
- `workflowBindingSnapshot`
- `storylineTreeVersion`
- `projectContextVersion`

创建 Run 时：

- 提交 `preflightToken`；
- 服务端校验 Token 未过期且输入摘要一致；
- 对关键配置和活跃运行再次校验，避免检查后状态变化。

Token 具体实现属于契约冻结阶段，可使用服务端短期记录或签名值；不得包含密钥。

## 4. 状态关系

```text
WorkflowRun queued → running → succeeded
                              └→ failed/cancelled

WorkflowRun succeeded
→ validate normalized output
→ create CandidateBatch ready
→ candidates pending/stale
→ partial adoption
→ CandidateBatch partially_adopted
→ all candidates adopted/discarded
→ CandidateBatch adopted

CandidateBatch ready/partially_adopted
→ abandon
→ CandidateBatch abandoned
```

## 5. 事务边界

### 5.1 结果入库

一个事务内：

1. 校验 Run 尚未产生日志批次；
2. 创建 Batch；
3. 创建全部 Candidate；
4. 写来源和审计；
5. 提交。

任一步失败全部回滚。

### 5.2 单个采用

一个事务内：

1. 锁定 Candidate；
2. 校验 Candidate 状态和版本；
3. 校验当前 ChapterPlan 版本；
4. 创建 ChapterPlanRevision；
5. 创建或更新 ChapterPlan；
6. 更新 Candidate；
7. 更新 Batch 计数与状态；
8. 写审计。

### 5.3 批量采用

- 每个 Candidate 独立事务；一个失败不得回滚其他已提交 Candidate。
- stale/冲突项返回逐项结果，绝不写入当前章节。
- UI 已允许“采用无冲突项”，因此不得要求整批候选全部无冲突。
- 响应必须逐项返回 adopted、no_change、stale/conflict 或 failed，不能静默丢失。

## 6. 索引建议

- Batch: `(project_id, created_at DESC, id DESC)`
- Batch: `(project_id, status, created_at DESC)`
- Batch: unique `(source_workflow_run_id)`
- Candidate: unique `(batch_id, chapter_no)`
- Candidate: `(batch_id, status, chapter_no)`
- Candidate: `(base_chapter_plan_id, base_chapter_version)`
- Revision: unique `(chapter_plan_id, revision_no)`
- ChapterPlan: unique `(project_id, chapter_no)`

## 7. 安全和脱敏

- 不保存原始 API Key、Authorization、Cookie 或上游请求头。
- 不把 WorkflowConnection 原始配置复制到 Batch/Candidate。
- 只保存 Iteration 14 已脱敏快照引用或安全摘要。
- 错误只保存安全错误码、用户摘要和 request_id。
- 生成上下文摘要不得包含未授权原始素材全文。
- 原始 Idempotency-Key 不进入业务表、日志、报告或 UI。

## 8. Migration 验收

- 新表、字段、索引和外键以最终冻结模型为准。
- 只验收当前 Iteration 终态：
  - 空库可执行；
  - 当前表结构正确；
  - 数据写入、查询、关联和事务正确；
  - 环境最终状态正确。
- 不要求从当前版本回滚到历史版本。
