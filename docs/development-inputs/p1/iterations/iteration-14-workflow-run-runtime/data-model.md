# Iteration 14 — n8n 适配与 WorkflowRun 异步运行 — 数据模型

## 1. 激活既有配置运行状态

Iteration 12 已预留：

- `integrationStatus`；
- `enabled`；
- `lastVerifiedAt`；
- `lastErrorCode`；
- `lastErrorMessage`。

Iteration 14 开始允许这些字段变化。

### WorkflowConnection

状态规则：

- `not_connected`：仅保存配置，尚未执行真实验证；
- `unverified`：配置已进入真实适配阶段，等待验证或配置修改后需重新验证；
- `verified`：最近一次真实验证成功；
- `failed`：最近一次真实验证失败。

启用规则：

- 只有 `verified` 可以启用；
- 第二闭环最多一个连接 `enabled=true`；
- 使用数据库部分唯一索引或等价约束；
- Base URL、凭证、认证方式或类型专属字段变化后重置为 `unverified`。

### WorkflowConfiguration

- 只有绑定连接已验证且启用时，工作流才能验证和启用；
- `connection_id`、类型专属引用、契约版本变化后重置为 `unverified`；
- 连接变化后 `workflowType` 重新推导。

## 2. WorkflowRun

建议字段：

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | uuid | PK |
| `project_id` | uuid | 项目 |
| `stage` | varchar(40) | 四环节之一 |
| `status` | varchar(30) | `queued / running / succeeded / failed` |
| `binding_snapshot` | jsonb | 不含明文密钥的绑定快照 |
| `input_payload` | jsonb | 经 Schema 校验后的输入 |
| `output_payload` | jsonb | 成功后的安全输出 |
| `error_code` | varchar(80) | 脱敏错误码 |
| `error_message` | varchar(300) | 脱敏错误 |
| `retry_of_run_id` | uuid | nullable |
| `idempotency_key` | varchar(160) | 创建幂等 |
| `started_at` | timestamptz | nullable |
| `finished_at` | timestamptz | nullable |
| `version` | integer | 并发控制 |
| `created_at` | timestamptz | not null |
| `updated_at` | timestamptz | not null |

禁止在快照和 Payload 中保存：

- 明文 API Key；
- 加密密文；
- Authorization Header；
- 原始上游错误响应。

## 3. WorkflowRunEvent

字段：

- `id`；
- `run_id`；
- `sequence`；
- `event_type`；
- `safe_payload`；
- `created_at`。

事件示例：

- queued；
- worker_started；
- request_sent；
- response_received；
- output_validated；
- domain_commit_started；
- succeeded；
- failed；
- retry_created。

## 4. ProjectWorkflowBindingSnapshot

保存：

- 项目 ID；
- 环节；
- 绑定 ID 和版本；
- 工作流 ID、名称、版本；
- 连接 ID、类型、版本；
- 类型专属非敏感配置；
- 契约版本；
- 默认参数；
- 创建运行时的时间戳。

不保存凭证明文或密文。

## 5. Adapter 契约

通用 `WorkflowAdapter`：

- `verifyConnection`；
- `verifyWorkflow`；
- `execute`；
- `normalizeError`。

Iteration 14 实现：

- `N8nWorkflowAdapter`。

Adapter 输入凭证仅在 Worker 内短暂解密，使用后释放，不写日志和数据库。

## 6. 状态机和事务

合法状态转换：

```text
queued → running → succeeded
queued → running → failed
```

重试：

- 完整重试创建新 WorkflowRun；
- 原运行不可覆盖；
- 外部执行成功但领域提交失败时，可以创建领域提交重试；
- 领域提交重试不重复调用 n8n。

## 7. 迁移

- 激活 Iteration 12 预留状态字段；
- 将已有 `not_connected` 连接和工作流迁移为等待用户验证；
- 增加最多一个启用连接约束；
- 增加 WorkflowRun、Event 和快照存储；
- 不修改 LLM Provider 和分发平台为已验证状态。
