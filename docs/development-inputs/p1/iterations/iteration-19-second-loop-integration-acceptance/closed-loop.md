# Iteration 19 — 真实接入与第二用户闭环 — Closed Loop

## 1. 配置状态模型

Provider、Connection 和 Workflow Configuration 共用以下验证状态：

| 状态 | 含义 | 新运行资格 |
|---|---|---|
| `unverified` | 已保存但从未验证 | 无 |
| `verifying` | 外部验证进行中 | 无 |
| `verified` | 当前记录版本验证通过 | 还需 enabled 与依赖通过 |
| `failed` | 当前版本验证失败 | 无 |
| `stale` | 验证通过后关键字段被修改 | 无；enabled 和绑定保留 |

保存不等于验证，验证不等于启用。`executable` 由当前版本、enabled 和依赖链实时派生。

## 2. LLM Provider 闭环

```text
新增/编辑 OpenAI-compatible Provider
→ 保存凭据（前端只见已安全保存）
→ 获取模型目录或手动校验模型
→ 选择默认模型
→ 验证 Base URL、凭据、超时和默认模型
→ 启用
→ 成为 ACF-managed 工作流可选择依赖
```

- API Key 不回显；PATCH 未提交新密钥表示保留旧密钥。
- Base URL、密钥、超时、默认模型和模型参数变化使验证转 `stale`，但不自动停用。
- 默认模型消失或校验失败时，状态为 `failed/model_unavailable`；不得自动切换模型。
- 模型目录记录可用/不可用历史；运行快照只保存模型标识和安全 Provider 元数据。

## 3. n8n Connection 闭环

```text
新增/编辑 n8n Connection
→ 保存实例根 Base URL、认证方式与凭据
→ 验证服务、认证和兼容性
→ 启用
→ 成为 Workflow Configuration 可选择连接
```

- Connection Base URL 是实例根地址；Workflow ID/Webhook Path 只存在 Workflow Configuration。
- 修改 Base URL、认证、凭据或超时后转 `stale`，绑定和启用保留，新运行阻断。
- 手动停用不删除 Workflow Configuration、项目绑定或历史 Run。

## 4. Workflow Configuration 闭环

```text
选择已验证并启用的 Connection
→ 选择 Stage
→ 配置 Workflow ID 或 Webhook Path
→ 配置输入/输出契约和默认参数
→ 选择唯一 LLM 策略
→ 分层验证
→ 启用
```

LLM 策略：

- `acf_managed`：必须选择可执行 Provider 和模型；
- `n8n_managed`：ACF 不注入密钥，只记录策略；
- `none`：显式声明无需模型。

验证清单：Connection、Workflow 引用、Stage、输入契约、输出契约、LLM 策略。任何必需项失败，Workflow Configuration 不可执行。

关键字段变化更新同一 Workflow Configuration 记录并执行 `version + 1`，不创建配置历史表；验证状态转 `stale`。历史运行配置由 WorkflowRun 不可变快照承担。

## 5. 项目绑定闭环

- 四个 Stage 共用同一绑定组件。
- 项目只能选择 Stage 匹配且当前可执行的 Workflow Configuration。
- 不可执行候选可以查看原因和修复入口，但不能选择。
- 项目不得覆盖 LLM 策略、Provider 或模型。
- 依赖后续失效时绑定保留，读取模型返回 `bound=true/executable=false/ineligibilityReasons`。
- 修复依赖并重新验证后自动恢复，不需要重新绑定。

## 6. 四阶段 Preflight

章节规划、正文生成、审核、重写在创建 Run 前统一检查：

1. 项目绑定存在；
2. Workflow Configuration 当前版本已验证并启用；
3. Connection 当前版本已验证并启用；
4. LLM 策略完整；
5. ACF-managed Provider、模型当前可执行；
6. 输入/输出契约与 Stage 匹配。

检查失败不创建 WorkflowRun，不写领域数据，并返回安全原因及精准修复链接。

## 7. 真实运行

每个 Run 冻结：binding、Workflow Configuration、Connection、LLM 策略、Provider/模型、契约版本和安全参数快照。密钥、Credential、Authorization Header 和原始响应不得进入快照。

生命周期：

```text
queued → running → succeeded
              ↘ failed
              ↘ timed_out
queued/running → cancelling → cancelled
```

失败阶段通过 `failurePhase` 区分：`external_execution`、`output_validation`、`result_consumption`、`cancellation`。前端 `displayStatus` 将后两类显示为“输出校验失败”和“结果消费失败”。

## 8. 失败恢复

- `external_execution` / `output_validation`：创建新的 Runtime Run 重试；
- `result_consumption`：只重试持久化输出的领域提交，不调用 n8n/LLM，不创建新 Runtime Run；
- `timed_out`：按错误码和外部执行状态决定是否可新建运行；
- `cancelling`：等待外部取消确认，禁止重复取消；
- 领域写入遵循原子事务；失败必须零写入或完整回滚。

## 9. 配置版本重试

`current_configuration`：重新计算当前绑定和依赖，当前全部可执行时可用。

`original_configuration`：仅当 Run 快照完整、对应记录仍存在、当前凭据指纹与快照一致、外部引用可安全重放时可用。系统不保存密钥历史；凭据已替换时必须禁用并返回原因。

重试创建新 Run 并设置 `retryOfRunId`；原 Run 永不覆盖。相同 Idempotency-Key 和请求指纹只产生一个结果。

## 10. 旧 `/workflows`

本轮保留 `/workflows` 作为内置 Mock 流程说明页，明确不用于真实执行，并引导到 `/workflow-runs`。不在 Iteration 19 合并或删除路由。
