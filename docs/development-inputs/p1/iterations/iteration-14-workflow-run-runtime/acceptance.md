# Iteration 14 — n8n 适配与 WorkflowRun 异步运行 — 待执行验收清单

**状态：未完成。** CF-14-01 仅冻结契约；下列 CF-14 后续验收项均不得因本任务标记完成。

- [ ] n8n Adapter 支持连接验证、工作流验证和异步执行，凭证不写入日志、快照或响应。
- [ ] WorkflowConnection 可以验证、启用和停用；仅验证成功的连接可启用，且最多一个连接启用。
- [ ] WorkflowConfiguration 可以验证、启用和停用；启用前绑定连接已验证且已启用。
- [ ] WorkflowRun 状态机正确覆盖 queued、running、succeeded、failed、cancelled 和相应事件。
- [ ] Worker 正确入队、领取、执行、恢复状态并处理超时与失败。
- [ ] 创建、取消和重试命令正确处理 Idempotency-Key 与并发控制。
- [ ] 运行保存不含密钥的绑定/配置快照，输入、输出和错误详情均通过安全校验。
- [ ] 错误码、错误消息、错误详情和上游响应均脱敏。
- [ ] 完整重试创建新的 WorkflowRun，原运行记录不可修改。
- [ ] 取消运行的状态与时间戳正确持久化和展示。
- [ ] 项目运行摘要正确提供总运行次数、运行中、最近失败、最近运行和最近运行列表。
- [ ] `/workflow-runs` 展示全局运行记录，并以同一列表接口支持 projectId 与 stage 筛选。
- [ ] `/workflow-runs/{runId}` 展示运行详情、事件、项目链接和规定的流程中心面包屑。
- [ ] P14_08 已绑定可运行时作为确认弹窗工作；创建成功后直接进入详情。
- [ ] P14_09 未绑定时作为提示弹窗工作，不是独立页面。
- [ ] P14_01～P14_10 人工 UI 基线状态为 PASS，开发实现复用既有产品壳层。
- [ ] 真实 API、PostgreSQL、Worker 与浏览器联调通过。
- [ ] 数据库验收确认当前 Iteration 最终状态正确。
- [ ] 不验证历史 Migration 回滚。
- [ ] 不建设 callback server，不接入 LLM Provider 或分发平台执行。
- [ ] API：实现并以真实服务验证连接/配置 verify、enable、disable，以及 WorkflowRun 创建、列表、详情、事件、重试、取消和项目摘要；响应符合冻结 OpenAPI。
- [ ] 数据库终态：仅验证当前 Iteration 最终表结构、快照脱敏、运行及事件记录正确；不要求历史 Migration 回滚。
- [ ] Worker：queued 运行被异步领取，n8n Webhook 响应、超时和输出校验均形成脱敏事件与正确终态。
- [ ] 状态机：仅允许 `queued`、`running`、`succeeded`、`failed`、`cancelled` 及冻结的迁移；终态不可再变更。
- [ ] 幂等与并发：所有命令验证 Idempotency-Key 和 expectedVersion；同键同载荷回放稳定，不同载荷或版本冲突返回统一错误。
- [ ] 错误处理：`code`、`message`、`details`、`requestId` 语义完整，且不泄露密钥、Authorization、原始上游响应、SQL 或堆栈。
- [ ] 重试与取消：重试创建新的 WorkflowRun 并保留旧记录；仅 queued/running 可取消并记录 cancelledAt。
- [ ] UI：P14_01～P14_10 按冻结路由、筛选和弹窗语义实现；不得新增项目级运行列表路由或流程中心 Tab。
- [ ] 浏览器验证：使用真实浏览器验证列表、详情、空态、错误恢复、确认/未绑定弹窗、取消和重试，无控制台错误或技术字段泄露。
- [ ] 真实联调：以真实 API、PostgreSQL、Worker、n8n 和浏览器完成成功、失败、取消、重试与项目摘要闭环。
