# CF-18-01A — Iteration 18 真实正文重写业务规则与状态机

**状态：`frozen_cf_18_01a`。** 本文件冻结业务、API、输入输出、状态机、错误语义与 UI 契约。数据模型、事务、锁、索引和 Migration 由 CF-18-01B 冻结；本任务不对其作实现决定。

## 1. 唯一业务闭环

```text
ReviewReport / ReviewIssue
→ Rewrite Availability
→ 选择需要处理的 Issue
→ 配置重写要求
→ Rewrite Preflight
→ 二次确认
→ 创建 stage=rewrite 的 WorkflowRun
→ Runtime 异步执行
→ 严格校验 rewrite.output.v1
→ 原子产生一个非当前 Rewrite Candidate
→ 查看来源与候选差异
→ 用户执行 Set Current
→ CAS 更新当前版本
```

- Rewrite 不等于 Review，不修改原 ReviewReport 或 ReviewIssue，不覆盖来源 ContentVersion。
- Runtime `succeeded` 只表示外部执行成功，不等于 Candidate ready。
- Candidate 创建后不自动成为当前版本；Set Current 是独立、幂等的 CAS 命令。
- 不自动把 Issue 标记为 resolved/fixed，不引入此类 Iteration 17 未冻结状态。
- 不自动循环审核或重写，不自动发布。

## 2. 版本与来源关系

必须同时区分：

1. ReviewReport 审核的来源 ContentVersion；
2. Rewrite Run 创建时固定的来源 ContentVersion；
3. ContentItem 当前版本。

Rewrite 默认且只能使用 ReviewReport 的 `sourceContentVersionId`。Preflight 固定来源 ID、乐观锁版本与 SHA-256 内容摘要；Run 创建后来源不可漂移。ReviewReport、selectedIssues、Run 与 Candidate 均可追踪至该固定来源。

Runtime 期间 `currentVersion` 变化不改变 Run 输入，也不阻止 Candidate 创建。Candidate 生成不更新 `currentVersion`。Set Current 必须提交 `candidateVersionId`、`expectedCurrentVersionId` 与兼容 Iteration 16 的 `expectedCurrentVersion`；CAS 不匹配返回 409 `content_version_conflict`，不得静默覆盖。

Set Current 成功后旧版本、来源 Report/Issue 和 Run 均保留；ContentItem 置 `draft` 并清空 `reviewedAt`。Summary 仍返回 `candidate_ready`，同时 `candidateIsCurrent=true`、`canSetCurrent=false`。相同成功命令可幂等回放；Candidate 已是当前版本时返回 200 no-op，不重复修改。

## 3. Issue 选择

- Rewrite 必须绑定一个已完成的 ReviewReport。
- `selectedIssueIds` 必须属于该 Report，处置必须为 `open`；`ignored` Issue 不可选择。
- 每次必须选择 1～50 个 Issue；空选择禁止，不存在“整个报告重写”的第二种语义。
- 客户端只提交 Issue ID，不提交 Issue 内容、证据、位置或处置。
- ID 必须唯一；重复 ID 请求返回 422 `rewrite_not_available`。服务端按 `position ASC, issueId ASC` 形成稳定顺序。
- Preflight 和 Run 快照记录 Issue ID、version、disposition 与安全内容。创建时任一 Issue 漂移返回 409 `rewrite_preflight_stale`。
- Candidate 成功与 Set Current 均不改变 Issue disposition；Iteration 17 仍只有 `open/ignored`。

## 4. Availability、Preflight 与创建

### 4.1 Availability

`GET /api/v1/reviews/{reviewId}/rewrite-availability` 是只读查询，不创建 Run、Candidate、Token 或幂等记录，不外呼。它区分 Report 未完成、无 open Issue、未配置与 active Run 冲突。已有 Run、失败或 Candidate 不因后续配置失效而消失。

### 4.2 Preflight

`POST /api/v1/reviews/{reviewId}/rewrites/preflight` 只接受：

- `selectedIssueIds`；
- `optionalInstructions`；
- `rewriteOptions`。

它校验 Project/ContentItem/Report/source ContentVersion/Issue 归属与快照、`rewrite` Binding、Configuration、Connection、`rewrite.input.v1`/`rewrite.output.v1` 声明，以及同一 Report 无 queued/running Rewrite Run。通过后签发 10 分钟、单次使用、绑定 actor 与全部输入/配置摘要的 Token。Preflight 无副作用，不调用 Runtime/n8n。

`optionalInstructions` 可为 null，空字符串规范化为 null，最长 2,000 个 Unicode 字符。`rewriteOptions` 只有 `strategy=targeted_fix|creative_rewrite`，拒绝未知字段。

### 4.3 创建

`POST /api/v1/reviews/{reviewId}/rewrites` 必须使用 `Idempotency-Key`，请求体只有 `preflightToken`。服务端固定：

- `stage=rewrite`；
- `triggerSource=manual`；
- `subjectType=review_report`；
- `subjectId=reviewId`；
- `inputContract=rewrite.input.v1`；
- `outputContract=rewrite.output.v1`。

来源 ContentVersion、Report、selectedIssues、Binding、Configuration 与 Connection Snapshot 均为服务端保护字段。首次创建返回 201；同 Key 同请求返回原 Run 和 200；同 Key 异请求返回 409 `idempotency_conflict`。Token 过期为 422 `rewrite_preflight_expired`；漂移、已消费或 active Run 分别为明确 409。

## 5. `rewrite.input.v1`

服务端按以下固定顺序构建：

1. `schemaVersion`
2. `workflowRunId`
3. `correlationId`
4. `projectId`
5. `contentItemId`
6. `sourceContentVersionId`
7. `sourceContentVersionVersion`
8. `sourceContentHash`
9. `sourceTitle`
10. `sourceContent`
11. `reviewReportId`
12. `reportSnapshot`
13. `selectedIssues`
14. `optionalInstructions`
15. `rewriteOptions`

`reportSnapshot` 只含 Report ID、固定来源 ID/version/hash、conclusion、summary、completedAt。`selectedIssues` 是 1～50 个完整安全快照，包含 Issue ID/Report ID/key/position/version/category/severity/title/description/evidence/location/suggestion 与固定 `disposition=open`。

`sourceTitle` 1～120 字符，`sourceContent` 1～200,000 字符，`correlationId` 1～128 字符。客户端不能提交或覆盖任何保护字段。输入、Run、Event 和日志严禁凭据、Token、Authorization、Cookie、Webhook、数据库信息、内部 URL、原始上游响应、Preflight Token 或原始 Idempotency-Key。

## 6. `rewrite.output.v1`

唯一允许结构：

```json
{
  "schemaVersion": "rewrite.output.v1",
  "title": "第12章 雨夜访客",
  "content": "重写后的完整正文。",
  "summary": "修复逻辑冲突并压缩转场。",
  "addressedIssues": [
    {
      "reviewIssueId": "66666666-6666-4666-8666-666666666666",
      "summary": "统一人物背景设定。"
    }
  ],
  "unresolvedIssues": [],
  "warnings": [],
  "metadata": {
    "changeSummary": "保留原叙事视角。"
  }
}
```

- 八个顶层字段均必填；`metadata` 可为 null。
- `title` 1～120、`content` 1～200,000、`summary` 1～5,000 字符。
- addressed/unresolved 各最多 50 项；每项 summary 1～2,000 字符。
- warnings 最多 50 项，每项 1～1,000 字符。
- metadata 只允许 nullable `changeSummary`（最多 2,000 字符），禁止其他字段。
- addressed/unresolved 只能引用 selectedIssueIds，跨两数组不得重复，并必须恰好覆盖全部 selectedIssueIds。
- 严格解析单一 JSON 对象：未知字段、尾随 JSON、非法枚举/长度、重复 ID、候选 ID/current 标记、凭据、Token、Cookie、Webhook、数据库、SQL、堆栈、内部节点/URL均拒绝。
- wordCount 由服务端根据 content 计算，不接受 Provider 注入。
- 校验失败为 `output_validation_failed`，零 Candidate；消费失败整笔回滚，零 Candidate 和零部分关联数据。

## 7. Candidate 与兼容

Rewrite Candidate 复用现有 ContentVersion/Candidate 体系，来源标识为 `workflow_rewrite`，保存固定来源 ContentVersion 与 WorkflowRun 关系；不建立第二套 Candidate 系统。一个成功 Run 最多一个 Candidate。Candidate 创建时非当前、只读预览；用户明确 Set Current 后才进入当前编辑语义。

Iteration 16 的 Candidate、Set Current 与 CAS 保持兼容。Iteration 17 的 Report、Issue、八状态及 `open/ignored` 不变。generation、review 与 WorkflowRun 既有生命周期不变。P0 `mock_rewrite` 与旧 Candidate 保持可读，但不得伪造 ReviewReport、selectedIssues 或真实 Rewrite Run 来源。

## 8. Summary 八状态与优先级

| State | 唯一持久化依据 |
|---|---|
| `idle` | 配置可用且无须展示的 Run/Candidate |
| `not_configured` | 无可恢复 Run/失败/Candidate，且 rewrite 配置不可用 |
| `queued` | 最新 active Rewrite Run 为 queued |
| `running` | 最新 active Rewrite Run 为 running |
| `candidate_ready` | 同一 succeeded Run 的 Candidate 已完整原子持久化 |
| `runtime_failed` | Run failed 或 cancelled |
| `output_validation_failed` | succeeded Run 的输出未通过 `rewrite.output.v1` |
| `result_consumption_failed` | 输出合法但 Candidate 原子消费失败 |

优先级：active Run → 已消费 Candidate → 最新失败事实 → not_configured/idle。`succeeded` 且无 Candidate 不得返回 `candidate_ready`；校验失败与消费失败必须区分。刷新只依赖持久化事实恢复，不新增 Summary 表。`latestError` 只含安全 code/message/correlationId/occurredAt。

Set Current 后仍是 `candidate_ready`，以 `candidateIsCurrent=true` 明确已提升，不引入第九个状态。

## 9. Retry

### Runtime Retry

复用 `POST /api/v1/workflow-runs/{runId}/retries`。rewrite Run 仅允许 `failed`、`cancelled`、`output_validation_failed`；`result_consumption_failed` 禁止调用 Runtime。请求必须带 `expectedVersion` 与 `Idempotency-Key`，`useCurrentConfiguration` 必须缺省或 false，禁止 `inputOverride`。新 Run 固定继承来源 ContentVersion、Report、Issue、Binding/Configuration/Connection Snapshot，并记录 `retryOfRunId`。首次 201，幂等回放 200。

### Result Consumption Retry

`POST /api/v1/workflow-runs/{workflowRunId}/rewrite-result-consumption-retries` 仅允许 succeeded + `result_consumption_failed`，请求含 `expectedRunVersion` 与 `Idempotency-Key`。它不调用 Runtime/n8n、不创建新 Run，重用原持久化输出。Candidate 已存在时返回已有结果；成功后 Summary 恢复 `candidate_ready`。成功与回放均返回 200。

## 10. Set Current

复用 `POST /api/v1/content-items/{contentItemId}/current-version`，请求包含 `candidateVersionId`、`expectedCurrentVersionId`、`expectedCurrentVersion` 和 `Idempotency-Key`。

- Candidate 不存在或不属于该 ContentItem：404 `rewrite_candidate_not_found`；
- Candidate 输出尚未完整消费：409 `rewrite_candidate_not_ready`；
- Candidate 已是当前：200 no-op；
- CAS/来源基线冲突：409 `content_version_conflict`；
- 同 Key 异请求：409 `idempotency_conflict`。

Set Current 不重新调用 Runtime，不创建 Candidate，不修改来源版本、ReviewReport 或 ReviewIssue。

## 11. 错误语义

Rewrite 专用错误码唯一集合：

`review_not_found`、`review_issue_not_found`、`rewrite_not_available`、`rewrite_not_configured`、`rewrite_preflight_expired`、`rewrite_preflight_stale`、`rewrite_preflight_consumed`、`active_rewrite_run_conflict`、`rewrite_candidate_not_found`、`rewrite_candidate_not_ready`、`rewrite_output_validation_failed`、`rewrite_result_consumption_failed`、`content_version_conflict`、`idempotency_conflict`、`workflow_run_not_found`、`workflow_run_version_conflict`、`internal_error`。

- 400：JSON、UUID、Header 或基础请求格式错误；
- 422：业务输入非法、配置不可用或 Preflight 过期；
- 404：Report、Issue、Run 或 Candidate 不存在；
- 409：幂等、active Run、Preflight 漂移/消费、乐观锁、Run version 或 CAS 冲突；
- 500：安全 `internal_error` 或原子消费失败。

ErrorEnvelope 只返回安全 message/details/request_id，不泄漏 SQL、连接信息、堆栈、n8n 节点、Webhook、凭据或原始上游响应。

## 12. 正式 API 范围

新增唯一正式接口：Rewrite Availability、Rewrite Preflight、Create Rewrite Run、Rewrite Summary、Rewrite History、Rewrite Result/Candidate Detail、Result Consumption Retry。

复用：Set Current、WorkflowRun Detail、WorkflowRun Events、Runtime Retry、Cancel Run、ContentVersion 只读详情。不得创建平行接口或让业务页调用通用 Run 创建接口。
