# Iteration 18 — 真实正文重写

**状态：`rebuild_candidate_2026_07_29`。** 当前包只重建需求文档和 9 个原型，不修改 OpenAPI、Migration 或业务代码。CF-18-01 才执行正式契约冻结。

## 1. 目标

基于一个明确 ReviewReport 和用户选中的 open ReviewIssue，通过真实 `content_rewrite` WorkflowRun 创建新的非当前候选 ContentVersion，保留来源版本、报告、问题、Run 和候选的完整追踪关系。

## 2. 用户闭环

审核报告选择 Issue → 创建页预检与二次确认 → queued/running → 严格输出校验 → 原子创建候选和 IssueLink → 查看/对比候选 → 明确设为当前版本。

异常分别恢复：未配置、Runtime 失败、输出校验失败、结果消费失败和 stale set-current。

## 3. 核心模型

- WorkflowRun(stage=content_rewrite, subjectType=review_report)
- ContentVersion(source=workflow_rewrite)
- ReviewReport / ReviewIssue（只读输入）
- RewriteSourceIssueLink
- idempotency_records

详细规则见 `business-rules.md`、`data-model.md` 和 `transaction-and-migration-design.md`。

## 4. API 候选范围

- Preflight：`POST /api/v1/review-reports/{reviewReportId}/rewrite-runs/preflight`
- Create：`POST /api/v1/review-reports/{reviewReportId}/rewrite-runs`
- Summary：`GET /api/v1/review-reports/{reviewReportId}/rewrite-summary`
- Consumption Retry：`POST /api/v1/workflow-runs/{workflowRunId}/rewrite-result-consumption-retries`
- 复用 Review、ContentVersion、版本列表/比较/设为当前、WorkflowRun Detail/Event/Runtime Retry/Cancel。

最终字段由 CF-18-01 OpenAPI 冻结。

## 5. UI 顺序

| 顺序 | Frame | 页面/状态 |
|---:|---|---|
| 1 | I18_D2_REVIEW_REWRITE_ENTRY | 审核结果选择问题并创建重写 |
| 2 | I18_D4_CREATE_REWRITE | 创建正文重写 |
| 3 | I18_D4_REWRITE_CONFIG_DRAWER | 查看项目配置抽屉 |
| 4 | I18_D4_REWRITE_RUNNING | queued/running |
| 5 | I18_D5_REWRITE_RESULT | 成功候选版本 |
| 6 | I18_D5_SET_CURRENT_CONFIRM | 设为当前确认 |
| 7 | I18_D5_RESULT_CONSUMPTION_FAILED | 结果消费失败 |
| 8 | I18_D4_REWRITE_FAILED | Runtime/输出校验失败 |
| 9 | I18_D4_REWRITE_AVAILABILITY | 未配置、配置失效与空状态 |

## 6. 实施顺序

1. CF-18-01：冻结业务、输入输出、OpenAPI、数据终态、事务、Summary 和 9 Frame 追踪；
2. CF-18-02：后端开发；
3. CF-18-03：前端开发；
4. CF-18-04：真实 n8n 联调、浏览器验收和最终 Review。

## 7. 不在范围

- 不覆盖或删除源版本；
- 不自动设为当前、审核或发布；
- 不修改 ReviewIssue disposition；
- 不建设 n8n 编辑器、字段映射器、多实例路由、费用大盘或自动模型路由；
- 不允许业务页面临时切换工作流；
- 不改动 P0 Mock Rewrite 契约。
