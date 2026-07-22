# Iteration 14 — n8n 适配与 WorkflowRun 异步运行 — 数据模型

**状态：待 CF-14-01 冻结。** 以下为 Iteration 14 草案，不提前声明最终 Schema 或 Migration 已冻结。

## 连接与工作流运行就绪状态

`WorkflowConnection` 与 `WorkflowConfiguration` 激活既有的 `integrationStatus`、`enabled`、`lastVerifiedAt`、`lastErrorCode` 和 `lastErrorMessage`。

- 验证状态：`not_connected`、`unverified`、`verified`、`failed`；连接或工作流的关键配置变更后重置为 `unverified`。
- 仅 `verified` 的连接可以启用；本闭环最多一个启用连接，启用新连接需原子停用旧连接。
- 工作流验证和启用要求其绑定连接已验证且已启用；工作流配置变化或关联连接变化后需要重新验证。
- 最近错误只保存脱敏错误码和用户可读的脱敏消息，不保存凭证或原始上游响应。

## WorkflowRun

| 字段 | 草案类型 | 说明 |
|---|---|---|
| `id` | uuid | 主键 |
| `run_number` | varchar | 用户可搜索、展示的运行编号 |
| `project_id` | uuid | 归属项目 |
| `stage` | varchar(40) | 项目环节 |
| `workflow_configuration_id` | uuid | 创建时使用的工作流配置 |
| `trigger_source` | varchar(40) | 项目侧、流程中心或重试等触发来源 |
| `status` | varchar(30) | `queued`、`running`、`succeeded`、`failed`、`cancelled` |
| `binding_snapshot` | jsonb | 不含密钥的项目绑定、连接和配置快照 |
| `input_payload` | jsonb | Schema 校验后的输入 |
| `output_payload` | jsonb | 成功且经输出校验后的安全输出 |
| `error_code` | varchar(80) | 脱敏错误码 |
| `error_message` | varchar(300) | 脱敏错误消息 |
| `error_details` | jsonb | 脱敏、允许展示的错误详情 |
| `retry_of_run_id` | uuid nullable | 完整重试所关联的原运行 |
| `idempotency_key` | varchar(160) | 创建命令幂等键 |
| `started_at` | timestamptz nullable | Worker 开始时间 |
| `finished_at` | timestamptz nullable | 结束时间 |
| `cancelled_at` | timestamptz nullable | 取消时间 |
| `version` | integer | 乐观并发控制 |
| `created_at` / `updated_at` | timestamptz | 审计时间 |

快照包含项目、环节、绑定版本、工作流配置标识和版本、连接类型与非敏感配置、契约版本、默认参数及创建时间；不得包含明文或密文密钥、Authorization Header、原始上游错误响应。

## WorkflowRunEvent

`WorkflowRunEvent` 保存不可变时间线：`id`、`run_id`、`sequence`、`event_type`、`safe_payload`、`created_at`。事件包括 queued、worker_started、request_sent、response_received、output_validated、succeeded、failed、cancelled 和 retry_created；`safe_payload` 必须脱敏。

## 状态机、重试与查询聚合

- 合法主路径：`queued → running → succeeded`、`queued → running → failed`；在允许取消的边界可进入 `cancelled`，并记录 `cancelled_at`。
- 完整重试始终创建新的 `WorkflowRun`，通过 `retry_of_run_id` 关联；原运行记录、快照和事件不可修改。
- 项目运行摘要是对 WorkflowRun 的查询聚合，不新增重复业务表；提供总运行次数、运行中、最近失败、最近运行和最近运行列表。
- 数据库验收只要求当前 Iteration 的最终状态正确，不要求历史版本 Migration 回滚。
