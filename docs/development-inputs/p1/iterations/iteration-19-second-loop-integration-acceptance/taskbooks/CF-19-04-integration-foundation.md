本任务为执行任务，直接执行当前任务。

# CF-19-04 — 公共集成基础

- 任务书版本：`v2`
- 冻结来源：Task 01 Commit `39fa4f3c4838c5d67b98c7958ba2f7a42ece75b7`
- 详细验收标准：本文件“七、验收标准”

- 推荐模型：GPT-5.6 Sol
- 推理等级：high
- 执行环境：Windows PowerShell
- 仓库：`D:\github\ai-content-factory2`
- 分支：`feature/second-user-loop`
- 前置状态：Task 03 Migration 19、Repository 和数据库门禁 PASS。

## 一、任务目标

在现有 `globalconfig` 与公共基础设施中实现统一验证状态机、执行资格、安全 HTTP 外呼和 Secret/Credential 语义，为后续具体 Provider/n8n 闭环提供唯一基础。

## 二、强制边界

- 禁止重新制定 Iteration 19 计划。
- 禁止重新评估已冻结业务方案、UI 和产品决策。
- 禁止执行本任务之后的任务。
- 禁止修改未授权模块或顺带重构。
- 禁止创建平行领域模型、重复接口或第二套状态机。
- 禁止 amend、rebase、squash、force push。
- 禁止使用 `git add .`、`git add -A` 或 `git commit -a`。
- 禁止实现 WorkflowRun Runtime、四 Stage 消费和前端。

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
- `apps/api/internal/platform/`

这些文件是当前任务的强制输入。发生冲突时先按主 OpenAPI/Schema 优先级定位；只有真正互斥且无法兼容时才允许 BLOCKED。

## 五、文件修改逻辑

### `apps/api/internal/globalconfig/service.go`

- 抽取/实现 Provider、Connection、Workflow Configuration 共用的保存、stale、Verify 开始/完成、Enable/Disable 和 expectedVersion 规则。
- 验证完成必须确认目标 version 未变化；过期结果返回 409，不覆盖新配置。
- 关键字段变化保留 enabled 和引用，但清理当前验证详情并转 stale。
- PATCH 未提交新 Secret/Credential 表示保留；空字符串不能隐式清除。

### `apps/api/internal/globalconfig/validation.go（职责不存在时新增）`

- 集中定义 ValidationStatus、检查项结果、安全错误映射和合法状态转换。
- 不得复制 OpenAPI 枚举字符串到多个文件；使用单一领域常量。

### `apps/api/internal/globalconfig/eligibility.go（职责不存在时新增）`

- 实现服务端 `executable` 派生和 `ineligibilityReasons[]`。
- 原因必须包含 code/message/dependencyType/dependencyId/repairAction，顺序稳定。
- 禁止将 executable 持久化或接受客户端写入。

### `apps/api/internal/platform/safehttp 或现有 HTTP 基础文件`

- 实现只允许 http/https、禁止 loopback/link-local/private/metadata 等地址的安全校验。
- 每次 DNS 解析和重定向都重新校验；限制重定向次数、TLS、连接/响应超时、响应体大小。
- 提供可测试的 Resolver/Transport/Clock 注入点，禁止测试真实公网。

### `apps/api/internal/globalconfig/secret.go（职责不存在时新增）`

- 集中处理加密值更新、configured 状态、fingerprint、保留旧值和安全差异。
- 日志、Audit、错误、快照和响应禁止包含明文、密文、Header、Cookie 或 Idempotency-Key。

### `对应 *_test.go 与 *_integration_test.go`

- 覆盖全部状态转换、版本竞争、stale、enable 门槛、原因排序、SSRF/DNS/redirect/TLS/timeout/body limit 和日志脱敏。
- 使用本地 httptest/DNS stub；不得依赖真实第三方服务。

## 六、实施顺序

1. 先实现状态枚举与状态转换测试。
2. 实现 Secret/Credential PATCH 和指纹语义。
3. 实现安全 HTTP Client 及完整攻击面测试。
4. 实现通用执行资格计算与稳定原因。
5. 将现有 globalconfig 保存路径接入公共基础，但不新增具体外部 Verify Adapter。

## 七、验收标准

### 7.1 统一验证状态机

- Provider、Connection、Workflow Configuration 复用同一套验证状态和合法转换，不得各自维护不一致状态机。
- 新建配置保存后为 `unverified`；开始验证转为 `verifying`；成功为 `verified`；失败为 `failed`。
- 已验证配置的关键字段变化后必须转为 `stale`，同时保留 `enabled`、引用关系和项目绑定。
- `verified` 不自动等于 `enabled`，`enabled` 不自动等于 `executable`。
- 非法状态转换必须返回稳定业务错误，不得静默覆盖。

### 7.2 版本竞争与幂等

- Verify 开始时绑定目标配置版本，完成时必须再次比较版本。
- 验证期间配置被修改时，旧验证结果不得覆盖新配置，接口返回 409 或冻结契约规定的版本冲突。
- 重复 Verify、Enable、Disable 请求遵循现有 Idempotency-Key 和 expectedVersion 规则。
- 同一幂等键重复调用不能产生多条验证事实、重复审计或错误版本增长。
- 状态更新必须检查 RowsAffected，禁止无声更新零行。

### 7.3 Secret/Credential 语义

- PATCH 未提供新 Secret/Credential 时保留旧值；空字符串不能隐式清除。
- 清除凭据必须使用冻结契约允许的显式动作或字段。
- 对外仅返回 `configured`、安全 fingerprint 或安全保存状态，不返回明文、密文、掩码原值。
- Secret/Credential 变化必须影响配置版本和验证状态。
- 日志、错误、Audit、快照、测试失败输出和 HTTP 响应均不能包含测试 Secret。

### 7.4 执行资格

- `executable` 由服务端实时派生，客户端请求不能写入，数据库不能持久化为事实状态。
- 不可执行原因必须稳定包含 `code / message / dependencyType / dependencyId / repairAction`。
- 同一输入多次计算得到相同原因顺序，避免前端状态抖动。
- 至少覆盖未验证、验证失败、stale、已停用、模型不可用、依赖无效和策略不完整。
- 依赖恢复并重新验证后，无需重新绑定即可恢复 executable。

### 7.5 安全 HTTP 基础

- 只接受明确允许的 `http` 和 `https` URL。
- IPv4/IPv6 loopback、private、link-local、metadata 和未允许地址必须被拒绝。
- DNS 首次解析、连接前解析和每次重定向都重新执行地址校验。
- 重定向次数、连接超时、总超时、响应体大小和 TLS 行为均有固定上限。
- 测试覆盖 DNS rebinding、重定向到私网、超时、过大响应、非法协议和证书错误。
- 测试使用 `httptest`、Resolver/Transport stub，不依赖真实公网。

### 7.6 复用与架构边界

- 公共能力位于现有 `globalconfig`/`platform` 基础模块，Provider、Connection 和 Workflow Configuration 后续 Adapter 必须复用。
- 不得在具体 Adapter 中复制 SSRF、凭据脱敏、验证状态或执行资格逻辑。
- 不得创建与主 OpenAPI 枚举不同的第二套字符串定义。
- 本任务不实现具体 Provider 模型发现、n8n Verify、WorkflowRun Runtime 或前端页面。

### 7.7 测试与门禁

- 状态转换、版本竞争、Secret PATCH、执行资格和安全 HTTP 单元测试全部 PASS。
- `globalconfig` PostgreSQL 集成测试 PASS，`SKIP=0`。
- 安全测试可证明敏感数据未出现在日志、错误和 DTO 快照。
- 受影响后端包、静态检查、数据库一致性门禁全部 PASS。
- 测试连续执行后数据库无状态、时间、孤儿或验证版本异常。
- `git diff --check`、精确暂存、Commit、Push 和最终工作区 clean 全部 PASS。

## 八、验证要求

- globalconfig 单元与 PostgreSQL 集成测试 PASS，SKIP=0。
- 安全 HTTP 测试覆盖 IPv4/IPv6 loopback、private、link-local、metadata、DNS 变化和重定向。
- 竞态测试证明旧 version 验证结果不能覆盖新配置。
- 日志/错误/DTO 快照搜索不存在测试 Secret。
- 受影响 API 包测试、go test、go vet/项目门禁 PASS。
- 数据库一致性门禁仍全部 PASS。

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

固定 Commit message：`feat: add integration validation foundation`

```powershell
git commit -m "feat: add integration validation foundation"
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
- 下一任务：Task 05 — 集成配置与项目绑定闭环。

BLOCKED 仅允许用于：冻结契约真正互斥、必须扩大授权、外部依赖不可恢复、需要破坏性操作或必须修改上游冻结方案。普通代码、SQL、类型、测试、格式、路径、暂存和网络问题必须自行修复到 PASS。
