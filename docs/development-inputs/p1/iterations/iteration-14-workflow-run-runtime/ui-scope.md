# Iteration 14 — WorkflowRun 异步运行 — UI Scope

**状态：`frozen_cf_14_01_r3`。**

## UI 验收结论

**PASS。** P14_01～P14_10 是唯一 UI 基线；保留既有 P14 页面结构，不重新设计 UI。Stitch HTML 仅为结构和视觉参考。

## 路由与展示

- 流程中心共享列表：`/workflow-runs`；项目入口只使用 `?projectId={projectId}` 和 `?projectId={projectId}&stage={stage}` 过滤。
- 详情使用 `/workflow-runs/{runId}`；项目摘要使用冻结的摘要接口。
- 列表、详情、loading、empty、error/retry、确认弹窗、未绑定提示和项目摘要继续对应 P14_01～P14_10。

## 当前范围

UI 消费内部 Connection、WorkflowConfiguration 记录和 Runtime Run 数据。当前不激活真实 n8n、连接验证、工作流验证、verify、enable、disable、Worker 或外部执行；UI 不得要求展示或实现这些操作。

项目环节可绑定存在的 WorkflowConfiguration（及其存在的 Connection）。`enabled` 与 `integrationStatus` 是未来集成预留元数据，不得被 UI 解释为当前绑定或创建 queued Run 的资格条件。运行状态与错误使用中文资源及脱敏 ViewModel。
