# Iteration 14 — WorkflowRun 运行时 — 数据模型

## 1. 设计原则

1. 一次运行是一条不可变业务记录；
2. 完整重试创建新运行，不覆盖历史运行；
3. 项目侧摘要由运行记录聚合，不维护第二份统计真相；
4. 运行保存创建时的非敏感快照；
5. 输入、输出、错误和事件不得包含凭证明文、密文或原始 Authorization Header；
6. 当前 Iteration 只要求最终数据与环境状态正确，不要求支持回滚到历史指定版本。

## 2. WorkflowRun

建议字段：

| 字段 | 类型 | 约束/说明 |
|---|---|---|
| `id` | uuid | PK |
| `run_number` | varchar(40) | 唯一、用户可见，如 `RUN-20260722-00128` |
| `project_id` | uuid | not null，FK Project |
| `stage` | varchar(40) | `chapter_planning / content_generation / review / rewrite` |
| `binding_id` | uuid | 创建时使用的项目绑定 |
| `workflow_configuration_id` | uuid | 创建时使用的工作流配置 |
| `workflow_connection_id` | uuid | 创建时使用的连接 |
| `status` | varchar(30) | `queued / running / succeeded / failed / cancelled` |
| `trigger_source` | varchar(20) | `manual / system / api` |
| `triggered_by` | varchar(120) | 用户或系统主体标识 |
| `binding_snapshot` | jsonb | 不含敏感信息的绑定、工作流、连接快照 |
| `input_payload` | jsonb | 经输入 Schema 校验后的参数 |
| `output_payload` | jsonb | 成功后可安全保存的结构化输出 |
| `output_summary` | jsonb | UI 可直接使用的业务摘要 |
| `error_code` | varchar(80) | 归一化错误码 |
| `error_message` | varchar(300) | 脱敏、业务可理解的错误说明 |
| `retry_of_run_id` | uuid | nullable，完整重试来源 |
| `idempotency_record_id` | uuid | nullable，关联共享幂等记录，不保存 Key 明文 |
| `started_at` | timestamptz | nullable |
| `finished_at` | timestamptz | nullable |
| `cancelled_at` | timestamptz | nullable |
| `version` | integer | 乐观锁，初始 1 |
| `created_at` | timestamptz | not null |
| `updated_at` | timestamptz | not null |

### 2.1 索引与约束

- `UNIQUE(run_number)`；
- `(project_id, created_at desc)`；
- `(project_id, stage, created_at desc)`；
- `(status, created_at)`；
- `(workflow_configuration_id, created_at desc)`；
- `retry_of_run_id` 外键不级联删除历史；
- `finished_at >= started_at`；
- `succeeded/failed/cancelled` 必须有 `finished_at`；
- `cancelled` 必须有 `cancelled_at`；
- `succeeded` 不得同时存在业务失败错误码。

## 3. WorkflowRunEvent

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | uuid | PK |
| `run_id` | uuid | FK WorkflowRun |
| `sequence` | integer | 同一运行内严格递增 |
| `event_type` | varchar(60) | 事件类型 |
| `safe_payload` | jsonb | 脱敏后的事件信息 |
| `created_at` | timestamptz | not null |

约束：

- `UNIQUE(run_id, sequence)`；
- 事件追加写，不更新历史事件。

事件示例：

- `run_queued`；
- `worker_started`；
- `request_sent`；
- `response_received`；
- `output_validated`；
- `domain_commit_started`；
- `run_succeeded`；
- `run_failed`；
- `cancel_requested`；
- `run_cancelled`；
- `retry_created`；
- `domain_commit_retry_started`。

## 4. 绑定快照

`binding_snapshot` 至少保存：

- 项目 ID 和名称；
- 环节；
- Binding ID、version；
- WorkflowConfiguration ID、名称、version、类型；
- WorkflowConnection ID、名称、类型、version；
- 非敏感执行地址标识；
- 输入/输出契约版本；
- 默认参数；
- 创建运行时间。

禁止保存：

- API Key、Token、密码；
- Secret 密文；
- Authorization Header；
- 完整上游原始错误响应；
- 不必要的 PII。

## 5. 状态机

合法转换：

```text
queued → running
queued → cancelled
running → succeeded
running → failed
running → cancelled
```

禁止：

- 终态回到 running；
- 原记录被重试覆盖；
- failed 直接改为 succeeded；
- cancelled 再次执行。

完整重试：

```text
failed/cancelled → 创建新的 queued WorkflowRun
```

领域提交重试：

- 仅用于外部执行成功、领域提交失败；
- 不重新调用 n8n；
- 记录独立事件；
- 成功后原运行可推进至 `succeeded`，但必须保留完整事件轨迹。

## 6. 运行摘要

项目概览 P14_10 需要：

- 总运行次数；
- 运行中数量；
- 最近失败数量；
- 最近运行时间；
- 最近三条运行。

这些字段通过 `WorkflowRun` 聚合查询返回，不新增 `project_workflow_run_stats` 表。

统计口径：

- 总运行次数：项目所有未删除运行；
- 运行中：`queued + running`；
- 最近失败：默认最近 7 天 `failed` 数量；
- 最近运行：按 `created_at desc`；
- 最近三条运行：按 `created_at desc limit 3`。

## 7. 连接和工作流状态

沿用 Iteration 12 字段：

- `integration_status`；
- `enabled`；
- `last_verified_at`；
- `last_error_code`；
- `last_error_message`。

规则：

- Connection 只有 `verified` 才能启用；
- WorkflowConfiguration 只有自身和关联 Connection 都满足条件才可启用；
- Base URL、凭证、连接引用或类型专属配置变化后重置验证状态；
- 运行创建时冻结状态和版本，后续修改不改变历史。

## 8. Adapter 契约

通用 `WorkflowAdapter`：

- `verifyConnection`；
- `verifyWorkflow`；
- `execute`；
- `cancel`（Adapter 不支持时返回标准不可取消错误）；
- `normalizeError`。

Iteration 14 实现 `N8nWorkflowAdapter`。

凭证只在 Worker 内短暂解密，使用后释放，不写日志或数据库。

## 9. 迁移

- 激活已有连接、工作流验证和启停字段；
- 增加 `workflow_runs`；
- 增加 `workflow_run_events`；
- 增加索引、状态约束和引用关系；
- 不新增项目侧运行记录副本或统计表；
- 不修改 LLM Provider 和分发平台为已验证；
- 迁移验收只检查当前 Iteration 终态一致性，不验证历史版本回滚。
