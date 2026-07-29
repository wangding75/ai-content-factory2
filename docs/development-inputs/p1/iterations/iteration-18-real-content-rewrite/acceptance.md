# CF-18-01A — Iteration 18 业务与 API 契约冻结验收

**状态：`frozen_cf_18_01a`。** 本验收仅覆盖业务规则、API、输入输出、状态机、错误语义与 9 Frame UI 契约追踪。

| 范围 | PASS 条件 |
|---|---|
| 闭环 | Report/Issue → Availability → 选择 → 配置 → Preflight → 二次确认 → rewrite Run → 严格输出 → 原子 Candidate → Set Current/CAS 唯一链路完整 |
| Stage/Subject | `rewrite` + `review_report/reviewId` 唯一语义；无平行创建接口 |
| 来源版本 | Report 来源、Run 固定来源、ContentItem current 三者明确；运行中 current 漂移不改变输入 |
| Issue | 同 Report 的 1～50 个 `open` Issue；重复/空选择拒绝；`ignored` 不可选；不改 disposition |
| 输入 | `rewrite.input.v1` 字段顺序、服务端保护、Report/Issue 安全快照、长度与禁止字段完整 |
| 输出 | `rewrite.output.v1` 必填/nullable/数量/长度、Issue 精确分区、未知字段/尾随 JSON/内部字段拒绝完整 |
| API | Availability、Preflight、Create、Summary、History、Result、Consumption Retry 完整；复用 Set Current、Run Detail/Event/Retry/Cancel 与 ContentVersion Detail |
| 命令 | Idempotency-Key 完整；创建/Runtime Retry 首次 201、回放 200；消费 Retry/Set Current 成功与回放 200 |
| 状态 | 仅 `idle/not_configured/queued/running/candidate_ready/runtime_failed/output_validation_failed/result_consumption_failed`；优先级与刷新恢复明确 |
| 消费 | 输出校验失败零 Candidate；消费失败零 Candidate/部分关联；succeeded 不等于 ready |
| Retry | Runtime Retry 与仅消费 Retry 严格分离；rewrite 禁止 inputOverride/current configuration |
| Set Current | Candidate、expected current ID/version、CAS、已是当前、同 Key 异请求与成功后 Summary 语义唯一 |
| 错误 | 17 个 Rewrite 错误码去重；400/404/409/422/500 边界与安全 ErrorEnvelope 完整 |
| 兼容 | 复用 Iteration 16 Candidate/Set Current；Iteration 17 Report/Issue/八状态与 open/ignored 不变；Mock 不伪造真实来源 |
| UI | 9 个 Frame 均含入口、API、请求字段、响应状态、按钮行为与禁用/后续边界 |
| 工程 | OpenAPI、Iteration 17、Iteration 18 三脚本与 `git diff --check` 通过 |
| 保护 | Migration 前后为 17；代码、Migration、n8n、原型、Iteration 15～17 未修改 |

## 不在 CF-18-01A 范围

- 数据模型、事务、锁、索引与 Migration 决策或实现；
- 后端、前端、n8n 开发；
- Docker 应用栈、浏览器、视觉或真实 API 联调；
- 自动审核/重写循环、自动 Set Current 或发布；
- CF-18-01B 及后续开发任务。

不得因本次契约冻结扩大后续验收范围。
