# Iteration 12 — 全局配置中心 V2 — 数据模型

## 1. 通用基础字段

所有配置模型包含：

- `id`: UUID 主键；
- `name`: 资源内唯一名称；
- `integrationStatus`: `not_connected / unverified / verified / failed`；
- `enabled`: boolean；
- `lastVerifiedAt`: nullable timestamptz；
- `lastErrorCode`: nullable varchar(80)；
- `lastErrorMessage`: nullable varchar(300)；
- `version`: integer，初始值 1；
- `createdAt`、`updatedAt`: timestamptz。

Iteration 12 固定规则：

- 新建配置的 `integrationStatus=not_connected`；
- 新建配置的 `enabled=false`；
- Iteration 12 没有 API 可以修改 `integrationStatus` 或 `enabled`；
- `lastVerifiedAt`、`lastErrorCode`、`lastErrorMessage` 保持为空；
- 这些运行状态字段为后续适配迭代预留，不代表本迭代已经完成第三方验证。

通用安全规则：

- 写操作使用仓库统一 `Idempotency-Key` 规范；
- 更新必须校验 `version`；
- 密钥只保存 AES-256-GCM 密文与不可逆指纹；
- DTO 不返回密文、明文密钥或 Authorization Header；
- PATCH 未提供新凭证表示保留旧凭证；
- 清除凭证使用显式 `clearSecret`/`clearCredential` 语义；
- 空字符串不得隐式清除凭证；
- 凭证替换只记录安全审计事件，不触发第三方调用。

## 2. LlmProviderConfiguration

建议表：`llm_provider_configurations`

| 字段 | 类型 | 约束 |
|---|---|---|
| `id` | uuid | PK |
| `name` | varchar(120) | unique, not null |
| `provider_type` | varchar(40) | 当前仅 `openai_compatible`；创建后不可修改 |
| `base_url` | varchar(512) | not null，仅保存，不发起访问 |
| `default_model` | varchar(160) | not null，用户填写或选择静态候选 |
| `encrypted_secret` | text | nullable，仅内部读取 |
| `secret_fingerprint` | varchar(32) | nullable |
| `timeout_seconds` | integer | 5–600，作为后续调用配置保存 |
| 通用状态字段 | — | 见第 1 节 |

读取 DTO 可以返回：

- `hasSecret`；
- 安全掩码或指纹；
- `integrationStatus=not_connected`；
- `enabled=false`。

不得返回任何密钥内容。

## 3. WorkflowConnection

建议表：`workflow_connections`

通用的工作流服务连接，当前连接类型仅支持 n8n。

| 字段 | 类型 | 约束 |
|---|---|---|
| `id` | uuid | PK |
| `name` | varchar(120) | unique, not null |
| `connection_type` | varchar(40) | 当前仅 `n8n`；创建后不可修改 |
| `base_url` | varchar(512) | not null，仅保存，不发起访问 |
| `auth_type` | varchar(40) | 当前 n8n 仅 `api_key` |
| `encrypted_credential` | text | nullable，仅内部读取 |
| `credential_fingerprint` | varchar(32) | nullable |
| `timeout_seconds` | integer | 5–600 |
| `type_config` | jsonb | 连接类型专属非敏感参数，按本地 Schema 校验 |
| 通用状态字段 | — | 见第 1 节 |

数据库约束：

- 名称唯一；
- `connection_type` 枚举合法；
- `timeout_seconds` 范围合法；
- 创建后不得修改 `connection_type`；
- 已被工作流引用的连接不得硬删除；
- Iteration 12 不创建“最多一个启用连接”的部分唯一索引，因为本迭代没有启用动作；
- 该启用约束由 Iteration 14 在激活运行状态时增加。

## 4. WorkflowConfiguration

建议表：`workflow_configurations`

| 字段 | 类型 | 约束 |
|---|---|---|
| `id` | uuid | PK |
| `name` | varchar(160) | unique, not null |
| `connection_id` | uuid | FK → `workflow_connections.id`, not null |
| `applicable_stages` | jsonb 或关联表 | `chapter_planning / content_generation / review / rewrite`，至少一个 |
| `type_config` | jsonb | 由关联连接类型决定，仅做本地 Schema 校验 |
| `input_contract_version` | varchar(40) | not null |
| `output_contract_version` | varchar(40) | not null |
| `default_parameters` | jsonb | 默认 `{}` |
| `note` | text | nullable |
| 通用状态字段 | — | 见第 1 节 |

派生字段：

- `connectionType`：通过 `connection_id` 关联查询；
- `workflowType`：由 `connectionType` 服务器端推导；
- 当前 `connectionType=n8n` 时，`workflowType=n8n`；
- 客户端不得提交或修改 `workflowType`。

当前 n8n `type_config` 本地 Schema：

```json
{
  "referenceType": "workflow_id | webhook_path",
  "referenceValue": "string"
}
```

本迭代只校验字段格式，不访问 n8n。

业务约束：

- 创建工作流必须选择存在的连接；
- 禁止无连接的“系统内置工作流”记录；
- 修改 `connection_id` 后重新推导 `workflowType`；
- 如果新旧连接类型不同，清空旧 `type_config`；
- 如果类型相同，仍需按当前类型 Schema 重新校验；
- 修改关联连接不触发第三方验证；
- `integrationStatus` 保持 `not_connected`；
- `enabled` 保持 `false`。

## 5. DistributionPlatformConfiguration

建议表：`distribution_platform_configurations`

| 字段 | 类型 | 约束 |
|---|---|---|
| `id` | uuid | PK |
| `name` | varchar(120) | unique, not null |
| `platform_type` | varchar(60) | `wechat_official_account / douyin / youtube / custom`；创建后不可修改 |
| `account_identifier` | varchar(240) | not null |
| `endpoint_url` | varchar(512) | custom 类型必填，其他类型按本地 Schema 决定 |
| `auth_type` | varchar(40) | `api_key / oauth / access_token / custom` |
| `encrypted_credential` | text | nullable，仅内部读取 |
| `credential_fingerprint` | varchar(32) | nullable |
| `timeout_seconds` | integer | 5–600 |
| `type_config` | jsonb | 平台专属非敏感字段，按本地 Schema 校验 |
| `note` | text | nullable |
| 通用状态字段 | — | 见第 1 节 |

规则：

- 平台类型创建后不可修改；
- 用户可见列表不得展示认证凭证；
- 编辑态只显示 `hasCredential` 和安全掩码；
- 本迭代不执行 OAuth、不验证凭证、不发送发布请求；
- `integrationStatus=not_connected`；
- `enabled=false`。

## 6. AuditLog

复用现有审计模型；若不存在，新增最小结构。

Iteration 12 记录：

- `resourceType`: `llm_provider / workflow_connection / workflow_configuration / distribution_platform`；
- `resourceId`；
- `action`: `create / update / secret_replace / secret_clear / credential_replace / credential_clear / connection_rebind`；
- `safeDiff`；
- `actorId`、`result`、`createdAt`。

Iteration 12 不记录：

- `verify`；
- `enable`；
- `disable`；
- `execute`；
- `publish`。

禁止记录：

- 明文密钥或密文；
- Authorization Header；
- 完整请求体；
- 用户凭证；
- 任何虚构的第三方响应。

## 7. 迁移与兼容

- 不破坏第一闭环已有表和数据；
- 如果仓库已存在 `n8n_connections`，通过安全迁移演进为 `workflow_connections` 或建立唯一兼容层；
- 禁止同时保留两套语义重复的真值模型；
- 迁移前检查已有表、索引和 OpenAPI 名称冲突；
- 迁移必须支持测试环境重复初始化；
- Iteration 14 再增加真实验证、启用状态和单启用连接约束；
- 不因预留状态字段而在 Iteration 12 实现任何 Adapter。
