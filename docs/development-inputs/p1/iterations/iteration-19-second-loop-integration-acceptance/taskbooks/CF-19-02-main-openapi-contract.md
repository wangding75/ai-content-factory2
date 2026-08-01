本任务为执行任务，直接执行当前任务。

# CF-19-02 — 主 OpenAPI 契约同步

- 任务书版本：`v2`
- 冻结来源：Task 01 Commit `39fa4f3c4838c5d67b98c7958ba2f7a42ece75b7`
- 详细验收标准：本文件“七、验收标准”

- 推荐模型：GPT-5.6 Sol
- 推理等级：high
- 执行环境：Windows PowerShell
- 仓库：`D:\github\ai-content-factory2`
- 分支：`feature/second-user-loop`
- 前置状态：Task 01 文档提交 PASS，主 OpenAPI 尚未实施 Iteration 19。

## 一、任务目标

将 `api-scope.yaml` 的全部冻结能力同步到主 OpenAPI，保持 Iteration 12～18 兼容，重新生成契约类型并冻结唯一 HTTP 事实源。

## 二、强制边界

- 禁止重新制定 Iteration 19 计划。
- 禁止重新评估已冻结业务方案、UI 和产品决策。
- 禁止执行本任务之后的任务。
- 禁止修改未授权模块或顺带重构。
- 禁止创建平行领域模型、重复接口或第二套状态机。
- 禁止 amend、rebase、squash、force push。
- 禁止使用 `git add .`、`git add -A` 或 `git commit -a`。
- 禁止修改 Iteration 19 文档和 UI 资产。

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
- `scripts/validate-openapi.ps1`

这些文件是当前任务的强制输入。发生冲突时先按主 OpenAPI/Schema 优先级定位；只有真正互斥且无法兼容时才允许 BLOCKED。

## 五、文件修改逻辑

### `packages/contracts/openapi/openapi.yaml`

- 复用现有 Provider、Connection、Workflow Configuration、Binding、WorkflowRun Path 和 operationId；禁止创建 v2 或重复资源。
- 增加统一 `ValidationStatus`、`LlmStrategy`、Runtime status、display status、failure phase、retryability、RetryMode 和结构化不可执行原因 Schema。
- 扩展 Provider DTO：模型目录、默认模型、验证版本/时间、enabled、executable、安全错误；Secret 仅请求 writeOnly，响应只返回 configured/fingerprint 安全状态。
- 扩展 Connection DTO：验证、启停、版本、执行资格、依赖数量；Credential 不回显。
- 扩展 Workflow Configuration DTO：三种 LLM 策略、Provider/模型条件字段、六类验证检查、同记录 version+1 语义。
- 扩展 Binding DTO：bound/executable、配置版本、Connection/LLM 摘要、不可执行原因；项目层不接受策略覆盖。
- 扩展 WorkflowRun：cancelling/timed_out、failurePhase、safeError、retryability、retryOfRunId、retryMode、externalExecutionId、安全快照和高级筛选。
- 新增/扩展模型发现、Verify、Enable、Disable、retry-options、retries、cancel Operation；Result consumption retry 继续使用 Stage 专用接口。
- 所有写命令沿用 Idempotency-Key、expectedVersion、409 版本冲突和共享 ErrorEnvelope。

### `scripts/validate-openapi.ps1`

- 仅在现有验证器无法覆盖新契约时做最小扩展。
- 新增枚举唯一性、operationId 唯一性、敏感响应字段、必需 Path/Schema 和引用完整性检查。
- 禁止把业务契约写死成与 OpenAPI 不同的第二套定义。

### `packages/contracts 下现有 generated 文件`

- 先读取 package.json/生成脚本确定真实生成物；只通过既有生成命令更新。
- 不得手工编辑带 generated 标记的文件。
- 第二次执行生成命令必须无新增差异。

## 六、实施顺序

1. 先建立现有 Path/operationId/Schema 兼容清单。
2. 先添加共享枚举与基础 Schema，再扩展 Provider/Connection/Workflow/Binding，最后扩展 WorkflowRun 与恢复接口。
3. 逐 Operation 校验 request/response/error/security/状态码。
4. 运行 OpenAPI 生成命令并修复所有消费类型错误，但本任务不修改 apps/api 或 apps/web 业务代码。
5. 运行兼容性检查，确认未删除既有 Path、字段、枚举和响应。

## 七、验收标准

### 7.1 契约覆盖完整性

- `api-scope.yaml` 中 Provider、Connection、Workflow Configuration、Binding、WorkflowRun、取消和重试能力全部能在主 OpenAPI 中找到唯一对应。
- 不得存在只写在 Iteration 文档、但主 OpenAPI 没有 Path、Schema 或字段承载的冻结能力。
- 新增枚举至少完整包含：
  - `ValidationStatus`：`unverified / verifying / verified / failed / stale`；
  - `LlmStrategy`：`acf_managed / n8n_managed / none`；
  - Runtime 状态：保留已有状态并补齐 `cancelling / timed_out`；
  - Runtime 失败分类：可明确区分运行失败、输出校验失败、结果消费失败；
  - `RetryMode`：`current_configuration / original_configuration`。
- `enabled`、`executable` 和结构化不可执行原因必须分离表达，客户端不得通过单一状态字段推测执行资格。

### 7.2 Provider 与 Connection 契约

- Provider 响应必须包含模型目录摘要、默认模型、验证状态、最近验证版本/时间、启用状态、执行资格和安全错误摘要。
- Provider 模型发现、验证、启用、停用均有明确 Operation，且使用既有资源 Path 和命名体系。
- Connection 响应必须包含验证状态、最近验证版本/时间、启用状态、执行资格和依赖数量。
- Provider Secret、Connection Credential 只允许作为请求写入字段；任何响应 Schema 中都不得返回明文、掩码原值、密文或 Authorization Header。
- 未提交新 Secret/Credential 时，更新请求必须能够明确表达“保留现有值”。

### 7.3 Workflow Configuration 与 Binding 契约

- Workflow Configuration 必须支持三种 LLM 策略，并用条件约束明确字段组合：
  - `acf_managed` 必须有 Provider 和模型；
  - `n8n_managed` 不接受 ACF Provider/模型覆盖；
  - `none` 不接受 Provider/模型字段。
- 验证结果必须分层返回 Connection、Workflow 引用、Stage、输入契约、输出契约和 LLM 策略六项结果。
- Workflow Configuration 更新语义必须是同一记录 `version + 1`，不得通过契约暗示创建配置历史记录。
- Binding 响应必须同时表达 `bound` 和 `executable`；依赖失效时绑定关系仍然存在。
- Binding 候选必须返回 `selectable` 和不可选择原因，禁止前端自行推导。

### 7.4 WorkflowRun 与恢复契约

- WorkflowRun 列表和详情必须包含配置、连接、LLM、模型、配置版本、外部 Execution ID、重试关系和安全快照摘要。
- 列表查询必须支持冻结文档要求的常用筛选和高级筛选，不新增重复搜索 Endpoint。
- 取消接口必须能表达接受取消、正在取消、已终态冲突和版本冲突。
- `retry-options` 必须分别返回当前配置和原配置是否可用及禁用原因。
- Runtime 重试必须新建 Run，并通过 `retryOfRunId` 和 `retryMode` 保留追踪链。
- 结果消费失败不得被定义为通用 Runtime 重试；仍使用各 Stage 专用结果消费重试接口。

### 7.5 兼容性

- Iteration 12～18 已存在的 Path、operationId、成功响应和基础 ErrorEnvelope 不得删除或改名。
- 已有字段不得删除、改类型或从可选改为必填。
- 已有枚举值不得删除；旧状态如需迁移，必须通过兼容字段或新增枚举扩展完成。
- 现有客户端和服务端生成类型必须能够继续编译；不得要求本任务修改业务代码才能掩盖破坏性契约变更。
- OpenAPI 差异检查中不得出现未经冻结文档授权的资源、字段或状态。

### 7.6 OpenAPI 质量与安全

- 所有新增 Operation 具有唯一 `operationId`、明确 tags、请求体、成功响应和实际可能出现的错误响应。
- 所有新增 Schema 具有 description、明确 required、UUID/date-time/integer 格式和有限枚举。
- 禁止使用无约束 `object`、宽泛 `additionalProperties: true` 或 `string` 代替已冻结结构。
- 敏感请求字段标记 `writeOnly`，计算字段按现有规范标记 `readOnly`。
- 错误响应继续复用共享 ErrorEnvelope，不创建第二套错误格式。

### 7.7 生成物与门禁

- OpenAPI 验证器、YAML 解析、Schema 引用、operationId 唯一性和契约测试全部 PASS。
- 使用仓库既有命令生成所有受影响类型或客户端；禁止手工编辑 generated 文件。
- 生成命令连续执行两次，第二次 `git status --short` 不产生新增差异。
- `git diff --check` 和 `git diff --cached --check` 均 PASS。
- 最终修改范围仅限主 OpenAPI、既有生成物和必要的契约验证脚本；后端、前端、Migration、冻结文档和 UI Frame 均无修改。

## 八、验证要求

- YAML 解析、OpenAPI Schema 校验、引用完整性和 operationId 唯一性 PASS。
- `scripts/validate-openapi.ps1` PASS。
- 所有生成命令 PASS，第二次生成无漂移。
- 搜索确认冻结枚举、RetryMode、failure phase、快照和安全错误全部存在。
- 搜索确认响应中不存在 apiKey、credential、authorizationHeader、cookie 或 rawResponse 明文字段。
- 确认 `apps/api/**`、`apps/web/**`、Migration 和文档未修改。

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

固定 Commit message：`feat: freeze iteration 19 api contracts`

```powershell
git commit -m "feat: freeze iteration 19 api contracts"
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
- 下一任务：Task 03 — Migration 19 与持久化模型。

BLOCKED 仅允许用于：冻结契约真正互斥、必须扩大授权、外部依赖不可恢复、需要破坏性操作或必须修改上游冻结方案。普通代码、SQL、类型、测试、格式、路径、暂存和网络问题必须自行修复到 PASS。
