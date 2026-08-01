本任务为执行任务，直接执行当前任务。

# CF-19-09 — 真实联调与最终验收

- 任务书版本：`v2`
- 冻结来源：Task 01 Commit `39fa4f3c4838c5d67b98c7958ba2f7a42ece75b7`
- 详细验收标准：本文件“七、验收标准”

- 推荐模型：GPT-5.6 Sol
- 推理等级：high
- 执行环境：Windows PowerShell
- 仓库：`D:\github\ai-content-factory2`
- 分支：`feature/second-user-loop`
- 前置状态：Task 08 四 Stage 正式代码路径和自动测试全部 PASS。

## 一、任务目标

使用真实 OpenAI-compatible Provider、真实本地 n8n 和唯一开发数据库完成全链路 E2E、失败矩阵、安全测试、15 Frame 人工验收、独立 Review 和 Iteration 19 关闭。

## 二、强制边界

- 禁止重新制定 Iteration 19 计划。
- 禁止重新评估已冻结业务方案、UI 和产品决策。
- 禁止执行本任务之后的任务。
- 禁止修改未授权模块或顺带重构。
- 禁止创建平行领域模型、重复接口或第二套状态机。
- 禁止 amend、rebase、squash、force push。
- 禁止使用 `git add .`、`git add -A` 或 `git commit -a`。
- 真实凭据不得进入仓库、日志、报告、截图或 Commit。

## 三、执行前检查

执行：

```powershell
git branch --show-current
git rev-parse HEAD
git status --short --untracked-files=all
git diff --name-status
git diff --cached --name-only
git stash list
```

- 分支必须正确。
- 工作区和暂存区必须 clean。
- Stash 必须为空。
- 读取 `AGENTS.md` 和所有适用目录下的 `AGENTS.md`。
- 前置 Commit 必须已推送到 `origin/feature/second-user-loop`。

## 四、冻结输入

- `docs/development-inputs/p1/iterations/iteration-19-second-loop-integration-acceptance/iteration-plan.md`
- `docs/development-inputs/p1/iterations/iteration-19-second-loop-integration-acceptance/execution-plan.md`
- `docs/development-inputs/p1/iterations/iteration-19-second-loop-integration-acceptance/api-scope.yaml`
- `docs/development-inputs/p1/iterations/iteration-19-second-loop-integration-acceptance/data-model.md`
- `docs/development-inputs/p1/iterations/iteration-19-second-loop-integration-acceptance/closed-loop.md`
- `docs/development-inputs/p1/iterations/iteration-19-second-loop-integration-acceptance/acceptance.md`
- `compose.yml`
- `compose.n8n.yml`
- `infra/n8n/workflows/`
- `scripts/`
- `database/tools/Validate-Database-Consistency.ps1`
- `.ai-dev/state.json`

这些文件是当前任务的强制输入。发生冲突时先按主 OpenAPI/Schema 优先级定位；只有真正互斥且无法兼容时才允许 BLOCKED。

## 五、文件修改逻辑

### `infra/n8n/workflows/**`

- 冻结四个确定性测试 Workflow 或复用已有 Workflow；输入输出必须严格符合主 OpenAPI 和 Stage Schema。
- Workflow 可调用真实 Provider 或按 llmStrategy 接收 ACF 注入结果，但不得绕过策略。

### `scripts/setup-local-n8n-workflow.ps1 及现有 n8n 设置脚本`

- 导入/激活 Workflow、输出 ID、验证可访问性；保持幂等，可在新电脑重复运行。
- 如已有分 Stage 脚本，在原脚本扩展，不创建重复初始化体系。

### `scripts/verify-local-n8n-integration.ps1 或现有联调脚本`

- 验证 Provider→Connection→Workflow Configuration→Binding→Runtime→领域结果链路。
- 输出安全证据，不打印 Secret、Credential 或完整上游响应。

### `scripts/browser-acceptance/**`

- 按 15 Frame 和真实 API 更新浏览器验收；保持现有 AppShell 和路由。
- 覆盖 Provider/Connection/Workflow、Binding 两态、流程中心、独立详情、重试和共享状态。

### `apps/api/apps/web 测试文件`

- 只补最终真实联调暴露的缺口；不得借验收阶段重构架构。

### `database/testdata 与 dbcheck`

- 只有真实联调需要新的确定性 Fixture/检查时最小更新；Migration History 必须不变。

### `docs/development-inputs/.../acceptance.md 与 development-readiness.md`

- 仅在所有门禁和人工验收 PASS 后勾选实际通过项并记录证据。

### `.ai-dev/state.json`

- 仅最终 PASS 时更新 Iteration 19 completed/next_iteration；失败或未完成时不得提前关闭。

## 六、实施顺序

1. 在新电脑按数据库初始化指南恢复唯一开发数据库和服务。
2. 启动真实 n8n，导入并激活确定性四 Stage Workflow。
3. 配置真实 OpenAI-compatible Provider、模型、Connection 和 Workflow Configuration。
4. 依次完成四 Stage 成功 E2E。
5. 执行认证失败、Provider/Connection stale、Workflow 无效、输出非法、消费失败、取消、超时、当前/原配置重试矩阵。
6. 执行安全测试、数据库门禁、后端/前端全门禁和浏览器人工验收。
7. 进行独立全量 Code Review，修复 P0/P1，再重复受影响门禁。
8. 最后更新验收文档和 state，提交关闭证据。

## 七、验收标准

### 7.1 真实环境证明

- 最终验收使用真实 OpenAI-compatible Provider 和真实本地 n8n，不得使用 Mock Adapter、测试 Fake 或静态伪响应替代。
- Provider、模型、Connection、四个 Workflow Configuration 和四个项目绑定均完成验证并处于可执行状态。
- n8n 四个确定性 Workflow 已导入、激活，可通过稳定 Workflow ID 或引用重复初始化。
- 所有真实凭据只存在于本地安全配置或环境变量，不进入仓库、日志、截图、报告或 Commit。

### 7.2 配置与绑定闭环

- Provider 模型发现、验证、启用、模型失效和重新验证链路通过人工及自动验证。
- Connection 验证、认证失败、stale、停用和恢复链路通过。
- Workflow Configuration 三种 LLM 策略至少各完成一条确定性验证；四 Stage 使用与冻结方案一致的策略。
- 修改 Provider、Connection 或 Workflow Configuration 关键字段后，Binding 保留但立即不可执行；修复并重新验证后自动恢复。
- 不可执行候选在绑定抽屉可见但不可选择，原因与修复入口准确。

### 7.3 四 Stage 成功 E2E

- 章节规划成功生成候选并可采用，Run、外部 Execution ID、Candidate Batch 和 Revision 可追踪。
- 正文生成成功创建新 ContentVersion，来源链、版本号和当前版本操作符合领域规则。
- 内容审核成功创建 ReviewReport、Findings 和 Recommendations，固定源版本正确。
- 正文重写成功创建新版本或候选结果，不自动 Set Current，来源问题和源版本可追踪。
- 四 Stage 串联后形成真实第二闭环，正式路径中无 Mock 调用。

### 7.4 失败与恢复矩阵

- 至少验证：
  - Provider 认证失败；
  - Provider 模型不可用；
  - Connection 认证失败；
  - Connection/Workflow stale；
  - Workflow 引用无效；
  - Stage 或输入/输出契约不匹配；
  - 外部执行失败；
  - 输出校验失败；
  - 结果消费失败；
  - 用户取消；
  - 执行超时；
  - 当前配置重试；
  - 原配置可重放和不可重放。
- 每种失败在 UI、API、WorkflowRun Event、数据库和恢复动作中表现一致。
- 输出校验失败与取消/超时不得产生领域结果。
- 结果消费失败通过专用“重试提交结果”恢复，不重新调用 n8n/LLM。
- 重试创建新 Run 并保留完整 retry 链，原失败 Run 不被覆盖。

### 7.5 安全验收

- SSRF、IPv4/IPv6 私网、link-local、metadata、DNS rebinding、重定向、TLS、超时和响应大小测试全部 PASS。
- API Key、Credential、Authorization Header、Cookie、Idempotency-Key 和第三方原始响应不会出现在：
  - API 读取响应；
  - WorkflowRun 快照；
  - Event；
  - 应用日志；
  - 浏览器页面；
  - 测试报告；
  - 截图和提交差异。
- 日志和错误只保留安全、可行动摘要。
- 原配置重试不能通过快照恢复明文凭据；只允许受控引用和 fingerprint 验证。

### 7.6 稳定性与恢复

- API、Worker、n8n 或浏览器刷新/重启后，queued/running/cancelling Run 能正确恢复。
- 重复创建、验证、启用、取消、重试和消费重试均保持幂等。
- 并发取消与成功、超时与失败、配置更新与验证完成只产生一个最终事实。
- 连续完成至少两轮四 Stage 成功闭环后，数据库无重复、孤儿、状态或时间异常。
- 不要求未经冻结的性能指标，但不得出现明显无限轮询、无界重试或资源泄漏。

### 7.7 数据库与自动门禁

- Migration History PASS，数据库版本为 19、`dirty=false`。
- 全部 Schema consistency、Data consistency 和 Iteration 19 新检查 PASS，异常数全部为 0。
- 后端全量测试、前端 typecheck/lint/unit/build/E2E、OpenAPI 验证全部 PASS，`SKIP=0`。
- 初始化脚本可在新电脑重复执行并恢复确定性验收环境。
- 所有测试完成后工作区不包含临时凭据、日志、浏览器产物或未跟踪敏感文件。

### 7.8 UI 与人工验收

- 15 个冻结 Frame 逐项人工验收 PASS。
- AppShell、导航、中文语言、路由和 WorkflowRun 独立详情页保持冻结形态。
- 02、04、06 抽屉滚动和固定底部操作正常。
- 列表常用/高级筛选、Binding 正常/失效两态、Preflight 阻断、Runtime 恢复和重试弹窗均可实际操作。
- `/workflows` 只提供遗留只读说明和真实流程中心入口，不展示运行上下文。
- 页面无敏感信息、原始枚举、不可滚动区域或操作遮挡。

### 7.9 独立 Review 与关闭

- 对最终源码执行独立全量 Code Review，覆盖契约、Migration、状态机、并发、事务、SSRF、凭据、四 Stage 原子消费和前端安全。
- P0/P1 问题全部修复并重新执行受影响门禁；P2 非阻断项记录到问题台账。
- 只有真实 E2E、自动门禁、数据库、安全、UI 人工验收和 Review 全部 PASS 后，才允许更新 `acceptance.md`、`development-readiness.md` 和 `.ai-dev/state.json`。
- 最终 Commit 只包含实际验收脚本、必要修复、验收证据和状态更新，不包含真实凭据。
- Commit、Push 成功，远端分支包含最终提交，`git status --short` 无输出。

## 八、验证要求

- 真实 Provider 和真实 n8n 证据明确，最终验收不得使用 Mock Adapter。
- 四 Stage 成功结果与数据库领域记录可追踪到 WorkflowRun 和外部 Execution ID。
- 失败矩阵、取消、超时、输出校验、消费重试和两种配置重试符合契约。
- SSRF、DNS 重绑定、重定向、TLS、超时、响应大小、凭据和日志脱敏测试 PASS。
- Migration History、全部 Schema/Data consistency PASS，异常数 0。
- 后端全测、前端 typecheck/lint/test/build/E2E PASS，SKIP=0。
- 15 Frame 人工验收 PASS，独立 Review 完成，工作区 clean。

验证失败时先运行最小目标测试定位根因，再运行受影响包测试，最后运行完整当前任务门禁；禁止未定位根因前重复完整重跑。

## 九、差异与暂存

执行：

```powershell
git status --short
git diff --name-status
git diff --stat
git diff --check
```

- 只允许本任务授权文件。
- 逐个 `git add -- <file>` 精确暂存。
- `git diff --cached --check` 必须 PASS。

## 十、提交与推送

固定 Commit message：`test: close iteration 19 real integration loop`

```powershell
git commit -m "test: close iteration 19 real integration loop"
git push origin feature/second-user-loop
```

提交后再次执行当前任务核心验证，确认生成物无漂移且工作区 clean。

## 十一、最终回执

最终状态只能是 PASS 或 BLOCKED。

PASS 必须包含：

- 最终 Commit SHA 和 Commit message；
- Push 结果；
- 实际修改文件列表；
- 每项契约/数据/业务能力的实现摘要；
- 目标测试、包测试和完整门禁结果；
- `git diff --check`；
- 工作区 clean；
- 下一任务：Iteration 20 规划或用户指定的下一阶段。

BLOCKED 仅允许用于：冻结契约真正互斥、必须扩大授权、外部依赖不可恢复、需要破坏性操作或必须修改上游冻结方案。普通代码、SQL、类型、测试、格式、路径、暂存和网络问题必须自行修复到 PASS。
