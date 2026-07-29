# Iteration 17 — 真实内容审核验收标准

## A. 契约与来源

- [ ] Runtime Stage 使用 `review`，与 Iteration 13 Binding 一致；不存在 `content_review` 平行 Stage。
- [ ] 所有真实审核 Run 固定 `subjectType=content_version`、`subjectId=sourceContentVersionId`。
- [ ] Preflight Token 绑定 actor、来源版本 ID/version/hash、审核说明、维度和配置版本。
- [ ] 创建后来源 ContentVersion 不再原地修改；审核不改变正文内容。
- [ ] 同一固定版本最多一个 active review Run，不同版本可并行。

## B. 预检、幂等与异常

- [ ] Preflight 无副作用，不创建 Run/Event/Report/Issue，也不调用 n8n。
- [ ] 未保存正文、版本漂移、配置漂移、active Run 冲突均阻止创建。
- [ ] 相同 Idempotency-Key/相同请求回放首次 Run；同键异请求冲突。
- [ ] 基础设施错误不得伪装为 not_configured。
- [ ] 失败不消费错误 Token，不留下孤立 Event 或成功幂等记录。

## C. Runtime 与领域消费

- [ ] 真实 n8n 接收 `review.input.v1` 并返回严格 `review.output.v1`。
- [ ] Run 状态为 queued → running → succeeded|failed|cancelled。
- [ ] succeeded 只有在 Report/Issue 原子提交后才表现为 `review_ready`。
- [ ] 未知字段、尾随 JSON、非法 severity/location/evidence、重复 issueKey 均进入 `output_validation_failed`，零 Report/Issue。
- [ ] Report/Issue 事务失败进入 `result_consumption_failed`，零部分数据。
- [ ] Runtime Retry 创建新 Run；结果消费 Retry 不调用 n8n。
- [ ] 一个 Run 最多一个 ReviewReport，重复消费返回既有结果。

## D. 数据模型与兼容

- [ ] ReviewReport 关联唯一成功 WorkflowRun 和固定 ContentVersion。
- [ ] ReviewIssue 复用/扩展 `review_findings`，未创建平行 `review_issues` 表。
- [ ] Issue 顺序、issueKey、证据、位置、建议和 disposition 可追踪。
- [ ] 标记忽略不修改 Report 原始结论、统计、正文或 Run。
- [ ] P0 Mock Report/Finding/Recommendation 数据和 API 无回归。

## E. Summary 与 UI

- [ ] 支持 `idle/not_configured/queued/running/review_ready/runtime_failed/output_validation_failed/result_consumption_failed` 八状态。
- [ ] 页面刷新后状态、错误、Run 和 Report 从持久化数据恢复。
- [ ] 配置后续失效不隐藏已有 Run、失败或 Report。
- [ ] 抽屉只能选择已保存版本，工作流和维度只读。
- [ ] 运行中可安全离开页面；正文和未保存编辑状态不被轮询覆盖。
- [ ] 报告页显示统计、Issue、证据、定位、建议和来源关系。
- [ ] 全文定位读取被审核版本快照，不以当前正文替换。
- [ ] 历史页展示运行中、成功、失败和多版本记录。
- [ ] Iteration 17 的“创建重写任务”不可执行，并明确 Iteration 18 开放。

## F. 安全

- [ ] UI、Event、日志和 Run 快照不暴露密钥、Token、Authorization、Cookie、SQL、堆栈、内部路径、节点 ID、内部 URL 或原始上游响应。
- [ ] 错误页只展示安全错误码、Correlation ID、尝试次数、时间和可执行恢复动作。
- [ ] 所有查询和命令校验 Project/ContentItem/ContentVersion/Report/Issue 归属，禁止跨项目访问。

## G. 工程门禁

- [ ] OpenAPI、Migration、后端、前端、n8n、E2E 和安全验证通过。
- [ ] 8 张原型 screen/code 存在，AppShell 和中文术语一致。
- [ ] 先精确测试、再分组、最后总门禁；未定位根因前不反复完整重跑。
- [ ] 独立 Code Review、变更报告、测试报告、Git diff/status/Push 证据完整。
