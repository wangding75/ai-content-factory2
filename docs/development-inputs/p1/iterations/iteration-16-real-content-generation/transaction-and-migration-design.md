# CF-16-01A — 事务、并发与 Migration 设计

**状态：`frozen_cf_16_01a`。** 本文件实现业务规则与数据终态，不改变 Runtime 基础生命周期。

## 1. 预检

只读一致性边界读取 ContentItem/当前版本、confirmed ChapterPlan、上下文、项目绑定、配置/连接版本和活跃 subject Run。它不写 Run、Event、Version 或幂等记录。

## 2. 创建 Run 事务

同一事务或等效一致性边界内：验证并复核 preflight Token；锁定 ContentItem 与当前版本；校验 expected 当前 ID/version 未漂移；复核绑定、配置与执行可用性；检查 active subject Run；创建 queued Runtime Run、初始 Event、subject 和脱敏快照；保存 idempotency 结果。部分唯一索引是并发最终保证。提交后才交给执行适配层；任一步失败均不留下半个 Run、孤立 Event 或幂等成功记录。

## 3. 结果消费事务

完整协议校验在写事务前完成。事务内锁定 WorkflowRun 与 ContentItem，检查来源 Run 是否已有候选；已有则返回既有结果。否则计算 `MAX(version_no)+1`，创建完整 `workflow_generated` 候选及来源关系，不更新 `current_version_id`，追加 `result_consumed` Event，并原子提交。

任一写入失败整笔回滚，绝不保留部分 ContentVersion、来源关系或 current pointer。随后用独立安全事务写 `result_consumption_failed` Event。专用 Retry 仅允许 succeeded、已通过输出校验、存在消费失败且未消费的 content_generation Run；只重复以上结果消费，绝不调用 n8n/外部工作流。唯一 source Run 约束和消费幂等确保最多一个候选。

## 4. 设为当前版本事务

同一事务内锁定 ContentItem，验证 `expectedCurrentVersionId` 与 `expectedCurrentVersion`，锁定候选并验证同 Item、非当前、`workflow_generated`、editable；同时验证候选的 source ID/version 仍等于实际当前版本。成功后更新 `current_version_id`、ContentItem `status=draft`、`reviewed_at=NULL`、时间戳与命令幂等结果。任何漂移返回冲突（源基线漂移为 `candidate_source_stale`），没有强制覆盖；旧版本与审核记录不修改。

## 5. 幂等与并发矩阵

| 操作 | 幂等作用域 / 指纹 | 并发结果 |
|---|---|---|
| 创建 Run | content-item + operation；Token 冻结输入 | 同正文一个 active Run；不同正文并行。 |
| 结果消费 Retry | workflow-run + operation；runId + expectedRunVersion | 同 Run 返回同一候选；不外呼。 |
| 设为当前 | content-item + operation；candidate ID + expected 当前 ID/version | 同键重放首次结果；基线变化拒绝。 |

同一键同一规范化请求返回原结果；同一键不同请求返回 `idempotency_key_reused_with_different_payload`。所有原始 Idempotency-Key 仅经服务端安全派生后持久化，不能写入 Event、快照、日志或错误详情。

## 6. Migration 执行规则

CF-16-02A 在唯一开发数据库上新增向前 Migration；只验收最终结构。禁止修改历史 Migration、历史回滚/downgrade、空库验证、重建数据库、删除 Volume 或手工篡改业务数据。Migration 失败按当前数据库实际状态定位并修复新增 Migration，不改写历史文件。
