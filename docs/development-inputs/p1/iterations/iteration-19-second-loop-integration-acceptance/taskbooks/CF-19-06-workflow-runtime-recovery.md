本任务为执行任务，直接执行当前任务。

# CF-19-06 — Workflow Runtime 与失败恢复

- 任务书版本：`v2`
- 冻结来源：Task 01 Commit `39fa4f3c4838c5d67b98c7958ba2f7a42ece75b7`
- 详细验收标准：本文件“七、验收标准”

- 推荐模型：GPT-5.6 Sol
- 推理等级：high
- 执行环境：Windows PowerShell
- 仓库：`D:\github\ai-content-factory2`
- 分支：`feature/second-user-loop`
- 前置状态：Task 05 配置与 Binding 全链路 executable PASS。

## 一、任务目标

在现有 `workflowrun` 模块完成 Run 创建、安全快照、真实 n8n Runtime、状态机、取消、超时、失败分类、读取模型和版本化重试。

## 二、强制边界

- 禁止重新制定 Iteration 19 计划。
- 禁止重新评估已冻结业务方案、UI 和产品决策。
- 禁止执行本任务之后的任务。
- 禁止修改未授权模块或顺带重构。
- 禁止创建平行领域模型、重复接口或第二套状态机。
- 禁止 amend、rebase、squash、force push。
- 禁止使用 `git add .`、`git add -A` 或 `git commit -a`。
- 禁止实现四 Stage 领域结果消费。

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
- `apps/api/internal/workflowrun/`
- `apps/api/internal/workflowbinding/`

这些文件是当前任务的强制输入。发生冲突时先按主 OpenAPI/Schema 优先级定位；只有真正互斥且无法兼容时才允许 BLOCKED。

## 五、文件修改逻辑

### `apps/api/internal/workflowrun/domain.go`

- 统一 Runtime status、displayStatus、failurePhase、retryability、RetryMode、EventType 和合法转换。
- 失败 status 与 failure_phase/error/finished_at 形状必须符合数据库约束。

### `apps/api/internal/workflowrun/executor.go`

- 定义提交、查询、取消和安全输出的统一 Executor 接口；输入使用冻结快照，不读取运行中可变配置。

### `apps/api/internal/workflowrun/n8n_executor.go`

- 实现真实 n8n 提交、状态查询、Execution ID、输出读取和取消。
- 复用 Safe HTTP、认证和脱敏；禁止记录 n8n 原始敏感响应和内部节点数据。

### `apps/api/internal/workflowrun/service.go`

- 创建 Run 时事务锁定 Binding，复核 executable，冻结 binding/configuration/connection/LLM/input 快照和幂等记录。
- 实现 retry-options：当前配置实时复核；原配置按快照完整性、记录存在、fingerprint、安全 URL 和外部引用判断。
- 重试必须创建新 Run、设置 retry_of_run_id/retry_mode，原 Run 不改写。
- 结果消费失败拒绝 Runtime retry，并返回 Stage 专用“重试提交结果”动作。

### `apps/api/internal/workflowrun/worker.go`

- 驱动 queued→running→终态，持久化事件和外部 Execution ID。
- 实现 cancelling、外部取消、重复取消幂等、超时和进程恢复。
- 输出只持久化安全必要内容，后续 Stage 消费仍由 Task 08 接入。

### `apps/api/internal/workflowrun/repository.go`

- 实现快照原子写入、状态 CAS、事件追加、高级筛选、详情和 retry 链查询。
- 确保 created_at≤started_at≤finished_at，避免共享数据库测试污染。

### `apps/api/internal/workflowrun/*_test.go、*_integration_test.go`

- 覆盖成功、提交失败、轮询失败、认证失败、超时、取消竞争、重复命令、进程恢复、输出校验失败和重试资格。
- 使用本地 fake n8n/httptest，不依赖最终真实外部环境。

### `apps/api 既有 WorkflowRun Handler/路由文件`

- 实现主 OpenAPI 的列表高级筛选、独立详情、Events、retry-options、retries、cancel。
- 保留现有 Path/operationId 和 ErrorEnvelope。

## 六、实施顺序

1. 先冻结 domain 状态转换和 Repository CAS 测试。
2. 实现 n8n Executor 和安全外部交互测试。
3. 实现 Run 创建事务与不可变快照。
4. 实现 Worker 生命周期、取消和超时。
5. 实现 retry-options 和两种 RetryMode。
6. 最后扩展列表/详情/API，并运行完整 Runtime 恢复矩阵。

## 七、验收标准

### 7.1 Run 创建与不可变快照

- 创建 Run 前必须锁定当前项目 Binding，并在同一事务内复核 Stage、版本和 executable。
- Binding 不存在或不可执行时不得创建 WorkflowRun、Event 或幂等事实。
- 成功创建时一次性冻结 Workflow Configuration、Connection、LLM 策略、Provider、模型、契约版本和业务输入摘要。
- Worker 执行期间只读取冻结快照，不读取当前可变配置。
- 快照不得包含 Secret、Credential、Authorization Header、Cookie、完整第三方响应或 n8n 内部节点数据。
- 重复创建请求使用同一幂等键时只返回同一 Run，不生成重复 Run。

### 7.2 n8n Runtime 执行

- Executor 能完成提交、状态查询、输出读取和取消，保存外部 Execution ID。
- 提交失败、认证失败、轮询失败、超时和非法输出均映射到冻结的失败阶段和安全错误。
- 原始上游错误只保留安全摘要，不写入日志、事件或 Run 快照。
- 进程重启后 queued/running/cancelling Run 能根据持久化状态继续恢复，不依赖内存事实。
- Runtime 测试使用本地 fake/httptest；最终真实外部联调留给 Task 09。

### 7.3 状态机与事件时间线

- 合法状态转换至少覆盖：
  - `queued -> running -> succeeded/failed/timed_out`；
  - `queued/running -> cancelling -> cancelled`。
- 非法逆向或重复终态转换必须被 CAS 拒绝。
- 每次关键状态变化追加唯一、顺序稳定的 WorkflowRun Event。
- Run 必须满足 `created_at <= started_at <= finished_at`；取消和超时字段与终态一致。
- `displayStatus`、Runtime status 和 failure phase 不能产生互相矛盾的组合。

### 7.4 取消与超时

- queued 和 running Run 可请求取消；终态 Run 取消返回稳定冲突。
- 重复取消使用同一幂等键不产生重复外部取消和重复事件。
- 取消过程先进入 `cancelling`，外部确认后进入 `cancelled`；外部取消失败按冻结规则记录。
- 超时能终止或停止继续轮询，并进入 `timed_out`，记录安全错误和完成时间。
- 取消与成功/失败并发竞争只能产生一个最终事实。

### 7.5 失败分类与输出处理

- Runtime 能明确区分外部执行失败、输出校验失败和结果消费失败。
- 本任务只持久化经过安全限制的 Runtime 输出；不执行四 Stage 领域写入。
- 输出校验失败不能产生领域数据。
- 结果消费失败标记为专用恢复动作，不得自动重新调用 Executor。
- 详情读模型能返回 failure phase、安全错误、领域影响和允许的恢复动作。

### 7.6 Retry options 与版本化重试

- `current_configuration` 每次查询和执行时都实时复核当前 Binding 和依赖执行资格。
- `original_configuration` 只有在快照完整、引用存在、fingerprint 一致且外部引用可重放时可用。
- 原配置不可重放时返回稳定禁用原因，不得静默退回当前配置。
- Runtime 重试必须创建新 Run，设置 `retry_of_run_id` 和 `retry_mode`，原 Run 保持不变。
- 重复重试请求不得创建多条新 Run。
- 结果消费失败的 retry-options 不允许 Runtime 重试，只返回 Stage 专用“重试提交结果”动作。

### 7.7 列表、详情与查询

- 列表支持冻结的项目、Stage、状态、配置、Connection、Provider、模型、重试关系和时间范围筛选。
- 列表分页、排序和过滤结果稳定，不产生重复或漏行。
- 独立详情接口返回 Run 基本信息、时间线、安全配置摘要、失败诊断、外部 Execution ID、重试选项和领域影响。
- 高级筛选和详情不得暴露敏感字段或完整原始输出。
- Retry 链查询可从新 Run 追溯原 Run，且禁止自引用。

### 7.8 并发、持久化与范围

- 并发创建、状态 CAS、取消、超时和重试测试证明每个业务动作只有一个事实结果。
- Repository 和 Worker 测试连续执行后，数据库无孤儿 Event、时间异常、状态异常、快照缺失或 retry 自引用。
- WorkflowRun 单元、Repository、Worker、Handler 和 PostgreSQL 集成测试全部 PASS，`SKIP=0`。
- OpenAPI 验证和数据库一致性门禁全部 PASS。
- 本任务不得实现 ChapterPlan、ContentVersion、Review 或 Rewrite 的领域消费。
- `git diff --check`、精确暂存、Commit、Push 和最终工作区 clean 全部 PASS。

## 八、验证要求

- workflowrun 单元、Repository、Worker、API 和集成测试 PASS，SKIP=0。
- 并发/幂等测试证明重复创建、取消、重试不会生成多条事实。
- 原配置 fingerprint 变化时 retry-options 必须禁用。
- Result consumption failure 不调用 Executor、不创建新 Run。
- OpenAPI、受影响后端包和数据库一致性门禁全部 PASS。
- 测试结束数据库无 Run 时间、状态、孤儿或 retry 自引用异常。

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

固定 Commit message：`feat: complete workflow runtime recovery`

```powershell
git commit -m "feat: complete workflow runtime recovery"
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
- 下一任务：Task 07 — Iteration 19 完整前端。

BLOCKED 仅允许用于：冻结契约真正互斥、必须扩大授权、外部依赖不可恢复、需要破坏性操作或必须修改上游冻结方案。普通代码、SQL、类型、测试、格式、路径、暂存和网络问题必须自行修复到 PASS。
