# Iteration 19 — UI/API/数据模型追踪

| Frame | 用户动作/状态 | API | 数据事实 | 强制边界 |
|---|---|---|---|---|
| I19_01 | 列表/筛选/查看执行资格 | listLlmProviders | Provider 状态、模型数量、默认模型、lastVerifiedVersion | enabled 与 executable 分离 |
| I19_02 | 保存、发现模型、验证、启停 | create/update、discover/list models、verify/enable/disable Provider | Provider version、secret fingerprint、llm_provider_models | API Key 不回显；模型消失不自动切换 |
| I19_03 | 连接列表/依赖摘要 | listWorkflowConnections | Connection 状态、依赖派生计数 | 不保存 Workflow 引用 |
| I19_04 | 保存、验证、启停 | create/update、verify/enable/disable Connection | credential fingerprint、lastVerifiedVersion | Base URL 为实例根；停用不删除依赖 |
| I19_05 | Workflow 列表/资格筛选 | listWorkflowConfigurations | llmStrategy、Provider/model、version、执行资格 | UUID 次级展示；资格服务端派生 |
| I19_06 | 配置、分层验证、启用 | create/update/verify/enable/disable Workflow | 策略 CHECK、契约、validationDetails | 项目不可覆盖；关键字段变化转 stale |
| I19_07 | 四 Stage 全部可执行 | listProjectWorkflowBindings | Binding + 依赖派生摘要 | bound 不等于 executable |
| I19_08 | 依赖失效与修复入口 | listProjectWorkflowBindings | ineligibilityReasons | 不自动解绑，不影响历史 Run |
| I19_09 | 选择/更换候选 | listWorkflowConfigurations + putProjectWorkflowBinding | Stage 匹配、Binding version | 不可执行项禁止选择；事务时再次复核 |
| I19_10 | 列表、筛选、取消/重试入口 | listWorkflowRuns | displayStatus、配置摘要、retryOf | 高级筛选不改变 API 能力 |
| I19_11 | 时间线、诊断、安全快照 | getWorkflowRunDetail + events + retry-options | failurePhase、domainImpact、snapshots、externalExecutionId | 独立页；敏感数据禁止返回 |
| I19_12 | 当前/原配置重试 | getRetryOptions + retryWorkflowRun | retryMode、retryOf、fingerprint replayability | 原配置不可重放时禁用 |
| I19_13 | 跳转真实流程中心 | 无新增命令 | 无数据变化 | 只读 Mock 页面，不创建 Run |
| I19_14 | 运行前置检查 | Iteration 15～18 既有 Preflight | current eligibility、契约和绑定 | 失败零 Run、零领域写入 |
| I19_15 | 取消/超时/失败恢复 | cancel、runtime retry、stage consumption retry | cancelling/timed_out、failurePhase、retryability | consumption retry 不调用 Runtime |

## 统一来源

- HTTP 字段与错误：主 OpenAPI；
- 逻辑持久化：`data-model.md`；
- 交互与页面形态：`ui-scope.md` + Frame；
- 业务优先级和恢复：`closed-loop.md`；
- 不一致时必须在开发前修正文档和主 OpenAPI，禁止代码自行选择解释。
