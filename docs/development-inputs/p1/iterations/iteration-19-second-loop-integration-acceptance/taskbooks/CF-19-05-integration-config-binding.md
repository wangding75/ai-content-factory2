本任务为执行任务，直接执行当前任务。

# CF-19-05 — 集成配置与项目绑定闭环

- 任务书版本：`v2`
- 冻结来源：Task 01 Commit `39fa4f3c4838c5d67b98c7958ba2f7a42ece75b7`
- 详细验收标准：本文件“七、验收标准”

- 推荐模型：GPT-5.6 Terra
- 推理等级：medium
- 执行环境：Windows PowerShell
- 仓库：`D:\github\ai-content-factory2`
- 分支：`feature/second-user-loop`
- 前置状态：Task 04 公共验证、安全外呼和执行资格基础 PASS。

## 一、任务目标

在现有 `globalconfig` 与 `workflowbinding` 模块完成 LLM Provider、n8n Connection、Workflow Configuration 和四 Stage 项目绑定的完整配置闭环。

## 二、强制边界

- 禁止重新制定 Iteration 19 计划。
- 禁止重新评估已冻结业务方案、UI 和产品决策。
- 禁止执行本任务之后的任务。
- 禁止修改未授权模块或顺带重构。
- 禁止创建平行领域模型、重复接口或第二套状态机。
- 禁止 amend、rebase、squash、force push。
- 禁止使用 `git add .`、`git add -A` 或 `git commit -a`。
- 禁止修改 apps/web 和四 Stage 领域消费逻辑。

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
- `packages/contracts/openapi/openapi.yaml`
- `apps/api/internal/globalconfig/service.go`
- `apps/api/internal/workflowbinding/`

这些文件是当前任务的强制输入。发生冲突时先按主 OpenAPI/Schema 优先级定位；只有真正互斥且无法兼容时才允许 BLOCKED。

## 五、文件修改逻辑

### `apps/api/internal/globalconfig/service.go`

- 实现 LLM Provider CRUD 扩展、模型发现、默认模型校验、Verify、Enable/Disable；模型消失标记 unavailable，不自动切换。
- 实现 n8n Connection Verify、Enable/Disable、依赖数量和安全错误；Connection 只保存实例根 URL，不保存 Workflow ID/Webhook。
- 实现 Workflow Configuration 三种 LLM 策略、六类分层验证、同记录 version+1、Enable/Disable。
- 所有外部调用复用 Task 04 Safe HTTP/Secret 基础，不另建 Client 安全逻辑。

### `apps/api/internal/globalconfig/service_test.go、service_integration_test.go`

- 覆盖正确/错误凭据、模型列表为空/消失、默认模型无效、认证失败、Workflow 不存在/未激活、Stage/输入/输出/策略不兼容。
- 覆盖保存≠验证、验证≠启用、stale 保留 enabled、重新验证恢复 executable。

### `apps/api/internal/workflowbinding/domain.go`

- 保留四个 Stage 唯一枚举；增加 Binding 读取所需执行资格和依赖摘要领域类型。

### `apps/api/internal/workflowbinding/adapters.go`

- 接入 globalconfig 当前配置、Connection、Provider/模型执行资格读取，不复制验证逻辑。

### `apps/api/internal/workflowbinding/readmodel.go、dto.go、json.go`

- 返回 bound、executable、配置版本、Connection/LLM 摘要、lastVerifiedAt 和精准不可执行原因。
- 候选列表返回 selectable；不可执行候选可见但不可选择。

### `apps/api/internal/workflowbinding/service.go、closeloop.go`

- PUT Binding 事务内锁定并复核 Stage 与 executable；失效后不自动解绑。
- 依赖修复后读取时自动恢复 executable，无需重绑。

### `apps/api/internal/workflowbinding/repository.go`

- 只持久化既有 Binding 事实；不得新增 provider/model/strategy 覆盖字段。

### `apps/api 路由/Handler 实际文件`

- 按主 OpenAPI 接入 models/discover、verify、enable、disable 和 Binding 扩展；复用共享 ErrorEnvelope、Idempotency 和 expectedVersion。
- 先通过 rg 定位既有 Provider/Connection/Workflow Handler，禁止创建平行路由层。

## 六、实施顺序

1. 先完成 LLM Provider 闭环并局部验证。
2. 完成 n8n Connection 闭环并局部验证。
3. 完成 Workflow Configuration 策略和分层验证。
4. 最后接入 Binding 读取、候选和事务复核。
5. 运行 Provider→Connection→Workflow→Binding 联合集成测试。

## 七、验收标准

### 7.1 LLM Provider 闭环

- Provider 可创建、更新、发现模型、验证、启用和停用，且全部复用 Task 04 公共状态机和安全 HTTP。
- 模型发现成功后模型目录按 Provider 唯一键更新，不产生重复模型行。
- 默认模型必须来自当前可用模型目录；模型消失或不可用时不得自动切换到其他模型。
- Provider 验证成功只更新验证事实，不自动启用。
- Provider 未验证、验证失败、stale、已停用或默认模型不可用时 `executable=false`，并返回精确修复原因。
- API Key 在任何读取响应、日志和错误中均不可见。

### 7.2 n8n Connection 闭环

- Connection 可创建、更新、真实 Verify、启用和停用。
- Verify 至少能区分网络不可达、认证失败、服务不兼容、超时和安全 URL 拒绝。
- 修改 Base URL、认证或其他关键字段后转为 `stale`；保留 enabled、Workflow Configuration 和项目绑定。
- 停用 Connection 不级联删除 Workflow Configuration、Binding 或历史 Run。
- Connection 响应可返回依赖 Workflow Configuration 数量和受影响绑定数量，但不返回 Credential。
- 重新验证并恢复后，依赖执行资格可自动重新计算。

### 7.3 Workflow Configuration 闭环

- 三种 LLM 策略均可创建和更新，字段组合严格符合 OpenAPI 和数据库约束。
- Workflow 验证必须产生 Connection、Workflow 引用、Stage、输入契约、输出契约和 LLM 策略六项结果。
- 任一必需检查失败时不能启用；启用前必须在事务内重新确认版本和依赖状态。
- 更新关键字段使用同一记录 `version + 1`，验证转为 `stale`，不创建配置历史记录。
- `acf_managed` 必须引用已启用、已验证且模型可用的 Provider；`n8n_managed` 和 `none` 不接受项目级覆盖。
- Workflow ID/Webhook 引用无效或未激活时返回结构化错误和修复动作。

### 7.4 项目绑定与候选

- 四个 Stage 均可绑定、替换和解绑 Workflow Configuration，Stage 不匹配时必须拒绝。
- Binding 保存后 `bound=true`；依赖失效时仍保持 `bound=true`，但 `executable=false`。
- Binding 响应包含 Workflow Configuration 版本、Connection、LLM 策略、Provider/模型摘要、验证时间和不可执行原因。
- 候选列表只允许选择 Stage 匹配且当前 executable 的配置。
- 不可执行候选仍可见，但 `selectable=false`，并返回具体不可选择原因。
- 项目层不得写入或覆盖 Provider、模型和 LLM 策略。

### 7.5 依赖失效与恢复

- Provider、Connection 或 Workflow Configuration 修改关键字段后，所有受影响 Binding 立即反映不可执行，不自动解绑。
- Provider 模型不可用、Connection stale、Workflow 验证失败和策略不完整均能定位到准确依赖节点。
- 修复依赖并重新验证后，Binding 无需重新保存即可恢复 executable。
- 历史 WorkflowRun 及其快照不受当前配置失效影响。
- 删除或停用操作的影响范围符合冻结契约，不产生孤儿引用。

### 7.6 API、并发与错误语义

- 所有 Handler 复用主 OpenAPI Path、operationId、ErrorEnvelope、Idempotency-Key 和 expectedVersion。
- 版本冲突、重复命令、依赖失效和验证失败返回冻结错误代码。
- 并发验证、更新和启用不能导致旧结果覆盖新版本。
- 同一幂等请求重复调用不产生重复模型、验证事实、绑定或审计。
- 安全错误只返回可行动摘要，不返回第三方原始响应。

### 7.7 测试与范围

- Provider、Connection、Workflow Configuration、Binding 的单元、Repository、API 和 PostgreSQL 集成测试全部 PASS，`SKIP=0`。
- 必须覆盖成功、认证失败、模型不存在、stale、停用、版本竞争、策略非法、Stage 不匹配和依赖恢复。
- 数据库一致性门禁全部 PASS，测试结束不存在模型孤儿、非法策略、失效绑定或版本异常。
- 本任务不实现 WorkflowRun Worker、四 Stage 领域消费和前端。
- `git diff --check`、精确暂存、Commit、Push 和最终工作区 clean 全部 PASS。

## 八、验证要求

- globalconfig 全部单元/集成/API 测试 PASS，PostgreSQL SKIP=0。
- workflowbinding 单元/Repository/API/闭环测试 PASS。
- 验证 stale/disabled/model unavailable/strategy incomplete 的原因和修复入口稳定。
- 确认项目绑定表无策略覆盖字段，依赖失效时 Binding 记录仍存在。
- 受影响包完整测试、OpenAPI 验证和数据库一致性门禁 PASS。

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

固定 Commit message：`feat: complete integration configuration lifecycle`

```powershell
git commit -m "feat: complete integration configuration lifecycle"
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
- 下一任务：Task 06 — Workflow Runtime 与失败恢复。

BLOCKED 仅允许用于：冻结契约真正互斥、必须扩大授权、外部依赖不可恢复、需要破坏性操作或必须修改上游冻结方案。普通代码、SQL、类型、测试、格式、路径、暂存和网络问题必须自行修复到 PASS。
