# Iteration 18 — 真实正文重写

**状态：`frozen_cf_18_01a`。** CF-18-01A 已冻结业务、API、Runtime 输入输出、Summary、Retry、Set Current、错误语义和 9 Frame 追踪；数据与 Migration 设计仍由 CF-18-01B 冻结。

## 1. 目标

用户从一个明确 ReviewReport 选择同 Report 的 1～50 个 `open` ReviewIssue，通过真实 `WorkflowRun(stage=rewrite, subjectType=review_report)` 基于 Report 的固定来源 ContentVersion 创建一个新的非当前 Rewrite Candidate，并显式执行 Set Current/CAS。

## 2. 用户闭环

审核报告与 Issue → Rewrite Availability → 配置要求 → 无副作用 Preflight → 二次确认 → queued/running → 严格校验 `rewrite.output.v1` → 原子创建非当前 Candidate → 查看/比较来源与候选 → Set Current/CAS。

异常分别恢复：未配置、Runtime failed/cancelled、输出校验失败、结果消费失败和 CAS 冲突。刷新仅从持久化 Summary/Run/Event/Candidate 恢复。

## 3. 冻结模型边界

- 复用 WorkflowRun、ReviewReport、ReviewIssue、ContentItem、ContentVersion/Candidate 与 Idempotency 体系。
- Stage 唯一为 `rewrite`；subject 唯一为 `review_report/reviewId`。
- Input/Output 唯一为 `rewrite.input.v1` / `rewrite.output.v1`。
- Rewrite 不等于 Review，不修改 Report/Issue/来源版本。
- Candidate 非当前；Set Current 是独立幂等 CAS。
- 不新建第二套 Candidate、Review 或 Runtime 体系。

具体数据表、关联记录、约束、锁、索引和 Migration 不在 CF-18-01A 决策范围，以 CF-18-01B 最终冻结为准。

## 4. 正式 API

新增：

- `GET /api/v1/reviews/{reviewId}/rewrite-availability`
- `POST /api/v1/reviews/{reviewId}/rewrites/preflight`
- `POST /api/v1/reviews/{reviewId}/rewrites`
- `GET /api/v1/reviews/{reviewId}/rewrite-summary`
- `GET /api/v1/content-items/{contentItemId}/rewrite-history`
- `GET /api/v1/workflow-runs/{workflowRunId}/rewrite-result`
- `POST /api/v1/workflow-runs/{workflowRunId}/rewrite-result-consumption-retries`

复用 Set Current、WorkflowRun Detail/Event/Runtime Retry/Cancel 和 ContentVersion 只读详情。完整字段见 OpenAPI 与 `api-scope.yaml`。

## 5. Summary

只允许八状态：`idle`、`not_configured`、`queued`、`running`、`candidate_ready`、`runtime_failed`、`output_validation_failed`、`result_consumption_failed`。

active Run 优先；succeeded 且 Candidate 已原子持久化才是 ready；后续配置失效不遮蔽已有事实。Set Current 后保持 `candidate_ready`，通过 `candidateIsCurrent=true` 表达。

## 6. UI 顺序

| 顺序 | Frame | 页面/状态 |
|---:|---|---|
| 1 | I18_D2_REVIEW_REWRITE_ENTRY | 审核结果选择问题并创建重写 |
| 2 | I18_D4_CREATE_REWRITE | 创建、Preflight 与二次确认 |
| 3 | I18_D4_REWRITE_CONFIG_DRAWER | 只读查看项目配置 |
| 4 | I18_D4_REWRITE_RUNNING | queued/running |
| 5 | I18_D5_REWRITE_RESULT | candidate_ready |
| 6 | I18_D5_SET_CURRENT_CONFIRM | Set Current 确认 |
| 7 | I18_D5_RESULT_CONSUMPTION_FAILED | 仅消费 Retry |
| 8 | I18_D4_REWRITE_FAILED | Runtime/输出校验失败 |
| 9 | I18_D4_REWRITE_AVAILABILITY | idle/not_configured/可用性 |

## 7. 实施顺序

1. CF-18-01A：业务与 API 契约冻结（模型：GPT-5.6 Sol；推理：high；已完成）；
2. CF-18-01B：数据模型、事务与 Migration 契约冻结（模型：GPT-5.6 Sol；推理：high）；
3. CF-18-02A：后端 Preflight、Token、创建 Rewrite Run 与输入快照（模型：GPT-5.6 Sol；推理：high）；
4. CF-18-02B：后端输出校验、Candidate 原子消费与消费失败（模型：GPT-5.6 Sol；推理：high）；
5. CF-18-02C：后端 Summary、Runtime Retry、消费 Retry、Set Current 与历史（模型：GPT-5.6 Sol；推理：high）；
6. CF-18-03A：前端审核入口、Availability、Preflight 与创建重写（模型：GPT-5.6 Terra；推理：medium）；
7. CF-18-03B：前端 queued/running、失败恢复、结果和 Candidate 展示（模型：GPT-5.6 Terra；推理：medium）；
8. CF-18-03C：前端 Set Current、CAS 冲突、刷新恢复与历史（模型：GPT-5.6 Terra；推理：medium）；
9. CF-18-04A：Iteration 18 全量代码 Review（模型：GPT-5.6 Sol；推理：high；只做 Review，不修改代码）；
10. CF-18-04B：Iteration 18 前后端功能联调与工程冻结（模型：GPT-5.6 Sol；推理：high；不包含浏览器截图比对和视觉验收）。

任务按序执行；CF-18-01A 不提前执行 CF-18-01B。

## 8. 不在范围

- 不覆盖或删除来源版本；
- 不自动设为当前、审核、再次重写或发布；
- 不修改 ReviewIssue disposition；
- 不建设工作流编辑器、多实例路由、费用或自动模型路由；
- 不允许业务页面临时切换工作流；
- 历史 Mock 与旧 Candidate 不伪造真实 Rewrite 来源。
