# 可观测性与运行诊断

## 1. HTTP 日志

至少记录：

- timestamp。
- level。
- service。
- request_id。
- method、path、status_code。
- duration_ms。
- error_code。

## 2. WorkflowRun

记录：

- provider_key、workflow_key。
- subject_type、subject_id。
- queued、started、finished 时间。
- 输入摘要、输出摘要和错误摘要。
- 重试次数和最终状态。

## 3. AuditLog

核心写操作记录：actor、action、subject、payload 摘要和时间。

AuditLog 不替代业务事件，也不能保存密钥或完整敏感正文。

## 4. 健康检查

- `/healthz`：进程存活。
- `/readyz`：PostgreSQL、Redis 等依赖可用。
- Web 不应把 API 不可用误显示为业务空态。
