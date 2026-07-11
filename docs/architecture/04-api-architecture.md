# API 架构

## 1. 基线

- 前缀：`/api/v1`。
- 健康检查：`/healthz`、`/readyz`。
- Content-Type：`application/json`。
- 所有响应携带 `request_id`。

成功响应：

```json
{
  "data": {},
  "request_id": "req_xxx"
}
```

错误响应建议：

```json
{
  "error": {
    "code": "invalid_state",
    "message": "chapter plan is already confirmed",
    "details": {}
  },
  "request_id": "req_xxx"
}
```

## 2. 错误分类

| HTTP | 语义 |
|---|---|
| 400 | 输入结构、字段或枚举错误 |
| 404 | 资源不存在 |
| 409 | 重复、版本冲突或状态冲突 |
| 422 | 业务前置条件不满足 |
| 500 | 未预期错误 |
| 503 | 依赖未就绪 |

## 3. 契约优先

实现顺序：

```text
OpenAPI / JSON Schema
→ Generated or mapped types
→ Application use case
→ HTTP handler
→ Contract tests
→ Web client
```

## 4. 写接口

写接口必须定义：

- 输入校验。
- 状态前置条件。
- 事务边界。
- 冲突语义。
- 幂等策略。
- 审计事件。

完整 Endpoint 目录位于：

```text
packages/contracts/planning/p0/p0-api-catalog.yaml
```
