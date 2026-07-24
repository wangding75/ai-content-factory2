# CF-15-01B — 事务、幂等与 Migration 执行设计

**状态：`frozen_cf_15_01b`。** 本文实现 `data-model.md` 的事务边界，不改变 `business-rules.md` 状态机。

## 1. 事务矩阵

| 操作 | 锁 / 前置校验 | 同一事务内写入 | 失败与重放 |
|---|---|---|---|
| Run 结果消费 | 锁定 succeeded Run；完整输出校验；检查 source Run 唯一 | Batch、全部 Candidate、来源审计 | 任一非法或写失败全部回滚，零 Batch/零 Candidate；重复消费返回既有 Batch。 |
| Candidate 编辑 | 锁 Candidate；`pending/stale` 与 expected Candidate version | `current_snapshot`、diff/stale 重算、编辑审计、Candidate version | 版本冲突零写入；同键同载荷回放原结果。 |
| Recompare | 锁 Candidate 和目标 ChapterPlan（如有） | 新差异事实、Candidate stale/pending 状态与 version、审计 | 不更新 generated/base snapshot；版本冲突零写入。 |
| 单个 Adopt | 锁 Candidate、Batch 与目标 ChapterPlan；双 version 校验 | 新/更新 ChapterPlan、Revision、关系、来源、Candidate adopted、Batch 计数/状态、审计 | 任一失败全部回滚；no_change 不创建 Revision。 |
| 批量 Adopt | 先稳定排序；每个 Candidate 单独启动事务 | 每一项遵从单个 Adopt | 允许部分成功；外层响应与每项派生幂等记录完整重放。 |
| Candidate discard | 锁 Candidate、Batch；校验可处理状态/version | Candidate discarded、Batch 计数/状态、审计 | adopted/discarded 不被逆转；重放返回首次结果。 |
| Batch abandon | 锁 Batch；锁全部尚未采用 Candidate | 待处理/stale Candidate discarded、Batch abandoned、审计 | 已 adopted Candidate、Revision、ChapterPlan 不更新；重放返回首次结果。 |
| Chapter Confirm | 复用 P0 选中集合原子事务；锁 ChapterPlan/version | ChapterPlan confirmed、confirm Revision、审计 | 任一选中项冲突则整次 Confirm 回滚；已有 P0 确认幂等保持。 |

结果消费在读取 Run 及其安全输出后才开启写事务；输出 Schema 校验不得依赖部分持久化。单个 Adopt 的 ChapterPlan 关系表替换、Revision 与 current revision 指针须同事务提交。Batch 状态只能由事务内重新计数的 Candidate 事实得出，不能由客户端传入。

## 2. 幂等与版本

复用 `idempotency_records` 与其 advisory transaction lock。为避免保留原始 `Idempotency-Key`，CF-15 将 HTTP 原始值以服务端 HMAC-SHA-256 派生为 64 字符 key fingerprint 后才传给 Repository；原值不得写入 Batch、Candidate、Revision、Audit、日志或 idempotency_records。scope 固定包含操作与资源：`chapter-plan-candidate-adopt:<candidateId>`、`chapter-plan-batch-adopt:<batchId>`、`chapter-plan-candidate-discard:<candidateId>`、`chapter-plan-batch-abandon:<batchId>`，以及章节规划 Run 创建 scope。

请求 hash 是规范化请求体和作用域资源的 SHA-256；同 scope/key/hash 返回持久化首次响应，同 scope/key 不同 hash 返回幂等冲突。批量采用的外层 key 保护完整逐项响应；每项使用从外层 fingerprint、Candidate ID 和请求 hash 派生的内部 scope/key，因此在外层响应写入前中断后重放也不会重复采用已成功项目。

`expectedVersion` 指向：编辑/Recompare/Discard 为 Candidate；Abandon 为 Batch；单个 Adopt 为 Candidate 和既有目标 ChapterPlan；批量 Adopt 为 Batch 及每项 Candidate/目标 ChapterPlan；Confirm 沿用 P0 每项 ChapterPlan。数据库唯一冲突经 Repository 分类为领域 `active_run_conflict`、`run_already_consumed`、`chapter_no_conflict`、`revision_sequence_conflict` 或 `idempotency_conflict`；完整 HTTP 错误码由 01C 冻结。

## 3. 审计与安全

所有本任务写操作使用现有 `audit_logs`，记录 actor、动作、主体、请求安全摘要、版本前后值和关联 ID；不保存密钥、原始上游响应、原始 Idempotency-Key、SQL 或堆栈。WorkflowRun Event 仍只描述 Runtime 生命周期；候选消费/采用审计不新建第二套 Event 模型。

## 4. Migration 验收清单

- 空库与当前数据库升级均得到相同终态；现存 ChapterPlan 回填一个不可变完整 Revision 并建立 current 指针。
- 部分唯一索引拒绝同项目两个 `queued/running chapter_planning` Run；Run 终态后可创建下一 Run。
- source Run、跨项目来源、Batch 内章节号、Revision 序号及项目内 ChapterPlan 章节号均由唯一或复合 FK 约束拒绝并映射领域冲突。
- 任一结果候选非法或写失败时 Batch/Candidate 数量为零；单项 Adopt、Discard、Abandon 与 Confirm 的回滚符合上表。
- Batch abandon 不改变 adopted Revision/ChapterPlan；所有外键删除策略、JSONB object 检查和版本谓词均已验证。
