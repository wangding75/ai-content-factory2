本任务为执行任务，直接执行当前任务。

# CF-19-03 — Migration 19 与持久化模型

- 任务书版本：`v2`
- 冻结来源：Task 01 Commit `39fa4f3c4838c5d67b98c7958ba2f7a42ece75b7`
- 详细验收标准：本文件“七、验收标准”

- 推荐模型：GPT-5.6 Sol
- 推理等级：high
- 执行环境：Windows PowerShell
- 仓库：`D:\github\ai-content-factory2`
- 分支：`feature/second-user-loop`
- 前置状态：Task 02 OpenAPI 契约 Commit PASS，数据库为 Migration 18 当前状态。

## 一、任务目标

实现唯一 Migration 19、Go 持久化模型、Repository 映射、Fixture 和数据库一致性门禁，使数据库严格承载冻结契约。

## 二、强制边界

- 禁止重新制定 Iteration 19 计划。
- 禁止重新评估已冻结业务方案、UI 和产品决策。
- 禁止执行本任务之后的任务。
- 禁止修改未授权模块或顺带重构。
- 禁止创建平行领域模型、重复接口或第二套状态机。
- 禁止 amend、rebase、squash、force push。
- 禁止使用 `git add .`、`git add -A` 或 `git commit -a`。
- 禁止实现真实 LLM/n8n 外部调用和前端。

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
- `apps/api/migrations/`
- `database/testdata/complete-test-data.sql`
- `database/tools/Validate-Database-Consistency.ps1`

这些文件是当前任务的强制输入。发生冲突时先按主 OpenAPI/Schema 优先级定位；只有真正互斥且无法兼容时才允许 BLOCKED。

## 五、文件修改逻辑

### `apps/api/migrations/000019_iteration_19_real_integration.up.sql`

- 先审计 Migration 18 后实际 Schema；仅增加缺失列、表、约束和索引。
- 统一三类配置 validation 状态、enabled、last_verified_version/at、安全错误、validation_details 和 version；历史 not_connected 迁为 unverified。
- 新增 `llm_provider_models`，包含 provider FK、model_key、source、availability、last_seen 和唯一约束。
- 为 Workflow Configuration 增加 llm_strategy/provider/model 条件约束；不得新增历史表。
- 扩展 workflow_run_records 的 cancelling/timed_out、failure_phase/code/message、retryability、retry_of_run_id、retry_mode、external_execution_id、取消/超时时间和安全快照字段。
- 扩展 workflow_run_events 合法 eventType；仅添加实际查询需要且不重复的索引。
- 所有数据兼容迁移可重复验证、不可丢数据；约束在回填完成后启用。

### `apps/api/migrations/000019_iteration_19_real_integration.down.sql`

- 只回退 Migration 19 新增对象，不修改 1～18。
- 按依赖逆序删除索引、约束、列和 llm_provider_models；不得清理业务数据之外的历史对象。

### `apps/api/migrations/migration-checksums.sha256`

- 使用仓库既有工具追加 000019 up/down 校验和。
- 确认历史 1～18 校验和完全不变。

### `apps/api/internal/globalconfig/service.go`

- 对齐数据库枚举、版本和验证字段的扫描/写入模型；本任务不实现真实 Verify 外呼。
- 为 Provider/Connection/Workflow Configuration Repository 路径补齐新列和安全 DTO 所需字段。

### `apps/api/internal/workflowbinding/domain.go、dto.go、repository.go、readmodel.go`

- 增加读取配置版本、LLM 策略和依赖摘要所需持久化映射。
- bound/executable 仍为读取派生，禁止在 binding 表新增持久化状态。

### `apps/api/internal/workflowrun/domain.go、repository.go`

- 映射新 Runtime 状态、失败阶段、重试关系、外部执行 ID、时间和不可变快照。
- Repository 写入必须保持现有乐观锁、幂等和状态约束。

### `database/testdata/complete-test-data.sql`

- 为新非空字段和约束补齐确定性 Fixture；三种 LLM 策略和至少一条可执行/失效样例均可查询。
- Fixture 保持单事务、稳定 UUID 和完整外键顺序。

### `apps/api/cmd/dbcheck/** 与 database/tools/**`

- 只增加 Iteration 19 新一致性检查：验证版本、策略形状、Run 快照、重试自引用、失败时间和模型 FK。
- 不得改变已有 53/45 检查语义；新增检查使用新编号。

## 六、实施顺序

1. 对实际 Schema 做只读审计，形成字段/约束/索引差异。
2. 编写 up/down Migration 并更新校验和。
3. 同步 Repository/domain/DTO 持久化映射和测试 Fixture。
4. 扩展数据库一致性检查。
5. 在唯一开发数据库执行 Migration 19、初始化 Fixture、Repository 测试和数据库门禁。
6. 验证 down/up 仅在临时测试数据库或仓库既有安全机制中执行；不得破坏唯一开发库最终状态。

## 七、验收标准

### 7.1 Migration 边界

- 仓库只新增一组 `000019` up/down Migration，不修改 `000001`～`000018` 的任何内容。
- Migration 校验清单只追加 19 的校验和，历史 1～18 校验和逐项保持不变。
- 唯一开发数据库最终状态为 version 19、`dirty=false`。
- 不得创建 `workflow_configuration_versions`、`secret_history`、凭据历史或其他未冻结历史表。

### 7.2 向上迁移与数据兼容

- 从完整 Migration 18 数据库执行 up Migration 成功，已有 Provider、Connection、Workflow Configuration、Binding、WorkflowRun 和领域数据不丢失。
- 历史 `not_connected` 等旧状态按冻结规则迁移为明确的新验证状态，迁移后不存在非法枚举值。
- 新增非空列在启用约束前完成确定性回填，不允许依赖数据库随机值或当前时间造成不可复现结果。
- Workflow Configuration 仍使用同一记录的整数 `version`，不产生额外历史行。
- 新增索引与现有 1～18 索引不重复，命名符合仓库规范。

### 7.3 Schema 约束

- Provider、Connection、Workflow Configuration 均具备验证状态、启用状态、验证版本/时间、安全错误和配置版本字段。
- `llm_provider_models` 具有 Provider 外键、模型唯一约束、来源、可用性和最后发现时间，删除或停用 Provider 时行为符合冻结文档。
- Workflow Configuration 的 LLM 策略字段组合由数据库约束保护，三种策略均可写入合法样例，非法组合必须被拒绝。
- WorkflowRun 支持冻结的 Runtime 状态、失败阶段、重试关系、RetryMode、外部执行 ID、取消/超时时间和安全快照。
- `retry_of_run_id` 禁止自引用和跨不允许范围的非法关系。
- 时间约束必须保证 `created_at <= started_at <= finished_at`，并覆盖取消和超时状态的合法形状。

### 7.4 Down/Up 可逆性

- Down Migration 只删除 Migration 19 新增对象，不能修改或删除 1～18 的表、列、索引和数据。
- Down/Up 验证必须在临时测试数据库或仓库既有安全机制中完成，不破坏唯一开发数据库最终状态。
- 完整 down→up 后 Schema 与直接 up 的最终状态一致。
- Down 过程中不存在未处理的外键、索引或依赖对象错误。

### 7.5 持久化映射

- `globalconfig`、`workflowbinding`、`workflowrun` Repository 能完整读写所有新增字段，数据库 NULL 与 Go 可选类型映射明确。
- 新枚举在数据库值、Go 领域常量和 OpenAPI 生成类型之间一一对应，不使用任意字符串绕过。
- WorkflowRun 安全快照可完整 round-trip，且不包含 Secret、Credential、Authorization Header、Cookie 或原始上游响应。
- Binding 的 `bound/executable` 继续由读取模型计算，Binding 表不得新增持久化 executable 字段。
- Repository 保留既有乐观锁、幂等和 RowsAffected 检查。

### 7.6 Fixture 与一致性门禁

- `complete-test-data.sql` 可在空业务数据库中一次性导入成功，并包含：
  - 三种 LLM 策略样例；
  - 已验证可执行配置；
  - 配置失效但绑定保留样例；
  - 当前配置和原配置重试关系样例；
  - 合法模型目录样例。
- Fixture 使用稳定 UUID、正确外键顺序和完整合法 Revision/Version，不产生共享数据库污染。
- 新增 dbcheck 能检测策略形状、验证版本、模型外键、Run 快照、重试自引用、失败状态和时间异常。
- 原有 Schema/Data consistency 检查语义和编号不变；新增检查使用新的稳定编号。
- 初始化后所有一致性检查异常数为 0。

### 7.7 测试与范围

- Migration、Repository 和 PostgreSQL 集成测试全部 PASS，`SKIP=0`。
- `Initialize-Database.ps1`、Migration History、Schema consistency 和 Data consistency 全部 PASS。
- 测试连续执行两轮后数据库仍无孤儿、状态、时间、快照、模型或重试链异常。
- 本任务不得实现真实 LLM/n8n 外呼、Runtime Worker 或前端。
- `git diff --check`、暂存检查、Commit 和 Push 均 PASS，最终工作区 clean。

## 八、验证要求

- Migration History PASS，数据库版本 19、dirty=false。
- 历史 Migration 校验和无变化。
- globalconfig、workflowbinding、workflowrun Repository/集成测试 PASS，PostgreSQL SKIP=0。
- `Initialize-Database.ps1` 与完整 Fixture PASS。
- `Validate-Database-Consistency.ps1` 全部 PASS，所有新异常数为 0。
- 确认不存在 workflow_configuration_versions 或 secret_history 等未授权表。

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

固定 Commit message：`feat: add iteration 19 persistence model`

```powershell
git commit -m "feat: add iteration 19 persistence model"
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
- 下一任务：Task 04 — 公共集成基础。

BLOCKED 仅允许用于：冻结契约真正互斥、必须扩大授权、外部依赖不可恢复、需要破坏性操作或必须修改上游冻结方案。普通代码、SQL、类型、测试、格式、路径、暂存和网络问题必须自行修复到 PASS。
