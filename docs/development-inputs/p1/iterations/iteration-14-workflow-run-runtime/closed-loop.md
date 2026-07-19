# Iteration 14 — n8n 适配与 WorkflowRun 异步运行 — Closed Loop

## 1. 连接验证闭环

连接列表 → 点击验证 → 后端解密凭证 → n8n Adapter 发起测试 → 返回成功或脱敏失败 → 刷新状态。

成功后：

- `integrationStatus=verified`；
- 记录最近验证时间；
- 允许启用。

失败后：

- `integrationStatus=failed`；
- 保存安全错误码；
- 不允许启用；
- 不回显原始响应或凭证。

## 2. 工作流验证闭环

工作流列表 → 点击验证 → 检查关联连接已验证 → Adapter 验证 Workflow ID / Webhook Path → 更新状态。

只有工作流和连接都满足条件时才能启用工作流。

## 3. WorkflowRun 闭环

业务动作 → 读取项目绑定 → 检查运行就绪 → 创建绑定快照 → 创建 queued 运行 → Worker 领取 → running → 调用 n8n Webhook → 校验输出 → 领域提交 → succeeded。

失败路径：

- 连接或工作流未就绪：不创建运行，返回最近配置入口；
- HTTP、超时或 Adapter 错误：运行 failed；
- 输出 Schema 错误：运行 failed，不写领域结果；
- 领域提交失败：保留外部成功结果，允许领域提交重试；
- 页面刷新：从 WorkflowRun 和事件恢复。

## 4. 重试闭环

### 完整重试

失败详情 → 点击重试 → 选择使用原快照或当前配置 → 确认 → 创建新运行。

原运行保留，建立 `retryOfRunId` 关系。

### 领域提交重试

只在外部执行成功、领域提交失败时开放：

- 不重新调用 n8n；
- 使用已验证输出重新执行领域事务；
- 记录独立事件。

## 5. 无 callback server

- 后端 Worker 主动调用 n8n Webhook；
- n8n 通过 HTTP 响应返回执行结果；
- 不暴露回调端点；
- 超时按失败处理；
- 重试由 WorkflowRun 机制控制。

## 6. LLM 和分发平台边界

Iteration 14：

- 不验证 LLM Provider；
- 不获取模型列表；
- 不直接调用 LLM；
- 不验证分发平台；
- 不执行 OAuth；
- 不发布内容。
