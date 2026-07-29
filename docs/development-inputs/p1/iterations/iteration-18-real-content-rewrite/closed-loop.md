# Iteration 18 — 真实正文重写 Closed Loop

**状态：`rebuild_candidate_2026_07_29`。** 业务语义以 `business-rules.md` 为准。

## 1. 审核报告入口

`I18_D2_REVIEW_REWRITE_ENTRY` 复用 Iteration 17 审核结果页。用户勾选 1～50 个 `open` Issue；`ignored` 不可选。点击“创建重写任务”只进入创建页，不直接创建 Run。

## 2. 创建与预检

`I18_D4_CREATE_REWRITE` 显示固定来源版本、Report、选中 Issue、全局只读约束、只读工作流配置、重写策略和补充要求。页面先执行无副作用 Preflight；通过后用户二次确认，携带 Token 与 Idempotency-Key 创建 queued Run。

`I18_D4_REWRITE_CONFIG_DRAWER` 只读展示当前项目绑定。关闭无副作用；修改配置统一跳转项目设置。

## 3. 排队与运行

`I18_D4_REWRITE_RUNNING` 承载 queued/running 文案变体。页面显示固定来源、ReviewReport、选中 Issue 数、Run、工作流和安全进度。关闭状态条不取消任务；刷新后通过 ContentRewriteSummary、Run 和 Event 恢复。

## 4. 成功候选

Runtime 成功 → 严格校验 `rewrite.output.v1` → 单一事务创建非当前候选和全部 RewriteSourceIssueLink → 写 `result_consumed` → Summary 为 `candidate_ready`。

`I18_D5_REWRITE_RESULT` 展示 source→candidate 关系、修改摘要、issueResolutions 和候选只读正文。用户可以比较版本或打开只读预览；唯一提升操作是“设为当前版本”。

`I18_D5_SET_CURRENT_CONFIRM` 明确说明旧版本保留、原审核仍绑定源版本、不会自动审核。确认后复用 Iteration 16 CAS；stale 时不强制覆盖。

## 5. 失败与恢复

| 场景 | 数据结果 | UI/恢复 |
|---|---|---|
| Runtime failed/cancelled | 零候选、零 IssueLink | `I18_D4_REWRITE_FAILED`，Runtime Retry |
| 输出校验失败 | succeeded Run，零候选、零 IssueLink | 同一 Frame 的输出校验变体，Runtime Retry |
| 结果消费失败 | succeeded Run，零候选、零 IssueLink | `I18_D5_RESULT_CONSUMPTION_FAILED`，仅消费 Retry |
| 未配置/配置失效 | 无新 Run | `I18_D4_REWRITE_AVAILABILITY`，跳转配置 |
| 页面刷新 | 保留全部持久化事实 | 重新读取 Summary，不产生写入 |

失败页面只展示安全错误、Correlation ID 和执行详情入口，不展示原始 JSON、SQL、堆栈、内部节点或凭据。

## 6. 无副作用规则

取消、关闭抽屉、返回审核报告、关闭运行提示、查看配置和刷新均不创建 Run、候选或 IssueLink。只有二次确认后的创建命令、明确的 Retry 和设为当前命令会写入。
