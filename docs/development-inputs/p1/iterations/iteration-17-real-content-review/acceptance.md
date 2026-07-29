# Iteration 17 — 真实内容审核验收标准

**状态：`frozen_cf_17_01`。** CF-17-01 只按以下冻结矩阵验收，不增加其他 PASS 条件。

| 范围 | PASS 条件 |
|---|---|
| Stage | 全部统一为 `review`，不存在平行 Review Stage |
| 来源 | Run、Report、Issue 固定绑定已保存 ContentVersion |
| 输入输出 | `review.input.v1` 和 `review.output.v1` 字段、必填性、可空性、枚举、数量与长度限制完整 |
| API | Preflight、Create、Summary、History、Detail、Issue、Consumption Retry 完整，并复用 WorkflowRun Detail/Event/Runtime Retry/Cancel |
| 状态 | `idle/not_configured/queued/running/review_ready/runtime_failed/output_validation_failed/result_consumption_failed` 八状态及优先级冻结 |
| 数据模型 | 复用 `review_reports/review_findings/review_recommendations`，不建立平行 Review 模型 |
| Migration | 唯一开发库从 16 非破坏性升级到 17，历史 P0 与 Iteration 16 数据保留 |
| 事务 | Run 创建、Report/Issue 消费、Runtime Retry 与消费 Retry 原子边界明确 |
| 幂等 | 创建、消费 Retry、Issue 处置复用 `idempotency_records` |
| P0 | Mock Review API、字段及历史 Report/Finding/Recommendation 兼容 |
| UI 追踪 | 8 个 Frame 可追踪到最终 OpenAPI Schema、Summary 状态和持久化模型 |
| 计划 | Iteration 17 固定为 CF-17-01、CF-17-02、CF-17-03、CF-17-04 四个任务 |
| 工程 | OpenAPI、Migration 17、专项脚本、受影响测试和 Git 门禁通过 |
