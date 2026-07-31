# Iteration 19 — LLM、n8n 与 WorkflowRun 真实接入数据模型

**状态：`logical_model_frozen_for_migration_19`。** 本文件冻结逻辑字段、约束和快照语义。实现前必须核对 Migration 18 后的实际 Schema，只增加缺失字段/约束，禁止建立第二套 Provider、Connection、Workflow Configuration、Binding 或 WorkflowRun 表。

## 1. 通用配置状态

`llm_provider_configurations`、`workflow_connections`、`workflow_configurations` 复用现有字段并统一语义：

| 字段 | 类型 | 规则 |
|---|---|---|
| `integration_status` | text | `unverified/verifying/verified/failed/stale`；历史 `not_connected` 在 Migration 19 迁为 `unverified` |
| `enabled` | boolean | 用户启停事实；配置变更不自动改为 false |
| `last_verified_version` | integer nullable | 只有验证成功时等于当前 `version` |
| `last_verified_at` | timestamptz nullable | 最近验证完成时间 |
| `last_error_code` | varchar(80) nullable | 稳定安全错误码 |
| `last_error_message` | varchar(300) nullable | 安全中文摘要，不含敏感内容 |
| `validation_details` | jsonb | 安全分项结果；不得包含凭据、完整 Header 或原始响应 |
| `version` | integer | 乐观锁和配置版本，关键字段变化加 1 |

`executable` 不持久化，服务端按当前版本、enabled 和依赖链派生。`verified` 但 `enabled=false` 仍不可执行；`enabled=true` 但 `stale/failed` 也不可执行。

## 2. LLM Provider

复用 `llm_provider_configurations`：

- `provider_type=openai_compatible`；
- `base_url`、`timeout_seconds`、`encrypted_secret`、`secret_fingerprint`；
- `default_model`；
- 新增或确保 `model_catalog_updated_at`。

新增 `llm_provider_models`：

| 字段 | 类型 | 约束 |
|---|---|---|
| `id` | uuid | PK |
| `provider_id` | uuid | FK，ON DELETE CASCADE |
| `model_key` | varchar(200) | 非空 |
| `source` | text | `discovered/manual` |
| `availability` | text | `available/unavailable` |
| `last_seen_at` | timestamptz nullable | 最近发现时间 |
| `created_at/updated_at` | timestamptz | 非空 |

唯一约束：`UNIQUE(provider_id, model_key)`。模型从远端目录消失时标记 unavailable，不物理删除；默认模型不可用时 Provider 验证失败，禁止自动改选。

关键字段：`base_url`、凭据、`timeout_seconds`、`default_model`。变更后 `version+1`、`integration_status=stale`、清除当前验证详情，但保留 enabled。

## 3. n8n Connection

复用 `workflow_connections`：

- `connection_type=n8n`；
- `base_url` 必须是实例根地址；
- `auth_type`、`encrypted_credential`、`credential_fingerprint`、`timeout_seconds`；
- 不保存 Workflow ID 或 Webhook Path。

关键字段变化后同样转 `stale`。Connection 被引用或停用时不级联删除 Workflow Configuration、Binding 或历史 Run。

## 4. Workflow Configuration

复用 `workflow_configurations`，新增或确保：

| 字段 | 类型 | 约束 |
|---|---|---|
| `llm_strategy` | text | `acf_managed/n8n_managed/none`，非空 |
| `llm_provider_id` | uuid nullable | FK → Provider，ON DELETE RESTRICT |
| `llm_model` | varchar(200) nullable | ACF-managed 必填 |
| `last_verified_version` 等状态字段 | — | 见第 1 节 |

CHECK：

```text
acf_managed  => llm_provider_id IS NOT NULL AND llm_model IS NOT NULL
n8n_managed  => llm_provider_id IS NULL AND llm_model IS NULL
none         => llm_provider_id IS NULL AND llm_model IS NULL
```

关键字段：Connection、Stage、referenceType/referenceValue、输入/输出契约、默认参数、LLM 策略、Provider、模型。任何变化加版本并转 stale。

项目级绑定不得增加 Provider、模型、LLM 参数或策略覆盖字段。

## 5. ProjectWorkflowBinding 读取模型

`project_workflow_bindings` 不新增持久化状态字段。读取 DTO 派生：

- `bound`；
- `executable`；
- `ineligibilityReasons[]`；
- `workflowConfigurationVersion`；
- `connectionSummary`；
- `llmPolicySummary`；
- `lastVerifiedAt`。

依赖失效不修改或删除 Binding。

## 6. WorkflowRun 状态

复用 `workflow_run_records`，Runtime `status` 统一为：

- `queued`；
- `running`；
- `cancelling`；
- `succeeded`；
- `failed`；
- `cancelled`；
- `timed_out`。

新增或确保：

| 字段 | 类型 | 说明 |
|---|---|---|
| `failure_phase` | text nullable | `external_execution/output_validation/result_consumption/cancellation` |
| `failure_code` | varchar(100) nullable | 稳定错误码 |
| `safe_error_message` | varchar(500) nullable | 安全摘要 |
| `retryability` | text | `runtime_retry/result_consumption_retry/not_retryable` |
| `retry_of_run_id` | uuid nullable | FK → 同表，禁止 self-reference |
| `retry_mode` | text nullable | `current_configuration/original_configuration` |
| `external_execution_id` | varchar(200) nullable | n8n execution ID，非敏感 |
| `cancellation_requested_at` | timestamptz nullable | 取消请求时间 |
| `timed_out_at` | timestamptz nullable | 超时终止时间 |

`displayStatus` 为 API 派生：当 status=failed 且 failure_phase=output_validation/result_consumption 时分别返回对应 UI 状态。

## 7. 不可变运行快照

每个 Run 必须冻结且只读：

- `binding_snapshot`：bindingId、bindingVersion、stage；
- `configuration_snapshot`：id、name、version、reference、契约、默认参数安全摘要；
- `connection_snapshot`：id、name、version、type、baseUrl 安全规范化值、authType、credentialFingerprint；
- `llm_policy_snapshot`：strategy、providerId/name/version、model、secretFingerprint；
- `input_payload`：对应 Stage 的冻结输入。

快照不得包含密钥、密文、Credential、Authorization Header、Cookie、完整敏感上游响应、SQL、堆栈或内部节点数据。

## 8. 原配置可重放判断

不建设密钥历史表。`original_configuration` 仅当：

1. 快照字段完整；
2. 引用记录仍存在；
3. 当前 Secret/Credential fingerprint 与快照一致；
4. 快照 URL/引用通过当前安全校验；
5. 外部平台允许再次调用；
6. 业务输入仍符合重试规则。

任一条件不满足，返回不可重放原因，前端禁用原配置选项。

## 9. WorkflowRunEvent

复用 `workflow_run_events`，eventType 至少覆盖：

`created/queued/execution_started/llm_started/llm_completed/cancel_requested/cancelled/timed_out/output_validation_failed/result_consumption_failed/succeeded/failed/retry_created`。

Event 仅保存安全摘要和关联 ID，不保存原始外部响应。

## 10. 事务与一致性

- Verify 成功必须在单事务中更新当前版本状态、验证详情和 Audit；版本已变化时返回 409，不覆盖新配置。
- Enable 必须锁定记录并确认 `last_verified_version=version`；Workflow 还需锁定/复核依赖。
- 创建 Run 必须锁定绑定并在同事务冻结所有快照和幂等记录。
- 输出 Schema 校验完成后才能开始领域事务。
- Result consumption 失败必须零部分数据或完整回滚；专用重试复用持久化输出。
- Retry 创建新 Run，原 Run 不更新为新状态。

## 11. 索引与约束

Migration 19 只增加实际查询需要且现有不存在的索引：

- Provider/Connection/Workflow `(integration_status, enabled, updated_at)`；
- `llm_provider_models(provider_id, availability, model_key)`；
- Workflow `(applicable_stage, llm_strategy, integration_status, enabled)` 的实际存储形态索引；
- Run `(project_id, stage, status, created_at DESC, id DESC)`；
- Run `retry_of_run_id`；
- Run `external_execution_id` 在非空时按连接范围唯一。

不得重复 Iteration 15～18 已有索引。

## 12. Audit

新增安全 action：

- `llm_provider.verify/enable/disable/model_discover`；
- `workflow_connection.verify/enable/disable`；
- `workflow_configuration.verify/enable/disable`；
- `workflow_run.cancel_requested/retry_created`。

Audit 只记录 ID、版本、状态、错误码、模型标识和安全差异；不记录密钥、Credential、原始请求或 Idempotency-Key 明文。
