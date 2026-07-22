# Iteration 14 — WorkflowRun 异步运行 — UI Scope

## UI 验收结论

**PASS。** P14_01～P14_10 是人工验收通过的唯一 UI 基线。Stitch HTML 仅作为结构和视觉参考；开发必须复用现有产品壳层，不直接将其作为生产页面。

## 原型映射

| Frame | 用途 | 路由或入口 |
|---|---|---|
| `P14_01_WORKFLOW_RUN_LIST` | 全局运行记录列表 | `/workflow-runs` |
| `P14_02_WORKFLOW_RUN_LIST_PROJECT_FILTERED` | 项目筛选后的列表 | `/workflow-runs?projectId={projectId}` |
| `P14_03_WORKFLOW_RUN_LIST_EMPTY` | 运行记录空状态 | `/workflow-runs` |
| `P14_04_WORKFLOW_RUN_LIST_ERROR` | 运行记录错误和恢复 | `/workflow-runs` |
| `P14_05_WORKFLOW_RUN_DETAIL_SUCCEEDED` | 成功运行详情 | `/workflow-runs/{runId}` |
| `P14_06_WORKFLOW_RUN_DETAIL_RUNNING` | 运行中详情与取消 | `/workflow-runs/{runId}` |
| `P14_07_WORKFLOW_RUN_DETAIL_FAILED` | 失败详情与完整重试 | `/workflow-runs/{runId}` |
| `P14_08_RUN_CONFIRM_DIALOG` | 已绑定、可运行时的确认弹窗 | 项目业务页覆盖层 |
| `P14_09_WORKFLOW_NOT_BOUND_DIALOG` | 未绑定时的提示弹窗 | 项目业务页覆盖层，非独立页面 |
| `P14_10_PROJECT_WORKFLOW_RUN_SUMMARY` | 项目运行摘要与最近运行 | 项目概览 |

## 路由与展示规则

- 流程中心没有任何 Tab，直接展示全局运行记录；列表主路由仅为 `/workflow-runs`。
- 详情属于流程中心，统一使用 `/workflow-runs/{runId}`；面包屑为“流程中心 / 运行记录 / 运行编号”。
- 项目名称是详情字段和可点击链接，不是详情父级路由；不得新增项目级重复运行列表路由。
- 项目概览只提供摘要、最近运行列表及两个筛选入口：`/workflow-runs?projectId={projectId}` 与 `/workflow-runs?projectId={projectId}&stage={stage}`。
- 已绑定且可运行时“运行工作流”打开 P14_08；未绑定时打开 P14_09；创建成功后直接进入运行详情。

## 范围边界

本迭代激活 n8n 连接和工作流的验证、启用、停用，以及运行记录闭环；不激活 LLM Provider、分发平台、OAuth 或发布执行。运行状态与错误必须使用中文资源和脱敏 ViewModel。
