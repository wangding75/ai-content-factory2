# Iteration 17 — 真实内容审核 Closed Loop

## 1. 入口：正文编辑器

`I17_D1_EDITOR_REVIEW_ENTRY` 复用 Iteration 16 编辑器。页面显示当前已保存版本、保存状态和审核配置状态。

- 有未保存修改：提交审核禁用，先保存草稿；
- 无可审核版本：不打开抽屉；
- 配置缺失：可打开审核页查看原因和项目设置入口；
- 条件满足：点击“提交审核”打开抽屉，绝不直接创建 Run。

## 2. 发起审核抽屉

`D2_SUBMIT_REVIEW_DRAWER`：

1. 选择已保存的固定 ContentVersion；
2. 展示项目已绑定的 review 工作流、连接和审核维度；
3. 可填写 2,000 字以内审核说明；
4. 运行 Preflight；
5. 取消、关闭和 ESC 均无副作用；
6. 确认时携带 Token 与 Idempotency-Key 创建 Run。

工作流和审核维度只读，不允许临时切换。

## 3. 排队与运行

`STATE_TASK_RUNNING_BAR` 同时承载 queued/running 文案变体。页面显示固定版本、Run ID、工作流、启动时间和阶段摘要。用户可以离开页面；刷新后通过 `ContentReviewSummary`、Run 和 Event 恢复。

运行期间正文页面不被覆盖，审核不修改正文。

## 4. 成功报告

`D2_REVIEW_V2` 展示同一 Run 的 ReviewReport：

- conclusion、摘要和严重/警告/建议/通过规则统计；
- Issue 列表、分类、级别、标题和位置；
- 详情中的说明、原文证据、判断依据和建议；
- Report ID、sourceContentVersionId、workflowRunId；
- 单项或批量标记忽略。

“创建重写任务”在 Iteration 17 禁用并标明由 Iteration 18 开放。

## 5. 全文定位

`I17_D2_REVIEW_ISSUE_DETAIL` 使用来源 ContentVersion 的只读快照，不读取当前编辑器正文代替历史版本。按 Issue location 高亮证据，右侧显示结构化详情。

“在编辑器中打开”只能导航到正文编辑器；必须提示当前实时内容可能与被审核版本不同。不得把历史快照改成可编辑状态。

## 6. 失败与恢复

`STATE_TASK_FAILED_NOTICE` 根据 Summary 状态显示：

- `runtime_failed`：安全错误 + Runtime Retry；
- `output_validation_failed`：输出结构错误 + Runtime Retry；
- `result_consumption_failed`：领域提交失败 + 仅消费 Retry。

业务页面只展示安全错误码、Correlation ID、尝试次数和时间。不得显示原始 JSON、节点 ID、SQL、堆栈、内部地址或上游响应。

失败保持零 Report/Issue，正文和 current_version_id 不变。

## 7. 未配置

`STATE_NOT_CONFIGURED_EMPTY` 区分：

- 从未绑定 review 工作流；
- Binding 存在但 Configuration/Connection 失效；
- 配置可用但尚无审核记录。

前两种阻止创建 Run并跳转项目工作流设置。刷新配置只做查询，不创建 Run。

## 8. 审核历史

`I17_D2_REVIEW_HISTORY` 按 Run 创建时间倒序展示：

- queued/running：查看进度；
- failed：查看安全失败详情；
- succeeded + Report：查看报告；
- 同一版本多次审核均保留；
- 不同版本报告可切换比较，但不覆盖“当前报告”。

## 9. P0 与 Iteration 18 边界

P0 Mock Review 列表和详情继续可读，provider/source 清晰标识。真实审核不得改写历史 Mock 报告。Iteration 18 才能基于明确 Report/Issue 创建重写 Run。
