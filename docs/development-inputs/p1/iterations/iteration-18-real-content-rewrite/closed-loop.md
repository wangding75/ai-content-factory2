# CF-18-01A — Iteration 18 真实正文重写 Closed Loop

**状态：`frozen_cf_18_01a`。** 业务语义以 `business-rules.md`、HTTP 字段以 OpenAPI 为准。

## 1. 审核报告入口

`I18_D2_REVIEW_REWRITE_ENTRY` 复用 Iteration 17 审核结果页。页面读取 `getReview` 与 `getContentRewriteAvailability`，只允许勾选同一 Report 的 1～50 个 `open` Issue；`ignored` 不可选。点击“创建重写任务”只进入创建流程，不创建 Run。

## 2. 配置、Preflight 与二次确认

`I18_D4_CREATE_REWRITE` 显示 ReviewReport 固定来源版本、服务端读取的 selected Issue、安全 Report 摘要、只读配置、重写策略和补充要求。用户提交 `selectedIssueIds`、`optionalInstructions` 与 `rewriteOptions` 执行无副作用 `preflightContentRewrite`。

通过后显示二次确认；只有确认动作才携带 `preflightToken` 与 `Idempotency-Key` 调用 `createContentRewriteRun`。首次返回 201，回放返回 200。Run 固定 `stage=rewrite`、`subjectType=review_report`、`subjectId=reviewId`。

`I18_D4_REWRITE_CONFIG_DRAWER` 只读展示项目 Binding/Configuration/Connection 摘要。关闭无副作用；修改统一跳转项目设置。

## 3. 排队与运行

`I18_D4_REWRITE_RUNNING` 承载 `queued/running` 文案变体，读取 `getContentRewriteSummary`，并复用 WorkflowRun Detail、Events 与 Cancel。页面展示固定来源、Report、selected Issue 数与安全执行进度。关闭状态条不取消任务；刷新从持久化 Summary/Run/Event 恢复，不写入。

Runtime 期间 ContentItem 当前版本变化不影响固定 Run 输入。

## 4. 输出、Candidate 与结果

```text
Runtime succeeded
→ 严格解析并校验 rewrite.output.v1
→ 原子创建一个 workflow_rewrite 非当前 ContentVersion 及必要关联事实
→ candidate_ready
```

未知字段、尾随 JSON、非法/重复/越界 Issue 引用或凭据/内部字段均导致 `output_validation_failed`，零 Candidate。消费事务任一步失败全部回滚并进入 `result_consumption_failed`，不得存在部分 Candidate 或部分关联数据。

`I18_D5_REWRITE_RESULT` 读取 `getContentRewriteSummary`、`getContentRewriteResult` 与 ContentVersion 只读详情，展示 source→candidate 关系、summary、addressedIssues、unresolvedIssues、warnings、允许的 metadata 与候选全文。Candidate 默认不是 current，打开编辑器只能进入只读预览。

## 5. Set Current

`I18_D5_SET_CURRENT_CONFIRM` 说明旧版本保留、原 ReviewReport 仍绑定来源版本、不自动审核。确认后调用现有 `setCurrentContentVersion`，提交 Candidate ID、expected current ID/version 与 Idempotency-Key。

- 成功或相同命令回放：200；
- Candidate 已是当前：200 no-op；
- current/Candidate 来源 CAS 漂移：409 `content_version_conflict`；
- 同 Key 异请求：409 `idempotency_conflict`。

成功后 Summary 仍为 `candidate_ready`，但 `candidateIsCurrent=true`、`canSetCurrent=false`；不新增状态。

## 6. 失败与恢复

| 场景 | 数据结果 | UI/恢复 |
|---|---|---|
| 未配置/配置失效且无既有事实 | 无新 Run | `I18_D4_REWRITE_AVAILABILITY`，跳转配置 |
| Runtime failed/cancelled | 零 Candidate | `I18_D4_REWRITE_FAILED`，Runtime Retry |
| 输出校验失败 | succeeded Run，零 Candidate | 同 Frame 输出校验变体，Runtime Retry |
| 结果消费失败 | succeeded Run，零 Candidate/部分关联 | `I18_D5_RESULT_CONSUMPTION_FAILED`，仅消费 Retry |
| 页面刷新 | 保留全部持久化事实 | 读取 Summary，不产生写入 |

Runtime Retry 仅允许 failed/cancelled/output_validation_failed，继承固定来源、Report、Issue 与配置快照，禁止 inputOverride。结果消费 Retry 仅消费原 succeeded Run 已持久化输出，不调用 Runtime/n8n、不创建新 Run；成功恢复 `candidate_ready`。

失败页面只展示安全 code/message/correlationId/time 和执行详情入口，不展示原始 JSON、SQL、连接、堆栈、n8n 节点、Webhook 或凭据。

## 7. 无副作用与边界

取消、关闭抽屉/弹窗、返回审核报告、关闭运行提示、查看配置、查看详情和刷新均不创建 Run/Candidate，不修改 Report/Issue/currentVersion。只有二次确认创建、明确 Retry 与 Set Current 会写入。

不自动重新审核、不自动二次重写、不自动解决 Issue、不自动发布。CF-18-01B 才冻结数据模型、事务、锁、索引和 Migration。
