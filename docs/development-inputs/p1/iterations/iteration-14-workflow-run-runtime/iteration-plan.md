# Iteration 14 — WorkflowRun Runtime 与执行器抽象

## 当前状态

`frozen_cf_14_01_r3`。本迭代冻结“配置记录不等于真实集成”的终态契约。

## 当前目标与配置语义

- `WorkflowConnection` 和 `WorkflowConfiguration` 仅为平台内部创建的配置记录，为项目绑定和 WorkflowRun 脱敏快照提供数据。
- 它们不表示已连接 n8n、已完成鉴权、已验证可调用或已启用真实执行能力。
- `enabled` 与 `integrationStatus` 为未来真实集成预留元数据，当前不参与绑定或创建 Run 的资格判断。
- 合法闭环：创建 Connection → 创建 WorkflowConfiguration → 绑定到项目环节 → 创建 queued WorkflowRun → 查询、取消、重试、摘要。
- 绑定只要求 WorkflowConfiguration 及其引用的 WorkflowConnection 存在；创建 Run 还要求 Project 与对应 stage Binding 存在，并满足冻结的参数、版本和幂等要求。

## 当前 Run 行为

生产默认 `UnavailableWorkflowExecutor` 只创建 `queued` WorkflowRun、初始 Event 和脱敏配置快照；不调用 `WorkflowExecutor.Execute`、不启动 Worker、不调用 n8n、不伪造 `running` 或 `succeeded`。`FakeWorkflowExecutor` 仅用于测试。

## 延后范围

真实 n8n Adapter、鉴权、连接/工作流可调用性验证、verify、enable、disable、Worker、队列消费、callback server、外部状态回写及外部成功/失败/超时/取消联调均延后至 `CF-14-N8N-Integration`，不构成当前验收门槛。

## 后续顺序

CF-14-01-R3 → CF-14-02-R2 → CF-14-03A-R1 → CF-14-03B/C → Iteration 14 最终联调 → CF-14-N8N-Integration 后续独立开发。
