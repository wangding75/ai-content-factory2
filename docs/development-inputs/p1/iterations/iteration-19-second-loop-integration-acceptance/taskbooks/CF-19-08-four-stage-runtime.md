本任务为执行任务，直接执行当前任务。

# CF-19-08 — 四 Stage 真实 Runtime 接入

- 任务书版本：`v2`
- 冻结来源：Task 01 Commit `39fa4f3c4838c5d67b98c7958ba2f7a42ece75b7`
- 详细验收标准：本文件“七、验收标准”

- 推荐模型：GPT-5.6 Sol
- 推理等级：high
- 执行环境：Windows PowerShell
- 仓库：`D:\github\ai-content-factory2`
- 分支：`feature/second-user-loop`
- 前置状态：Task 07 完整前端 PASS，统一 Runtime 和 UI 已可用。

## 一、任务目标

在现有 Iteration 15～18 领域模块中，按章节规划→正文生成→内容审核→正文重写顺序接入统一真实 Runtime，保留领域规则、原子消费和专用消费重试。

## 二、强制边界

- 禁止重新制定 Iteration 19 计划。
- 禁止重新评估已冻结业务方案、UI 和产品决策。
- 禁止执行本任务之后的任务。
- 禁止修改未授权模块或顺带重构。
- 禁止创建平行领域模型、重复接口或第二套状态机。
- 禁止 amend、rebase、squash、force push。
- 禁止使用 `git add .`、`git add -A` 或 `git commit -a`。
- 禁止改变已冻结领域交互或新增自动循环/自动 Set Current。

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
- `apps/api/internal/chapterplan/`
- `apps/api/internal/contentitem/`
- `apps/web/src/features/chapter-plans/`
- `apps/web/src/features/content-items/`
- `apps/web/src/features/content-review/`
- `apps/web/src/features/project-works/`

这些文件是当前任务的强制输入。发生冲突时先按主 OpenAPI/Schema 优先级定位；只有真正互斥且无法兼容时才允许 BLOCKED。

## 五、文件修改逻辑

### `apps/api/internal/chapterplan/preflight.go、application.go`

- Preflight 使用当前 Binding executable；失败不创建 Run。
- 正式创建路径调用 workflowrun Service，冻结章节规划输入和契约，不再调用 Mock 生成器。

### `apps/api/internal/chapterplan/ingestion.go、runtime_consumer.go、consumption.go`

- 校验 n8n 输出 Schema/输入 digest 后才进入领域事务。
- 原子创建 Candidate Batch/Candidates；消费失败零部分数据。
- 保留已有专用结果消费重试，不重新调用 Runtime。

### `apps/api/internal/contentitem/generation_service.go`

- 正文生成 Preflight 和 CreateRun 接入统一 Runtime；固定 chapter/content version 输入。
- 消费成功只创建一个新 ContentVersion，保持来源链和幂等。

### `apps/api/internal/contentitem/real_review_service.go`

- 对固定 ContentVersion 发起真实审核 Run。
- 输出校验后原子写入 ReviewReport/Findings/Recommendations；保持 issue_key 和 disposition 规则。

### `apps/api/internal/contentitem/real_rewrite_service.go`

- 根据冻结审核问题和源版本发起真实重写。
- 输出校验后创建新 ContentVersion/候选，不自动 Set Current。

### `apps/api/internal/contentitem/query_service.go、application.go`

- 统一读取 active/latest Run、displayStatus、失败恢复和结果消费动作。

### `apps/api/internal/contentitem/mock_rewrite_provider.go 及其他 Mock Adapter`

- 仅从正式生产 wiring 移除；允许保留明确的单元测试 Fake 或旧 `/workflows` 只读演示数据。
- 不得删除 Iteration 15～18 回归测试依赖的测试替身。

### `apps/web 四 Stage Feature`

- 删除“模拟生成/模拟审核/模拟重写”正式文案，使用 Task 07 共享 Preflight 和 Runtime 状态。
- 保持主体布局、候选采用、编辑器、问题处理、结果比较和 Set Current 交互不变。

### `各 Stage *_integration_test.go`

- 每个 Stage 覆盖 Preflight 阻断、真实 Run、快照、输出非法、消费失败、专用消费重试、幂等和刷新恢复。
- 共享数据库测试必须 t.Cleanup 定向清理或完整合法 Fixture，不污染一致性门禁。

## 六、实施顺序

1. 先完成章节规划，执行其全部目标与包测试和数据库门禁。
2. 章节规划 PASS 后完成正文生成并重复同级门禁。
3. 正文 PASS 后完成内容审核。
4. 审核 PASS 后完成正文重写。
5. 最后统一移除正式 Mock wiring，执行四 Stage 联合闭环测试。

## 七、验收标准

### 7.1 通用 Stage 接入规则

- 四个 Stage 必须统一使用 Task 06 WorkflowRun Runtime，不创建各自的第二套外部执行状态机。
- 每个 Stage 在创建 Run 前执行 Binding executable Preflight；阻断时不得创建 Run 或领域数据。
- 每个 Run 使用冻结业务输入和契约快照，不能在执行中读取可变页面状态或当前配置。
- Runtime 成功输出必须先通过 Stage Schema 和输入 digest 校验，再进入领域事务。
- 输出校验失败不写入领域数据；结果消费失败必须零部分写入。
- 专用结果消费重试只复用持久化输出，不调用 n8n/LLM，不创建新 Runtime Run。

### 7.2 章节规划

- 成功链路创建一个 Candidate Batch 和完整 Candidates，来源 Run、输入 digest 和项目范围可追踪。
- 同一 Run 重复消费不会生成重复 Batch 或 Candidate。
- 输出非法、项目不匹配、章节编号冲突或引用非法时全部拒绝并零写入。
- 结果消费失败后通过章节规划专用消费重试恢复，Runtime 执行次数不增加。
- 原有候选比较、采用、Revision 和 current_revision 规则不回归。

### 7.3 正文生成

- 成功链路只创建一个新的 ContentVersion，并保持 ContentItem、ChapterPlan、源版本和 WorkflowRun 关系正确。
- 同一 Run 重复消费不会创建重复版本或跳号。
- 输出校验失败和消费失败均不能改变 current_version。
- 专用消费重试成功后只补齐缺失领域结果，不重新生成正文。
- 编辑器、候选版本和手动设置当前版本规则保持不变。

### 7.4 内容审核

- 审核 Run 固定引用提交时的 ContentVersion，后续 current_version 变化不影响该 Run。
- 成功消费原子创建 ReviewReport、Findings 和 Recommendations，数量和外键一致。
- `issue_key`、severity、disposition、证据和建议符合冻结契约。
- 重复消费不创建重复 Review 或 Findings。
- 消费失败后专用重试不重新执行审核模型，忽略/恢复问题等既有业务操作不回归。

### 7.5 正文重写

- 重写 Run 固定源 ContentVersion、选定审核问题和输入摘要。
- 成功消费创建新的重写 ContentVersion 或既有领域定义的候选结果，不自动 Set Current。
- 来源链、source version、source workflow run 和版本号一致。
- 重复消费不创建重复版本。
- 消费失败专用重试不重新调用 n8n/LLM；结果比较和手动设为当前版本保持不变。

### 7.6 Mock 路径收口

- 正式生产 wiring 中不再调用 `mockGenerate*`、`mockReview*`、`mockRewrite*` 或等价 Mock Adapter。
- 单元测试 Fake、确定性测试替身和 `/workflows` 只读演示数据可保留，但必须与生产 wiring 隔离。
- 四阶段正式页面不再显示模拟执行文案。
- 搜索结果中任何保留 Mock 引用都必须能证明仅用于测试或遗留只读展示。

### 7.7 刷新恢复与失败恢复

- 四个 Stage 页面刷新后均能恢复 active/latest Run 和正确 displayStatus。
- queued、running、cancelling、timed_out、运行失败、输出校验失败和结果消费失败状态与流程中心一致。
- Runtime 可重试时跳转或调用统一重试；结果消费失败只显示“重试提交结果”。
- 重试链和领域结果能从 WorkflowRun 详情追踪。
- 取消或超时后不产生领域结果。

### 7.8 分阶段门禁

- 必须按章节规划→正文生成→审核→重写顺序实施。
- 每完成一个 Stage，立即执行该 Stage 目标测试、包测试、PostgreSQL 集成测试和数据库一致性门禁；未 PASS 不得进入下一 Stage。
- 每个 Stage 的 PostgreSQL 测试 `SKIP=0`，测试结束数据库异常数为 0。
- 四 Stage 联合测试能完整跑通章节规划→正文→审核→重写的领域关系。
- 前端 typecheck/test/build、后端受影响包和最终数据库总门禁全部 PASS。
- `git diff --check`、精确暂存、Commit、Push 和最终工作区 clean 全部 PASS。

## 八、验证要求

- 四个 Stage 的目标、包级和 PostgreSQL 集成测试全部 PASS，SKIP=0。
- 每个 Stage 输出校验失败时无领域写入；消费失败时零部分数据。
- 专用消费重试复用持久化输出，不调用 n8n/LLM、不创建 Runtime Run。
- 正式代码路径搜索不再引用 mockGenerate/mockReview/mockRewrite。
- 前端 typecheck/test/build、后端全包测试和数据库一致性门禁 PASS。
- 刷新恢复、active/latest Run 和重试链行为不回归。

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

固定 Commit message：`feat: connect four stages to real workflow runtime`

```powershell
git commit -m "feat: connect four stages to real workflow runtime"
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
- 下一任务：Task 09 — 真实联调与最终验收。

BLOCKED 仅允许用于：冻结契约真正互斥、必须扩大授权、外部依赖不可恢复、需要破坏性操作或必须修改上游冻结方案。普通代码、SQL、类型、测试、格式、路径、暂存和网络问题必须自行修复到 PASS。
