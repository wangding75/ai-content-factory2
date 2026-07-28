# CF-16-01B — 事务、并发与 Migration 设计

## 1. 预检

只读事务或一致性读：读取 ContentItem、当前版本、confirmed ChapterPlan、上下文引用、项目绑定、配置/连接版本和活跃 subject Run。预检不得写 Run、Event、Version 或幂等记录。

## 2. 创建 Run 事务

1. 校验并消费预检 Token 语义；
2. 锁定 ContentItem 和当前版本；
3. 复核 `expectedCurrentVersionId/version`；
4. 复核绑定、配置版本和执行可用性；
5. 依赖数据库唯一索引检查活跃 subject Run；
6. 按 Iteration 14 Runtime 事务创建 queued Run、初始 Event、subject 和脱敏快照；
7. 持久化 Idempotency-Key 指纹；
8. 提交后交给既有执行适配层。

任一步失败均不得创建半个 Run 或孤立 Event。

## 3. 结果消费事务

协议校验在事务外完成；全部合法后：

1. 锁定 source Run 和 ContentItem；
2. 查询 `source_workflow_run_id` 已有版本；存在则返回既有成功结果；
3. 计算 `MAX(version_no)+1`，依赖唯一约束处理并发；
4. 创建一个 `workflow_generated` editable ContentVersion；
5. 不更新 `ContentItem.current_version_id`；
6. 追加 `result_consumed` Event；
7. 原子提交。

数据库失败时整笔回滚，并在独立安全事务追加 `result_consumption_failed` Event。不得保留 Version、来源关系或 current pointer 的部分写入。

## 4. 结果消费重试

专用重试只允许：

- Run 属于当前项目和 `content_generation`；
- Runtime 已 `succeeded`；
- 输出已通过协议校验；
- 存在 `result_consumption_failed` 且不存在 `result_consumed`；
- 没有 `source_workflow_run_id` 对应版本。

重试不调用外部执行器。成功后创建版本并追加 `result_consumed`。同 Idempotency-Key 和同 payload 返回首次结果；Run 已消费时返回既有版本。

## 5. 设为当前版本事务

1. 锁定 ContentItem；
2. 校验当前版本 ID 和乐观锁版本分别等于 `expectedCurrentVersionId`、`expectedCurrentVersion`；
3. 锁定候选版本并校验同一 ContentItem；
4. 校验候选 `source=workflow_generated`、非当前、editable；
5. 校验候选保存的源版本 ID 和源版本乐观锁号都等于当前版本；
6. 更新 `current_version_id=candidate.id`、`status=draft`、`reviewed_at=NULL`、`updated_at`；
7. 提交并返回 ContentItemDetail。

任何版本漂移返回 409；禁止强制覆盖。旧版本和 ReviewReport 不修改。

## 6. 幂等作用域

| 操作 | 作用域 | 指纹输入 |
|---|---|---|
| 创建生成 Run | content-item + operation | preflightToken 对应的完整冻结输入 |
| 结果消费重试 | workflow-run + operation | runId + expectedRunVersion |
| 设为当前版本 | content-item + operation | candidateVersionId + expectedCurrentVersionId + expectedCurrentVersion |

同键不同指纹返回 `idempotency_key_reused_with_different_payload`。

## 7. 并发矩阵

| 并发场景 | 结果 |
|---|---|
| 同正文同时创建两个 Run | 唯一索引只允许一个，另一个 `active_run_conflict` |
| 不同正文并行创建 Run | 允许 |
| Run 期间当前版本被保存 | source version 乐观锁变化；创建前漂移则拒绝，创建后候选保留但不能直接设为当前 |
| 同 Run 重复消费 | 唯一 source Run 返回同一候选 |
| 两个 Run 同时分配 version_no | item/version_no 唯一约束；冲突事务重试或安全失败 |
| 候选展示后当前版本变化 | 设为当前返回 `candidate_source_stale` |
| 设为当前命令重放 | 同 Key 返回首次结果，不重复改变状态 |
| 结果消费失败后重试 | 不外呼，原子创建一次候选 |

## 8. 安全

错误详情、Event payload 和输入快照不得包含密钥、Authorization、Cookie、内部地址、原始 HTTP body、SQL、堆栈、原始幂等键或未经授权的完整素材正文。
