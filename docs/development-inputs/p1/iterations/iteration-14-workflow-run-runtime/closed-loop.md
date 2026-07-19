# Iteration 14 — WorkflowRun 异步运行基础设施 — Closed Loop

## 入口

建立队列 Worker、统一状态机、运行详情、事件时间线、幂等与手动重试。

## 用户动作与系统结果

运行可从 queued 到 succeeded/failed，刷新后恢复，错误安全可诊断，重试保留原记录。

## 异常闭环

- 未配置或绑定失效：阻止发起并引导至最近配置入口。
- 运行失败：保留 WorkflowRun、安全错误和重试入口。
- 输出 Schema 非法：不写领域结果。
- 领域事务失败：不保存部分数据，允许专用提交重试。
- 页面刷新：从 WorkflowRun 恢复状态。
