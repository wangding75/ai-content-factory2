# Iteration 17 — 事务、并发与 Migration 设计

**状态：`frozen_cf_17_01`。** 目标是固定版本、Run、Report 和 Issue 的原子一致性；所有失败均不得留下部分审核结果。

## 1. Preflight

只读一致性边界读取 ContentVersion、ContentItem、Project、review Binding、Workflow Configuration、Connection 和 active subject Run。生成安全 input digest 与 10 分钟 Token。

Preflight 不冻结版本、不创建 Run/Event/Report/Issue、不写幂等记录、不调用外部工作流。

## 2. 创建 review Run 事务

同一事务或等效一致性边界内：

1. 检查命令幂等记录；
2. 校验并锁定 Preflight Token nonce；
3. 锁定 ContentVersion；
4. 锁定所属 ContentItem；
5. 校验 source ID/version/hash 与 Token 一致；
6. 复核 `review` Binding、Configuration、Connection 版本；
7. 检查同版本 active review Run；
8. 创建 queued Run、subject、input/configuration 快照；
9. 创建初始 Event；
10. 保存 Token 消费；
11. 保存命令幂等结果；
12. Commit。

提交后才交给 Runtime。任一步失败均无 Run、孤立 Event、Token 误消费或成功幂等记录。部分唯一索引是并发最终保证。

## 3. 输出验证

Runtime succeeded 后，在写事务前严格验证 `review.output.v1`：

- 单一 JSON 对象，无未知字段和尾随内容；
- schemaVersion、conclusion、summary、passedRuleCount 合法；
- issueKey 唯一，数量和字符串长度受限；
- severity、evidence、location、suggestion 结构合法；
- 不包含密钥、Authorization、Cookie、内部 URL、SQL、堆栈或原始上游包。

失败只追加安全 `output_validation_failed` Event，零 Report/Issue。

## 4. Report/Issue 消费事务

事务内：

1. 锁定 WorkflowRun；
2. 校验 stage=review；
3. 校验 status=succeeded；
4. 校验 subjectType=content_version；
5. 锁定来源 ContentVersion 与 ContentItem；
6. 查询该 Run 是否已有 Report；已有则幂等返回；
7. 创建 ReviewReport；
8. 按 position 批量创建全部 ReviewIssue；
9. 创建可选报告级 ReviewRecommendation；
10. 根据“来源是否仍为当前版本 + conclusion”条件更新 ContentItem 状态；
11. 追加 `result_consumed` Event；
12. Commit。

任一 Report、Issue、Recommendation、ContentItem 或 Event 写入失败全部回滚。回滚后使用独立安全事务追加 `result_consumption_failed`，不得产生部分 Report 或 Issue。

## 5. 结果消费 Retry

仅允许：

- stage=review；
- Run succeeded；
- 已存在 `result_consumption_failed`；
- 未存在 ReviewReport；
- 持久化输出仍通过严格校验。

Retry 只重复第 4 节事务，不调用 n8n，不创建新 Run。相同 Idempotency-Key 同请求返回首次 Report；同键异请求冲突。

## 6. Runtime Retry

failed、cancelled 或 output_validation_failed 走 Iteration 14 Runtime Retry，创建新 Run：

- retryOfRunId 指向原 Run；
- 继承同一来源 ContentVersion、subject 和配置快照；
- 不允许覆盖服务端字段；
- 不复用原 Preflight Token；
- 普通 succeeded、result_consumption_failed、已消费 Run 不允许 Runtime Retry。

## 7. Issue 处置事务

`open ↔ ignored` 命令包含 `expectedVersion` 与 `Idempotency-Key`。事务内锁定 Issue，校验 Report/Project 归属和版本，更新 disposition、ignored_at/by、version 和幂等结果。

处置不得修改 Report conclusion、P0 score、原始 issue 内容、正文或 WorkflowRun。

## 8. 历史与 Summary 并发

- 同一版本 active Run 由部分唯一索引限制；
- 同一 Run Report 由 workflow_run_id 唯一约束限制；
- 同一 Report Issue 由 issue_key/position 唯一约束限制；
- Summary 只把 Report 绑定到 LatestRun，不用历史 Report 伪装最新成功；
- 配置后续失效不能隐藏已有 queued/running/failure/report；
- 页面刷新只依赖持久化数据。

## 9. Migration 执行规则

Migration 17 在唯一开发数据库从 16 向前演进：增加 active review subject 索引、ReviewReport 兼容列与双 Runtime 延迟归属校验、ReviewIssue 字段/约束/索引。现有 `workflow_run_id` 同时承担 P0 与 P1 关联，因此不新增同义列：`mock` 报告校验旧 `workflow_runs`，`runtime` 报告校验 `workflow_run_records`。禁止修改历史 Migration、执行 downgrade、空库重放、删除 Volume、`down -v` 或手工篡改业务数据。
