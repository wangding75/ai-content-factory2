# CF-18-01A — Iteration 18 UI / API 契约追踪

**状态：`frozen_cf_18_01a`。** 9 个 Frame 均复用既有 AppShell、Review Route 或 Rewrite Route；不得创建截图专用路由。

## 1. Frame 追踪

| Frame | 页面或状态入口 | 使用 API | 关键请求字段 | 响应状态 | 主要按钮行为 | 禁用/后续边界 |
|---|---|---|---|---|---|---|
| `I18_D2_REVIEW_REWRITE_ENTRY` | `/review?reportId={reviewId}` 的已完成 Report | `getReview`, `getContentRewriteAvailability` | `reviewId` | availability 200；`idle/not_configured` | 勾选 1～50 个 open Issue；“创建重写任务”导航创建页 | ignored/跨 Report/空选择禁用；不创建 Run，不改 Issue |
| `I18_D4_REWRITE_AVAILABILITY` | Rewrite Route 初始、未配置或配置失效 | `getContentRewriteAvailability`, `getContentRewriteSummary` | `reviewId` | 200；`idle/not_configured`，安全 reason | 配置项目工作流、前往全局设置、修复配置、刷新查询 | 卡片互斥；已有 Run/失败/Candidate 优先；刷新无写入 |
| `I18_D4_CREATE_REWRITE` | 从 Report 携带 selected Issue 进入 | `preflightContentRewrite`, `createContentRewriteRun` | Preflight: `selectedIssueIds/optionalInstructions/rewriteOptions`; Create: `preflightToken` + `Idempotency-Key` | Preflight 200 passed/blocked；Create 201/200；422/409 | 运行 Preflight；通过后二次确认创建 queued Run；返回审核结果 | 客户端不可提交 Issue 内容、来源、Stage、subject 或配置快照；禁止空选择 |
| `I18_D4_REWRITE_CONFIG_DRAWER` | 创建页“查看项目配置”抽屉 | `getContentRewriteAvailability`，复用 Preflight `configurationSummary` | `reviewId` | 200 available/reason | 关闭抽屉；前往项目设置 | 只读，不允许临时切换工作流/连接；关闭无副作用 |
| `I18_D4_REWRITE_RUNNING` | Summary 为 `queued/running` | `getContentRewriteSummary`, `getWorkflowRunDetail`, `listWorkflowRunEvents`, `cancelWorkflowRun` | `reviewId/runId`; Cancel: `expectedVersion` + `Idempotency-Key` | 200；Cancel 200/409 | 查看执行详情；明确取消 queued/running Run；关闭状态条 | 关闭不取消；配置失效不遮蔽 active Run；不轮询写入 |
| `I18_D4_REWRITE_FAILED` | Summary 为 `runtime_failed/output_validation_failed` | `getContentRewriteSummary`, `getWorkflowRunDetail`, `retryWorkflowRun` | Retry: `expectedVersion`, `useCurrentConfiguration=false` + `Idempotency-Key`; 无 `inputOverride` | Summary 200；Retry 201/200；409 | 查看安全详情；重新执行 | result_consumption_failed 不得走 Runtime Retry；不展示原始输出/内部节点 |
| `I18_D5_RESULT_CONSUMPTION_FAILED` | Summary 为 `result_consumption_failed` | `getContentRewriteSummary`, `getWorkflowRunDetail`, `retryContentRewriteResultConsumption` | `expectedRunVersion` + `Idempotency-Key` | Summary 200；Retry 200；409/500 | “重试提交”仅重新消费持久化输出；查看详情 | 不调用 Runtime/n8n，不建新 Run；Candidate/比较/编辑/Set Current 禁用 |
| `I18_D5_REWRITE_RESULT` | Summary 为 `candidate_ready` | `getContentRewriteSummary`, `getContentRewriteResult`, `getContentVersion` | `reviewId/workflowRunId/versionId` | 200；Candidate ready | 查看 source→candidate、addressed/unresolved/warnings/metadata；对比；打开只读预览；打开 Set Current 确认 | Candidate 非当前时不可写；无自动审核/发布；Mock 不伪造来源 |
| `I18_D5_SET_CURRENT_CONFIRM` | 结果页“设为当前版本”弹窗 | `setCurrentContentVersion` | `candidateVersionId`, `expectedCurrentVersionId`, `expectedCurrentVersion` + `Idempotency-Key` | 200 成功/回放/已是当前；404/409/422 | 取消无副作用；确认执行独立 CAS | CAS 冲突不强制覆盖；不调用 Runtime、不建 Candidate、不改 Report/Issue；成功后 Summary 仍 candidate_ready |

## 2. 强制追踪规则

1. `workId=ContentItem.id`；Run 固定 `stage=rewrite`、`subjectType=review_report`、`subjectId=reviewId`。
2. Rewrite 默认使用且固定 ReviewReport 的 source ContentVersion；运行中 currentVersion 漂移不改变输入。
3. selected Issue 只允许同 Report 的 1～50 个 `open` Issue，客户端只提交唯一 ID。
4. Runtime 使用 `RewriteRuntimeInputV1`（`rewrite.input.v1`）与 `RewriteRuntimeOutputV1`（`rewrite.output.v1`）。
5. `candidate_ready` 必须存在同 Run 已完整原子持久化的 Candidate；succeeded 无 Candidate 不得 ready。
6. `result_consumption_failed` 只能调用专用消费 Retry；Runtime Retry 禁止 inputOverride。
7. Set Current 复用 Iteration 16 CAS；成功后 `candidateIsCurrent=true/canSetCurrent=false`，不新增 Summary 状态。
8. 失败 UI 只显示安全摘要，不展示 SQL、连接、堆栈、n8n 节点、Webhook、凭据或原始上游响应。
9. 所有 Frame 的关闭、返回、刷新、查看配置/详情均无业务写入；只有 Create、Cancel/Retry 与 Set Current 是命令。
