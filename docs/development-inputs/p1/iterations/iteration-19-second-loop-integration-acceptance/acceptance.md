# Iteration 19 — 验收标准

## 1. 文档与 UI 冻结验收（已完成）

- [x] 15 个正式 Frame 已归档，`04_n8n_2` 未进入正式 Manifest；
- [x] 此前 6 个产品决策均有 UI 证据；
- [x] WorkflowRun 使用独立详情页；
- [x] `/workflows` 保留只读并引导 `/workflow-runs`；
- [x] 四阶段共享 Preflight 和运行恢复组件；
- [x] AppShell、导航和 Iteration 15～18 主体布局不变；
- [x] UI、API Scope、数据模型和追踪文档已更新；
- [x] 强制开发修正已记录，不要求再次生成 Stitch。

## 2. 契约同步门禁

- [ ] 主 OpenAPI 与 `api-scope.yaml` 一致；
- [ ] 每个 Operation 有唯一 operationId、路径、状态码和 ErrorEnvelope；
- [ ] 后端 DTO、前端类型由冻结 OpenAPI 对齐；
- [ ] Migration 19 只做最小向前变更，不修改历史 Migration；
- [ ] `llmStrategy`、验证状态、Run 状态、失败阶段和 RetryMode 枚举唯一；
- [ ] 不存在项目级 Provider/模型覆盖字段。

## 3. LLM Provider

- [ ] 可保存、发现模型、读取模型目录、验证、启用和停用；
- [ ] 正确凭据成功，错误凭据返回安全错误；
- [ ] 默认模型必须存在且可用；消失后不自动切换；
- [ ] 关键字段变化转 `stale`，enabled 保留，executable=false；
- [ ] API Key 不回显、不写日志、不进入快照或错误；
- [ ] 超时、网络、认证、模型不存在、非法 Base URL 均有稳定错误码。

## 4. n8n Connection

- [ ] 验证实例根 Base URL、认证和兼容性；
- [ ] Workflow ID/Webhook Path 不保存在 Connection；
- [ ] 可启用和停用；
- [ ] 关键字段变化转 `stale`，依赖关系保留；
- [ ] Credential 和原始敏感响应不返回前端；
- [ ] 停用确认能返回受影响 Workflow/Binding 数量但不级联删除。

## 5. Workflow Configuration

- [ ] 支持 `acf_managed/n8n_managed/none` 三种且仅一种策略；
- [ ] ACF-managed 必须选择可执行 Provider 和模型；
- [ ] 项目不得覆盖策略；
- [ ] Connection、引用、Stage、输入、输出、LLM 策略分层验证；
- [ ] 关键字段变化更新同一 Workflow Configuration 记录、执行 `version + 1` 并使当前验证转 `stale`；不创建配置历史表；
- [ ] enabled 保留但 executable=false，重新验证后恢复；
- [ ] 未验证或依赖失效时不可成为新绑定候选或新 Run 依赖。

## 6. 项目绑定与 Preflight

- [ ] 四个 Stage 分别绑定且共用一套交互结构；
- [ ] 只允许选择 Stage 匹配、已验证、已启用、依赖完整的候选；
- [ ] 已绑定工作流后续失效时不自动解绑；
- [ ] 返回绑定状态、执行资格、依赖摘要和精准原因；
- [ ] 四阶段运行前检查失败时不创建 Run；
- [ ] 修复依赖后无需重新绑定即可恢复。

## 7. WorkflowRun

- [ ] Runtime 生命周期支持 queued/running/cancelling/succeeded/failed/cancelled/timed_out；
- [ ] displayStatus 支持 output_validation_failed/result_consumption_failed；
- [ ] 独立详情页显示时间线、安全配置快照、外部执行 ID、领域影响和恢复动作；
- [ ] 列表支持项目、Stage、状态、时间和高级配置筛选；
- [ ] 刷新后恢复；取消和重试具备乐观锁与幂等；
- [ ] 失败阶段、错误码、可重试性与用户文案稳定；
- [ ] 结果消费重试不调用 n8n/LLM，不创建新 Runtime Run。

## 8. 配置重试

- [ ] 当前配置重试重新检查当前依赖；
- [ ] 原配置重试只有完整可重放时启用；
- [ ] 凭据指纹变化后原配置重试禁用；
- [ ] 新 Run 设置 retryOfRunId，原 Run 保留；
- [ ] 幂等重放不创建多个 Run 或多次领域结果。

## 9. 四 Stage 真实闭环

- [ ] 章节规划真实创建候选批次；
- [ ] 正文生成真实创建 ContentVersion；
- [ ] 审核真实创建 Report/Findings/Recommendations；
- [ ] 重写真实创建新 ContentVersion；
- [ ] 四阶段最终验收不使用 Mock Adapter；
- [ ] 输出校验失败和领域提交失败均满足零部分数据；
- [ ] 第二用户闭环人工验收 PASS。

## 10. 工程和安全

- [ ] 单元、Repository、API、前端测试、E2E 和安全测试通过；
- [ ] Migration History、Schema、Data consistency 全部 PASS；
- [ ] 无静默 Skip；
- [ ] SSRF、重定向、DNS 重绑定、证书、超时和凭据脱敏规则通过；
- [ ] AppShell、导航、中文 locale 和 UI 人工验收通过；
- [ ] 独立 Code Review 完成；
- [ ] Git 状态 clean，报告和证据完整。

## 11. 任务级阶段门禁

本节是迭代级汇总。具体执行拆分可以调整，但不得降低、替换或绕过本文件定义的验收结果。

- [ ] Task 02 完成后：主 OpenAPI 是唯一 HTTP 事实来源，兼容检查和生成物无漂移；
- [ ] Task 03 完成后：Migration 19、Repository、Fixture、Schema/Data consistency 全部 PASS；
- [ ] Task 04 完成后：统一状态机、执行资格、安全外呼和 Secret 语义具备独立测试；
- [ ] Task 05 完成后：Provider→Connection→Workflow Configuration→Binding 配置闭环可验证、可启停、可恢复；
- [ ] Task 06 完成后：WorkflowRun 状态机、快照、真实 n8n 调用、取消、超时和重试完整；
- [ ] Task 07 完成后：15 Frame 对应 UI 接入真实 API，前端 typecheck/lint/test/build PASS；
- [ ] Task 08 完成后：四 Stage 正式路径均不调用 Mock Adapter，领域写入保持原子性；
- [ ] Task 09 完成后：真实 LLM、真实 n8n、四 Stage E2E、安全、数据库、人工 UI 和独立 Review 全部 PASS。
