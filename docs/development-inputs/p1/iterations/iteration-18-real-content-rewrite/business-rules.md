# Iteration 18 — 真实正文重写业务规则与状态机

**状态：`rebuild_candidate_2026_07_29`。** 本文件用于替换旧的简略规划，作为 CF-18-01 契约冻结的业务输入；正式 HTTP 字段仍以 CF-18-01 更新后的 OpenAPI 为唯一来源。

## 1. 目标与边界

- 用户从一个明确、已完成的 ReviewReport 中选择需要处理的 `open` ReviewIssue，基于报告绑定的不可变 ContentVersion 创建真实异步重写任务。
- Runtime Stage 固定为 `content_rewrite`；输入契约候选为 `rewrite.input.v1`，输出契约候选为 `rewrite.output.v1`。
- 成功结果只创建一个新的非当前候选 ContentVersion。来源版本、ReviewReport、ReviewIssue 和 WorkflowRun 全链路可追踪。
- 本迭代不覆盖来源正文、不自动设为当前版本、不自动重新审核、不自动发布、不删除历史版本。
- P0 Mock Rewrite 保持兼容，但不得伪装为真实 n8n 重写证据。

## 2. 入口与 Issue 选择

- 入口位于 Iteration 17 的审核报告页。只有存在明确 ReviewReport 时才可创建重写。
- 可选 Issue 必须属于该 Report、处置为 `open`、ID 唯一；`ignored` Issue 不可选择。
- 至少选择 1 项，最多选择 50 项。选择只存在于创建流程中，不改变 ReviewIssue disposition。
- Report 的来源 ContentVersion 是重写基线；即使 ContentItem 当前版本后来变化，Run 和候选仍绑定原来源版本。
- 原型中的“待解决/进行中/待处理”不引入新持久化状态，生产只使用 `open/ignored`。

## 3. 预检与创建

预检是只读操作，不创建 Run/Event/ContentVersion/IssueLink/幂等记录，不调用 n8n。至少校验：

- Project、ContentItem、ReviewReport、来源 ContentVersion 和 selected Issue 的归属；
- Report 已完成且来源版本固定；
- selected Issue 全部为 `open`，版本和内容未漂移；
- `content_rewrite` Binding、Workflow Configuration 和 Connection 可用；
- Configuration 声明 `rewrite.input.v1` / `rewrite.output.v1`；
- 同一 ReviewReport 没有 queued/running 的 active rewrite Run；
- rewriteStrategy 合法，optionalInstructions 不超过 2,000 个 Unicode 字符。

通过时签发 10 分钟有效的 Preflight Token，绑定 actor、projectId、contentItemId、reviewReportId、sourceContentVersionId/version/hash、selected Issue ID/version/disposition 集合、rewriteStrategy、optionalInstructions 摘要、Binding/Configuration/Connection 版本、input digest、nonce 和 expiresAt。

创建命令必须使用 `Idempotency-Key`。服务端固定：

- `stage=content_rewrite`；
- `triggerSource=manual`；
- `subjectType=review_report`；
- `subjectId=reviewReportId`。

客户端不能覆盖 Stage、subject、工作流、Connection、来源版本或问题快照。创建时在一致性事务内重新校验 Token、Report、Issue 集合、来源版本、配置与 active Run。相同键同请求回放首次 Run；相同键不同请求返回幂等冲突。

## 4. Runtime 输入

`rewrite.input.v1` 的候选结构必须由服务端构建，至少包含：

- schemaVersion、projectId、contentItemId、reviewReportId；
- sourceContentVersionId、sourceContentVersionVersion、sourceContentHash、sourceTitle、sourceContent；
- selectedIssues：issueId、issueKey、category、severity、title、description、evidence、location、suggestion；
- globalConstraints：项目/章节只读约束快照；
- rewriteStrategy：`targeted_fix` 或 `creative_rewrite`；
- optionalInstructions；
- workflowRunId、correlationId。

Run/Event/日志不得记录 Preflight Token、原始 Idempotency-Key、凭据、Authorization、Cookie、内部 URL 或未过滤的上游响应。

## 5. Runtime 输出

`rewrite.output.v1` 必须是严格 JSON，拒绝未知字段和尾随 JSON。候选最小结构：

```json
{
  "schemaVersion": "rewrite.output.v1",
  "title": "第12章 雨夜访客",
  "content": "重写后的完整正文",
  "summary": "本次重写摘要",
  "wordCount": 3200,
  "changeSummary": "修复逻辑冲突并压缩转场",
  "issueResolutions": [
    {
      "issueKey": "logic-conflict-1",
      "status": "addressed",
      "summary": "统一人物背景设定"
    }
  ]
}
```

规则：

- title、content、summary、wordCount、changeSummary 必填；content 非空；wordCount 必须与服务端计算结果一致或由服务端重算后校验；
- issueResolutions 必须且只能引用输入 selectedIssues，issueKey 唯一，并完整覆盖全部选中 Issue；
- resolution status 仅允许 `addressed/partially_addressed/not_addressed`；
- 不接受候选 ID、versionNo、current 标志、ReviewIssue disposition、数据库字段、凭据或内部信息；
- 数量、字符串长度和正文上限在 CF-18-01 由 OpenAPI/Schema 冻结；
- 输出不合法时零候选、零 IssueLink，Summary 为 `output_validation_failed`。

## 6. 候选、来源与 IssueLink

- 合法输出在单一事务内创建 `source=workflow_rewrite`、`status=editable_draft` 的新 ContentVersion。
- 候选保存 sourceContentVersionId/sourceContentVersionVersion/sourceWorkflowRunId；一个 Run 最多创建一个候选。
- 同事务创建 RewriteSourceIssueLink，关联候选、Run、ReviewReport 和每个选中 ReviewIssue；链接数量必须等于选中 Issue 数量。
- 候选默认不是 current；创建后 ContentItem.currentVersionId、status 和 reviewedAt 不变。
- 候选在设为当前前只读预览；“打开编辑器”不得把非当前候选当作当前可写草稿。
- 原 Report 和 Issue 不被修改；issueResolutions 是重写输出说明，不等于自动将 Issue 标记为 resolved。

## 7. Summary 状态机

| State | 持久化依据 | UI 行为 |
|---|---|---|
| `idle` | 配置可用且无需要展示的 Run/候选 | 可预检和创建 |
| `not_configured` | 无可恢复执行事实，且 Binding/配置/连接不可用 | 引导项目或全局设置 |
| `queued` | 最新 active rewrite Run 为 queued | 显示排队，可离开页面 |
| `running` | 最新 active rewrite Run 为 running | 显示运行信息和详情 |
| `candidate_ready` | 同 Run 的候选与全部 IssueLink 已成功消费 | 展示候选、对比和设为当前 |
| `runtime_failed` | Run failed/cancelled | Runtime Retry 创建新 Run |
| `output_validation_failed` | succeeded Run 输出不符合 `rewrite.output.v1` | Runtime Retry 创建新 Run，零候选 |
| `result_consumption_failed` | 输出合法但候选/IssueLink 事务失败 | 仅结果消费 Retry，不外呼 |

`WorkflowRun.succeeded` 不等于 `candidate_ready`。配置后来失效时，已有 active、failure 或 candidate 事实优先于 `not_configured`。

## 8. Retry

- Runtime Retry：仅适用于 failed、cancelled、output_validation_failed；创建新 Run、继承冻结输入，禁止 inputOverride。
- 结果消费 Retry：仅适用于 succeeded、存在 result_consumption_failed、无候选的 Run；只消费已持久化输出，绝不调用 n8n。
- 同一 Run 重复消费返回原候选，不创建第二个版本或第二组 IssueLink。

## 9. 设为当前版本

- 复用 Iteration 16 的候选设为当前命令与 CAS 语义。
- 请求必须带 candidateVersionId、expectedCurrentVersionId、expectedCurrentVersion 和 Idempotency-Key。
- 只有实际当前版本仍等于候选的 sourceContentVersionId/version 时才可提升；否则返回 candidate_source_stale，不提供强制覆盖。
- 成功只切换 currentVersionId，并将 ContentItem 置为 draft、reviewedAt 清空；旧版本、ReviewReport、Issue、Run 和 IssueLink 全部保留。
- 设为当前后不自动触发重新审核。

## 10. 相邻迭代边界

- Iteration 16 提供不可变版本、候选创建与设为当前基础能力。
- Iteration 17 提供真实 ReviewReport、ReviewIssue 和固定来源版本；不创建重写版本。
- Iteration 18 只基于明确 Report/Issue 生成新候选。
- Iteration 19 才负责第二闭环的最终集成验收，不在本迭代扩展多实例路由、费用或工作流编辑器。
