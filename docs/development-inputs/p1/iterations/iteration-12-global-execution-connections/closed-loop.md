# Iteration 12 — 全局执行连接 — Closed Loop

## 入口

实现 OpenAI-compatible LLM Provider 与单 n8n Connection 的安全配置、验证和启停。

## 用户动作与系统结果

管理员配置并验证 Provider/n8n，项目绑定可读取可用连接，密钥始终不回显。

## 异常闭环

- 未配置或绑定失效：阻止发起并引导至最近配置入口。
- 运行失败：保留 WorkflowRun、安全错误和重试入口。
- 输出 Schema 非法：不写领域结果。
- 领域事务失败：不保存部分数据，允许专用提交重试。
- 页面刷新：从 WorkflowRun 恢复状态。
