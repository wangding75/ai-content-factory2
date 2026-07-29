# Iteration 18 — 事务、并发与 Migration 设计

**状态：`rebuild_candidate_2026_07_29`。** 本文件是 CF-18-01 的事务候选设计；实际 Migration 编号须在执行前基于仓库最新版本确定，不在本次文档/原型替换中创建。

## 1. Preflight

只读一致性边界读取并校验 Project、ContentItem、Report、来源 ContentVersion、selected Issue、配置版本和 active subject Run。它不写 Run、Event、ContentVersion、IssueLink 或幂等记录。

## 2. 创建 Run 事务

同一 Serializable 事务或等效一致性边界中：

1. 处理命令幂等；
2. 锁定并校验 Preflight Token nonce；
3. 锁定 ReviewReport、来源 ContentVersion 和 ContentItem；
4. 锁定/复核 selected ReviewIssue 集合及版本/disposition；
5. 重新校验 Binding、Configuration、Connection；
6. 检查同 Report active rewrite Run；
7. 创建 queued WorkflowRun、subject、配置/输入快照和初始 Event；
8. 消费 Token并保存幂等结果；
9. Commit。

任一步失败均不得留下半个 Run、孤立 Event、已消费 Token 或成功幂等记录。部分唯一索引处理并发最终冲突。

## 3. 输出校验与结果消费

严格输出校验在领域写事务前完成。事务内：

1. 锁定 succeeded WorkflowRun；
2. 校验 stage/subject、来源 Report/版本和冻结输入；
3. 检查是否已存在该 Run 的候选；有则返回既有结果；
4. 锁定 ContentItem，计算下一个 versionNo；
5. 创建完整 `workflow_rewrite` 非当前候选；
6. 为每个 selected Issue 创建 RewriteSourceIssueLink 和 resolution；
7. 校验 Link 数量与 issueKey 覆盖完整；
8. 追加 result_consumed Event；
9. Commit。

任一写入失败全部回滚。随后使用独立安全事务追加 result_consumption_failed；不得保留部分候选或部分 Link。

## 4. Retry

- Runtime Retry 创建新 Run并保留 retryOfRunId；继承原冻结输入，禁止 inputOverride。
- 结果消费 Retry 锁定原 succeeded Run，只重复第 3 节事务，绝不调用 n8n。
- 既有候选时返回该候选；唯一 sourceWorkflowRun 和 Link 唯一约束确保不重复。

## 5. 设为当前版本

完全复用 Iteration 16 CAS：锁定 ContentItem 和候选，验证 expected current ID/version 与候选 source ID/version；成功只更新 current pointer、status=draft、reviewedAt=NULL 和幂等结果。stale 候选保持可查看，不提供强制覆盖。

## 6. 幂等与并发矩阵

| 操作 | 作用域/请求指纹 | 并发结果 |
|---|---|---|
| 创建 Rewrite Run | report + operation；Token 冻结 Issue/来源/策略 | 同 Report 一个 active Run；同键回放首次 Run |
| 结果消费 Retry | workflowRun + operation；runId + expectedRunVersion | 同 Run 一个候选和一组 Link；不外呼 |
| 设为当前 | contentItem + operation；candidate + expected current | 同键回放；基线变化拒绝 |

原始 Idempotency-Key 不写入 Event、快照、日志或错误详情。

## 7. Migration 审计要求

CF-18-01 必须先检查 Iteration 16/17 实际表、约束和最新 Migration，避免重复字段、索引和误报。只为确实缺失的终态创建一个向前 Migration；本次替换包不修改 `apps/api/migrations`。
