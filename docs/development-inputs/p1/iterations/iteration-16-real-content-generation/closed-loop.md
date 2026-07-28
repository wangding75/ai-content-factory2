# Iteration 16 — 真实正文生成 Closed Loop

> 业务状态以 `business-rules.md` 为准。

## 1. 发起与确认

`/projects/{projectId}/works/{workId}`（`workId=ContentItem.id`）打开正文编辑器。用户选择当前 ContentVersion 作为基线、填写补充要求和上下文选项，点击生成后先执行无副作用预检。预检通过仅返回限时 Token 和报告；用户二次确认后才以 `Idempotency-Key` 创建 queued `content_generation` WorkflowRun。阻断、Token 过期、版本漂移或活跃 Run 冲突均不创建 Run，用户修正后重新预检。

## 2. 运行、恢复与失败

queued/running 显示安全状态、详情和流程中心入口；关闭提示仅隐藏提示，不取消 Run。刷新或重新进入时读取 `ContentGenerationSummary`、Runtime Run/Event 与版本历史恢复真实持久化状态。

| 场景 | 保持的数据 | 恢复动作 |
|---|---|---|
| 未配置 / 执行不可用 | 无 Run、无候选 | 去项目绑定或最近全局配置入口。 |
| Runtime failed/cancelled | 当前正文、Run 和安全错误；零候选 | Runtime Retry 创建新 Run。 |
| 输出校验失败 | 当前正文、Run 和安全错误；零候选 | 修复输出后 Runtime Retry 创建新 Run。 |
| 结果消费失败 | 当前正文、succeeded Run、失败 Event；零候选 | 专用消费 Retry，仅消费已有输出，不外呼。 |
| 刷新 | 所有持久化事实 | 读取 Summary，无额外写入。 |

## 3. 候选闭环

Runtime 成功 → 完整输出校验 → 单一事务创建 `workflow_generated` 非当前候选与 `result_consumed` Event → Summary 为 `candidate_ready` → 用户打开候选 → 与当前版本比较 → 明确“设为当前版本” → CAS 成功后仅更新 current pointer。

候选创建永不覆盖当前正文。运行期间或展示后当前版本变化，候选仍可打开和比较，但设为当前返回 `candidate_source_stale`；不存在强制覆盖入口，用户必须以最新当前版本重新生成。

## 4. 后续边界

候选成为当前版本后，用户可继续编辑；提交审核是 Iteration 17 的显式动作，固定指向明确 ContentVersion。本迭代不自动审核、不创建 ReviewReport、不触发重写。
