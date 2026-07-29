# CF-18-01B — Iteration 18 事务、并发与 Migration 设计

**状态：`frozen_cf_18_01b`。** 本文件冻结 Preflight/Create、输出消费、Runtime Retry、Result Consumption Retry、Set Current/CAS 五类边界，以及统一锁、幂等和失败事实规则；后续实现不得改变 CF-18-01A 的 API、状态或错误语义。

## 1. 统一事务原则

- Create Rewrite Run 使用 `SERIALIZABLE`；其余写事务使用 `SERIALIZABLE`，或在相同 advisory/row lock、唯一约束与 CAS 条件下提供等价一致性。
- 原始 Idempotency-Key、Preflight Token、凭据、内部 URL、原始上游响应不得进入 Run/Event/日志/错误；持久化只保存作用域键、请求哈希、首次安全响应和 Token nonce 消费事实。
- 所有命令先做同 Key 同请求的成功回放；同 Key 异请求立即返回 `idempotency_conflict`，不得继续执行业务检查。
- `rewrite.output.v1` 的严格解析、未知字段/尾随 JSON/长度/枚举/Issue 精确分区/安全字段校验必须发生在任何 Candidate 领域写入之前。
- 数据库部分唯一索引、Candidate 来源 Run 唯一索引和延迟来源触发器是最终保护，不得只靠“先查后写”。

## 2. 统一锁序

### 2.1 advisory lock

所有 Rewrite 写事务先按以下顺序取得事务级 advisory lock；不存在的项跳过：

1. 命令幂等：`idempotency:{scope}:{key}`；
2. Token nonce：`preflight-token:{nonce}`（仅 Create）；
3. ContentItem 聚合：`content-item:{contentItemId}`；
4. active subject：`rewrite-active:{projectId}:{reviewReportId}`（Create/Runtime Retry）；
5. WorkflowRun 消费：`rewrite-run:{runId}`（Consume/Consumption Retry/失败 Event）。

ContentItem/Report/Run ID 可先用普通一致性读取发现；取得 advisory lock 后必须在同一事务重新读取并锁定验证。相同层级的多个 UUID 按 UUID 字节升序取得。advisory lock 只负责序列化键或聚合，不能替代 FK、CHECK、部分唯一索引或 CAS。

### 2.2 row lock

取得 advisory lock 后，唯一 row lock 顺序为：

1. 已存在的 WorkflowRun（Run-centric 操作）；
2. source ContentVersion；
3. ContentItem；
4. ReviewReport；
5. ReviewIssue，按 `position ASC, id ASC`；
6. Candidate ContentVersion。

Create 尚无 WorkflowRun，故从 source ContentVersion 开始；Set Current 无需锁定来源 Run/Report/Issue，锁定 ContentItem 后锁 Candidate。ContentItem advisory lock 先于所有 row lock，消除 Create/Consume/Retry/Set Current 在同一 Item 上因历史成熟实现 row 顺序差异产生的交叉等待。

### 2.3 锁类型

| 事实 | 锁 |
|---|---|
| 幂等记录 | 先 advisory key lock；存在行再 `SELECT ... FOR UPDATE` |
| Token | nonce advisory lock；存在消费标记行 `FOR UPDATE` |
| source ContentVersion、ContentItem、ReviewReport、selected ReviewIssue、WorkflowRun、Candidate | 需要写前稳定性时 `FOR UPDATE` |
| Binding/Configuration/Connection | `FOR SHARE`，复核 ID/version/enabled/contract；不修改 |
| Availability、Preflight、Summary、History、Result GET | 普通一致性读取；Preflight 不取持久写锁 |
| active Run | active subject advisory lock + 普通查询；部分唯一索引最终裁决 |
| 失败 Event 去重 | `rewrite-run:{runId}` advisory lock + WorkflowRun `FOR UPDATE` 后查 Event |

`result_consumption_failed` / `output_validation_failed` Event 的去重不能用无锁的 exists-then-insert；锁定同 Run 后查询并最多追加一次。

## 3. Preflight：只读边界

Preflight 在一个只读一致性快照中：

1. 读取 ReviewReport、source ContentVersion、ContentItem、Project；
2. 读取 selected Issue，验证 1～50、ID 唯一、同 Report、`open`，并按 position/ID 排序；
3. 读取并验证 rewrite Binding、Configuration、Connection、`rewrite.input.v1` / `rewrite.output.v1`；
4. 查询同 Report active Rewrite Run；
5. 计算 source hash、Report/Issue/configuration/request digest；
6. 签发 10 分钟、单次使用、actor/作用域/nonce/expiry 全绑定 Token。

Preflight 不创建或修改 Run、Event、Candidate、ContentItem、ReviewReport、ReviewIssue、Token 行或命令幂等记录，不调用 Runtime/n8n。签发 Token 不是数据库持久化事件。

## 4. Create Rewrite Run

同一 Serializable 事务：

1. 取得命令幂等 advisory lock，`FOR UPDATE` 查询记录；同 Key 同请求优先回放；
2. 取得 Token nonce advisory lock，`FOR UPDATE` 查询消费事实并验证未消费、未过期；
3. 普通读取 Token 指向的 Item/Report 后取得 ContentItem 与 active subject advisory lock；
4. `FOR UPDATE` 锁定 source ContentVersion；
5. `FOR UPDATE` 锁定 ContentItem；
6. `FOR UPDATE` 锁定 ReviewReport，验证 completed/runtime、固定来源和 Project/Item；
7. 按 position/ID `FOR UPDATE` 锁定 selected ReviewIssue，验证 ID 唯一、同 Report、`open`、version 与 Token 快照一致；
8. `FOR SHARE` 复核 Binding、Configuration、Connection、input/output Contract；
9. 复核 actor、source ID/version/hash、Report/Issue、optionalInstructions、rewriteOptions、配置与完整请求摘要；
10. 查询 active Rewrite Run；
11. 创建 `queued` WorkflowRun，固定 `stage=rewrite/manual/review_report/reviewId`，持久化 `rewrite.input.v1` 与配置快照；
12. 创建唯一初始 `queued` Event；
13. 保存 Token nonce 消费事实；
14. 保存首次 201 幂等响应；
15. Commit，提交后才允许 Runtime dispatch。

任一步失败整笔回滚：零新 Run、零孤立 Event、Token 不误消费、无成功幂等记录。并发唯一冲突映射为 `active_rewrite_run_conflict`；不得把数据库索引名泄露给客户端。

## 5. Rewrite 输出校验与消费

Runtime succeeded 后先严格解码持久化 `output_payload`，校验 `rewrite.output.v1`、安全边界以及 addressed/unresolved 对 Run 输入 selected Issue 的恰好一次完整分区。失败只走第 8 节安全失败 Event，零 Candidate。

输出合法后，在同一事务：

1. 取得 ContentItem/Run advisory lock；
2. `FOR UPDATE` 锁定 WorkflowRun，验证 `stage=rewrite/status=succeeded/subject=review_report`；
3. 从锁定 Run 的 `input_payload` 重新读取固定 source/Report/Issue 快照，并确认输出仍是刚才验证的同一持久化值；
4. `FOR UPDATE` 锁定 source ContentVersion；
5. `FOR UPDATE` 锁定 ContentItem；
6. `FOR UPDATE` 锁定 ReviewReport，并按 position/ID 锁定 ReviewIssue；校验 Report/来源关系和 Issue 归属，但不因之后的 disposition 变化改写冻结输入；
7. 用 `source_workflow_run_id=:runId AND source='workflow_rewrite'` 查询已有 Candidate；存在则返回已有 Candidate；
8. 在已锁 Item 下计算 `MAX(version_no)+1`；
9. 创建唯一 `workflow_rewrite/editable_draft` Candidate，写三项既有来源字段；
10. 追加唯一 `result_consumed` Event；
11. Commit。

Candidate、来源关系和 `result_consumed` 原子提交。事务不更新 `current_version_id`，不修改来源版本、ReviewReport 或 ReviewIssue。并发消费由 Run/Item advisory lock、Run row lock、`content_versions_source_workflow_run_unique_idx` 与 `(content_item_id,version_no)` 共同收敛为最多一个 Candidate。

如果事务或 Commit 失败，全部领域写回滚；随后仅允许独立安全事务按第 8 节追加去重的 `result_consumption_failed`，不得留下部分 Candidate、矛盾的 `result_consumed` 或 current pointer 变化。

## 6. Retry

### 6.1 Runtime Retry

Runtime Retry 复用既有 WorkflowRun Retry 命令，在一个幂等事务中：

1. 取得幂等 advisory lock；同 Key 同请求幂等回放优先；
2. 普通一致性读取（normal consistent read）原 WorkflowRun，只用于发现 ContentItem、ReviewReport 和冻结 subject 标识；此步不得取得任何 row lock；
3. 取得 ContentItem advisory lock；
4. 取得 active subject advisory lock；
5. `FOR UPDATE` 重新读取并锁定原 WorkflowRun；
6. 在锁内重新校验原 Run、固定 source ContentVersion、ReviewReport、selected Issue 快照和 Retry 资格：仅允许 `failed`、`cancelled` 或具有 `output_validation_failed` Event 的 Rewrite Run；拒绝普通 succeeded、`result_consumption_failed`、`result_consumed` 或已有 Candidate；
7. 按统一 row lock 顺序锁定 source ContentVersion、ContentItem、ReviewReport 与按 `position ASC, id ASC` 排序的 ReviewIssue；
8. 检查 active Rewrite Run；
9. 创建新的 queued Rewrite Run，`retry_of_run_id` 指向原 Run；继承固定 source ContentVersion、ReviewReport、selected Issue 业务快照与 Binding/Configuration/Connection 快照；新 Run 只更新自身 `workflowRunId/correlationId`；禁止 `inputOverride`，`useCurrentConfiguration` 必须缺省或 false；创建初始 Event、保存首次 201 幂等响应；
10. Commit。

Runtime Retry 不重新消费原 Preflight Token，也不允许客户端改变冻结输入；新 Run 通过同一 active 部分唯一索引。

### 6.2 Result Consumption Retry

Result Consumption Retry 使用原 Run，不创建新 Run且不调用 Runtime/n8n：

1. 幂等回放优先；
2. 取得 Item/Run advisory lock并 `FOR UPDATE` 锁定原 WorkflowRun；
3. 校验 `expectedRunVersion`；
4. 已有同 Run Candidate 时保存/回放该 Candidate 结果；
5. 否则只允许 `stage=rewrite/status=succeeded`、存在去重的 `result_consumption_failed`、不存在 `output_validation_failed/result_consumed`；
6. 对原持久化输出重新执行严格校验；
7. 调用第 5 节同一消费事务函数；
8. 保存首次 200 幂等响应并 Commit。

成功后写 `result_consumed`，Summary 恢复 `candidate_ready`。再次请求返回同一 Candidate。

## 7. Set Current / CAS

不建立 Rewrite 专用采用系统，扩展复用 Iteration 16 `setCurrentContentVersion` 的候选资格。

同一事务：

1. 幂等 advisory lock + 幂等记录 `FOR UPDATE`，同 Key 同请求优先回放；
2. 取得 ContentItem advisory lock；
3. `FOR UPDATE` 锁定 ContentItem；
4. `FOR UPDATE` 锁定 Candidate ContentVersion；
5. 验证 Candidate 属于该 Item，`source=workflow_rewrite`、editable，来源 Run 为 succeeded Rewrite Run且已有 `result_consumed`；
6. Candidate 已是 `current_version_id` 时返回 200 no-op，并保存首次安全响应；
7. 比较 `expectedCurrentVersionId` 与实际 current ID；
8. 比较 `expectedCurrentVersion` 与实际当前 ContentVersion 的乐观锁 `version`；
9. 验证 Candidate 保存的 source ID/version 与冻结采用规则一致；
10. 以 `WHERE id=:itemId AND current_version_id=:expectedId AND version=:expectedItemVersion` CAS 更新 `current_version_id`、`status='draft'`、`reviewed_at=NULL`、Item `version+1`；
11. 保存首次 200 响应并 Commit。

CAS 影响行数为零或当前基线变化返回 `content_version_conflict`，不得静默覆盖。命令不创建 ContentVersion、不调用 Runtime、不修改来源版本、ReviewReport 或 ReviewIssue。

## 8. 安全失败 Event

输出校验或消费失败 Event 使用独立短事务：

1. 取得 `rewrite-run:{runId}` advisory lock；
2. `FOR UPDATE` 锁定 WorkflowRun；
3. 查询同 Run/同 event_type 是否已存在；
4. 不存在时追加一个只含安全 code/message/correlationId/occurredAt 的 Event；
5. Commit。

`output_validation_failed` 与 `result_consumption_failed` 不改变 succeeded Run 的 Runtime status。已有 Candidate 或 `result_consumed` 时不得再追加矛盾的消费失败 Event。

## 9. Migration 18

`000018_real_content_rewrite_foundation.up.sql`：

- 扩展 `content_versions_source_check` 允许 `workflow_rewrite`；
- 增加 Rewrite Candidate shape CHECK；
- 增加 Rewrite Run subject shape CHECK；
- 增加 `workflow_run_records_active_rewrite_subject_idx`；
- 增加两个延迟约束触发器，校验 Run → Report/Issue/input/configuration 与 Candidate → Run/Report/source Version；
- 不增加列或表，不更新业务数据，不回填历史来源。

`000018_real_content_rewrite_foundation.down.sql` 只删除 Migration 18 的触发器、函数、索引、CHECK，并恢复 Migration 17 的 source CHECK；不删除既有字段、索引或数据。唯一开发数据库禁止执行 Down。

## 10. 兼容性与后续实现边界

- Iteration 16 `workflow_generated` Candidate、Set Current/CAS 字段和约束保持；
- Iteration 17 Report/Issue、`open/ignored`、active review Run 与八状态保持；
- WorkflowRun 五状态与既有 Event 枚举不变；
- P0 Mock 与历史 ContentVersion 无需虚假来源；
- 本冻结不开发 Go、Web 或 n8n，不提前执行 CF-18-02A。
