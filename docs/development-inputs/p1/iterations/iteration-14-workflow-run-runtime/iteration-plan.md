# Iteration 14 — n8n 适配与 WorkflowRun 异步运行

## 1. 目标

在 Iteration 12 配置 CRUD 和 Iteration 13 项目绑定基础上，实现：

1. n8n Connection Adapter；
2. 连接真实验证与启用/停用；
3. 工作流引用真实验证与启用/停用；
4. WorkflowRun 创建、队列、Worker、状态机、事件时间线和重试；
5. 后端异步调用 n8n Webhook；
6. 安全错误、绑定快照、输出校验和领域提交边界。

Iteration 14 是第二闭环中首次允许调用真实工作流第三方服务的迭代。

## 2. 前置条件

- Iteration 12 已保存 LLM、连接、工作流和分发平台配置；
- Iteration 13 已保存项目四环节绑定；
- 连接和工作流默认处于 `not_connected`；
- 数据库迁移可以安全激活验证和启用状态。

## 3. 真实接入闭环

### 3.1 连接

连接列表 → 验证连接 → n8n Adapter 使用加密凭证发起安全测试 → 成功后状态为 `verified` → 用户启用连接。

规则：

- 当前只实现 n8n Adapter；
- 失败保存安全错误码和脱敏错误；
- 第二闭环最多一个启用连接；
- 启用新连接时原连接在同一事务中停用；
- 连接配置变化后状态重置为 `unverified`。

### 3.2 工作流

工作流列表 → 验证工作流 → 通过绑定连接验证 Workflow ID / Webhook Path → 成功后状态为 `verified` → 用户启用工作流。

规则：

- 工作流类型由连接类型推导；
- 关联连接变化后重新验证；
- 关联连接未验证或未启用时，工作流不能启用。

### 3.3 运行

业务动作 → 读取项目绑定 → 检查连接和工作流可运行 → 创建绑定快照 → 创建 `WorkflowRun(queued)` → Worker 调用 n8n Webhook → 校验输出 → 成功或失败 → UI 刷新恢复。

不建设 callback server。n8n 通过 Webhook HTTP 响应返回结果；超时、断开和非预期输出进入失败状态。

## 4. 数据模型

- `WorkflowRun`
- `WorkflowRunEvent`
- `ProjectWorkflowBindingSnapshot`
- 连接和工作流验证状态；
- Adapter 安全错误模型。

详细字段见 `data-model.md`。

## 5. API

Iteration 14 新增或激活：

- 连接验证、启用、停用；
- 工作流验证、启用、停用；
- 创建 WorkflowRun；
- WorkflowRun 列表、详情、事件；
- 运行重试；
- 领域提交重试。

冻结范围见 `api-scope.yaml`。

## 6. UI 与原型

本迭代包含：

- Workflow Center；
- WorkflowRun 详情抽屉；
- 重试确认弹窗；
- 运行中状态条；
- 失败通知；
- 未配置/未接入状态。

同时激活 Iteration 12 页面中此前禁用的：

- n8n 连接验证和启用；
- 工作流验证和启用。

## 7. 实施顺序

### 14.1 Adapter 契约与安全基础

- 定义通用 `WorkflowAdapter` 接口；
- 实现 n8n Adapter；
- 实现超时、重试策略、错误归一化和日志脱敏；
- 使用测试服务器或可控 n8n 环境完成契约测试。

### 14.2 连接与工作流验证

- 激活连接验证/启停 API；
- 增加最多一个启用连接的数据库约束；
- 激活工作流验证/启停 API；
- 配置变更后的状态重置。

### 14.3 WorkflowRun 基础设施

- 运行模型和迁移；
- 队列和 Worker；
- 状态机和事件；
- 绑定快照；
- 幂等和并发控制。

### 14.4 n8n 真实执行

- 创建运行；
- 调用 Webhook；
- 校验输入和输出；
- 处理超时、HTTP 错误、Schema 错误和领域提交失败；
- 不实现 callback server。

### 14.5 UI 联调和验收

- Workflow Center 与运行详情；
- 刷新恢复；
- 手动重试；
- 局部 Adapter 测试；
- API/Worker 分组测试；
- 一次全链路总门禁；
- Code Review 和 Git 验收。

## 8. 不在范围

- 不实现 n8n 可视化编辑器；
- 不实现多 n8n 实例智能路由；
- 不实现 Coze、ComfyUI 等其他 Adapter；
- 不实现 callback server；
- 不直接调用 LLM Provider；
- 不获取 LLM 模型列表；
- LLM Provider 的真实验证和直接调用放到 Iteration 15 或首个直接 LLM 业务迭代；
- 不实现分发平台验证、OAuth 或发布；
- 不静默改变第一闭环 Mock 契约。

## 9. 完成定义

- [ ] n8n 连接可以真实验证和启用；
- [ ] 工作流引用可以真实验证和启用；
- [ ] 最多一个连接启用约束生效；
- [ ] 运行从 queued 到 running，再到 succeeded 或 failed；
- [ ] 刷新后状态恢复；
- [ ] 错误脱敏且可诊断；
- [ ] 重试保留原运行记录；
- [ ] 不存在 callback server；
- [ ] 不调用 LLM 或分发平台；
- [ ] OpenAPI、实现和 UI 一致；
- [ ] 局部、分组和全链路验收通过。
