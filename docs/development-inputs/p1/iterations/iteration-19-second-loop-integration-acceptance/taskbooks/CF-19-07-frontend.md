本任务为执行任务，直接执行当前任务。

# CF-19-07 — Iteration 19 完整前端

- 任务书版本：`v2`
- 冻结来源：Task 01 Commit `39fa4f3c4838c5d67b98c7958ba2f7a42ece75b7`
- 详细验收标准：本文件“七、验收标准”

- 推荐模型：GPT-5.6 Terra
- 推理等级：medium
- 执行环境：Windows PowerShell
- 仓库：`D:\github\ai-content-factory2`
- 分支：`feature/second-user-loop`
- 前置状态：Task 06 后端配置、Binding 和 Runtime API 全部 PASS。

## 一、任务目标

严格按 15 个冻结 Frame 和现有 ACF AppShell 实现完整 Iteration 19 前端，接入真实 API，不改变路由和信息架构。

## 二、强制边界

- 禁止重新制定 Iteration 19 计划。
- 禁止重新评估已冻结业务方案、UI 和产品决策。
- 禁止执行本任务之后的任务。
- 禁止修改未授权模块或顺带重构。
- 禁止创建平行领域模型、重复接口或第二套状态机。
- 禁止 amend、rebase、squash、force push。
- 禁止使用 `git add .`、`git add -A` 或 `git commit -a`。
- 禁止修改 AppShell、导航、UI Frame 和后端配置/Runtime 契约。

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
- `docs/development-inputs/p1/iterations/iteration-19-second-loop-integration-acceptance/ui-scope.md`
- `docs/development-inputs/p1/iterations/iteration-19-second-loop-integration-acceptance/ui-review.md`
- `docs/development-inputs/p1/iterations/iteration-19-second-loop-integration-acceptance/ui-manifest.json`
- `apps/web/src/features/`

这些文件是当前任务的强制输入。发生冲突时先按主 OpenAPI/Schema 优先级定位；只有真正互斥且无法兼容时才允许 BLOCKED。

## 五、文件修改逻辑

### `apps/web/src/features/global-config/llm-provider-api.ts 与 test`

- 对齐 OpenAPI Provider、模型目录、discover、verify、enable、disable、validation/executable 和安全错误。
- 禁止定义宽泛 string/any，凭据响应只处理 configured 状态。

### `apps/web/src/features/global-config/workflow-connection-api.ts 与 test`

- 接入 Connection 验证、启停、版本、依赖数量和 executable。

### `apps/web/src/features/global-config/workflow-api.ts 与 test`

- 接入三种 LLM 策略、Provider/模型条件字段、六项验证结果、启停和配置版本。

### `apps/web/src/features/global-config/connection-settings-page.tsx、workflow-settings-page.tsx、connection-settings.css`

- 实现 Frame 01～06；复用现有设置 Tabs 和中文 AppShell。
- 02/04/06 抽屉内部滚动、底部操作固定；保存、验证、启用分离。
- 配置列表明确未验证/验证中/成功/失败/stale/启停/executable，不把已保存当可用。

### `apps/web/src/features/global-config/global-settings-tabs.tsx 与 css`

- 只保持现有 Tab/二级导航；不得新增一级导航或改变 AppShell。

### `apps/web/src/features/workflow-bindings/workflow-binding-api.ts 与 test`

- 对齐 bound/executable、依赖摘要、配置版本、不可执行原因和 selectable 候选。

### `apps/web/src/features/workflow-bindings/** 页面/组件`

- 实现 Frame 07～09：正常态、依赖失效态和共享选择抽屉。
- 不可执行候选可见但禁选；四 Stage 共用组件，不复制四套逻辑。

### `apps/web/src/features/workflow-runs/workflow-run-api.ts 与 test`

- 对齐高级筛选、displayStatus、失败阶段、快照、retry-options、cancel/retry 和领域影响。

### `apps/web/src/features/workflow-runs/** 列表与详情组件`

- 实现 Frame 10～12；详情保持 `/workflow-runs/[runId]` 独立页面，不改抽屉。
- 列表只常驻常用筛选，高级配置筛选折叠；Provider/模型/Connection 主要放详情。

### `apps/web/src/features/global-lite/workflow-page.tsx`

- 实现 Frame 13 最小引导；保留 `/workflows` 只读定位，不显示 Run ID 或运行详情。

### `chapter-plans、content-items、content-review、project-works 中共享状态入口`

- 接入 Frame 14～15 的 Preflight 阻断和 Runtime 恢复组件。
- 本任务只接入 UI 状态与 API 类型，不改变四 Stage 领域执行实现。

### `对应 app 路由包装文件`

- 保持 `/settings`、项目设置、`/workflow-runs`、`/workflow-runs/[runId]`、`/workflows` 现有路由。

## 六、实施顺序

1. 先同步 OpenAPI 前端类型和 API client。
2. 完成 01～06 全局配置页面。
3. 完成 07～09 Binding 页面。
4. 完成 14～15 共享状态组件。
5. 完成 10～12 WorkflowRun 列表、独立详情和重试弹窗。
6. 最后完成 13 旧页面引导和全局视觉/文案统一。

## 七、验收标准

### 7.1 路由与产品骨架

- 保持现有 `/settings`、项目设置、`/workflow-runs`、`/workflow-runs/[runId]` 和 `/workflows` 路由，不新增替代路由。
- WorkflowRun 详情继续使用独立页面，不改为右侧抽屉。
- AppShell、左侧导航、顶部导航、中文 locale 和现有视觉 Token 不变。
- `/workflows` 保持只读页面，仅增加真实运行入口引导，不显示 Run ID、运行详情或真实执行控件。

### 7.2 Provider 与 Connection 页面

- Provider 列表和抽屉完整表达未验证、验证中、验证成功、验证失败、stale、已启用、已停用和不可执行原因。
- Provider 支持模型发现、默认模型选择、验证、保存、启用和停用，操作顺序与后端契约一致。
- 模型不可用时不自动切换，必须提示重新选择并验证。
- Connection 列表和抽屉支持真实验证、启停、最近验证和依赖影响。
- 02、04 抽屉内容区可滚动，底部主操作区固定；小屏下不遮挡按钮。
- Secret/Credential 仅显示“已安全保存”或 configured 状态，不显示掩码原值。

### 7.3 Workflow Configuration 页面

- 列表能显示业务名称、Stage、LLM 策略、Provider/模型摘要、验证状态、启用状态、配置版本和不可执行原因。
- 抽屉支持三种 LLM 策略，并按策略动态显示或隐藏 Provider/模型字段。
- 六项验证结果逐项展示，不使用单一“验证失败”替代具体原因。
- 保存、验证和启用为独立操作；保存成功不能直接显示为可执行。
- 修改关键字段后页面立即显示 stale，并保留启用状态和依赖关系。
- 06 抽屉内部滚动、底部操作固定，错误区域不会推动主按钮离开可视区。

### 7.4 项目绑定页面

- 四个 Stage 共用同一绑定组件，不复制四套状态逻辑。
- 正常态明确显示“已绑定”和“可执行”是两个独立结论。
- 依赖失效态显示具体 Provider、Connection、Workflow 或 LLM 策略原因，并提供精准修复入口。
- 选择/更换抽屉只允许选择 Stage 匹配且 `selectable=true` 的候选。
- 不可执行候选仍可查看，但禁用选择并显示原因。
- 更换工作流时展示原绑定与新绑定摘要；项目层不提供 LLM 策略覆盖。

### 7.5 WorkflowRun 列表、详情与恢复

- 列表常驻项目、Stage、状态等常用筛选；Connection、Provider、模型、配置版本等放入高级筛选。
- 列表覆盖 queued、running、cancelling、succeeded、failed、cancelled、timed_out 及失败分类的展示。
- 可重试、不可重试和重试关系均有明确视觉表达。
- 独立详情页显示状态时间线、Connection、Workflow Configuration、LLM 策略、Provider、模型、配置版本、外部 Execution ID、安全错误和领域影响。
- 重试弹窗支持当前配置与原配置；原配置不可用时选项禁用并显示具体原因。
- 结果消费失败的操作文案固定为“重试提交结果”，不得显示为普通“重新运行”。

### 7.6 共享 Preflight 与 Runtime 状态

- 四个业务 Stage 共用 Frame 14 的不可执行状态组件，覆盖未绑定、Provider 失效、Connection 失效、Workflow 失效和策略不完整。
- Frame 15 覆盖排队、运行、取消中、超时、运行失败、输出校验失败和结果消费失败。
- 页面刷新后能够从 API 恢复 active/latest Run，不依赖本地临时状态。
- 本任务只接入状态和 API，不改变 Iteration 15～18 的领域主体交互。
- 正式页面不得再出现“模拟生成、模拟审核、模拟重写”作为真实执行说明；最终生产 wiring 清理由 Task 08 完成。

### 7.7 交互、可访问性与脱敏

- 抽屉、弹窗和详情页支持键盘关闭、焦点管理和禁用态，不产生重复提交。
- 验证、启用、停用、取消和重试操作有 loading、成功、失败和 409 刷新恢复。
- 所有错误使用安全文案，不显示 Secret、Credential、Header、Cookie、rawResponse 或内部节点数据。
- 未知枚举不得直接显示原始字符串；合法枚举必须显式映射。
- 页面在现有桌面目标分辨率下无横向溢出、遮挡或不可滚动区域。

### 7.8 自动与人工验收

- API client、状态映射和主要组件参数化测试全部 PASS。
- 覆盖五种验证状态、stale、不可执行原因、双 RetryMode、结果消费恢复和脱敏。
- `typecheck`、`lint`、全部 unit tests、正式 build 全部 PASS，`SKIP=0`。
- 逐个核对 15 个 Frame：结构、信息层级、文案、状态和操作均与冻结原型一致；开发修正规则优先于原型中的旧壳层细节。
- 浏览器验收确认 AppShell 不变、详情页独立、抽屉滚动正常、高级筛选收起/展开正常。
- 最终差异只位于授权前端文件；后端、Migration、主 OpenAPI、冻结文档和 UI Frame 均未修改。
- `git diff --check`、精确暂存、Commit、Push 和最终工作区 clean 全部 PASS。

## 八、验证要求

- 每个 API 文件参数化单元测试 PASS。
- 组件测试覆盖五种验证状态、stale、不可执行原因、双 RetryMode 禁用原因和脱敏。
- 完整前端 typecheck、lint、unit test、build PASS，SKIP=0。
- 浏览器核对 15 Frame，AppShell、中文 locale、抽屉滚动和独立详情页符合冻结要求。
- 搜索确认没有 Secret/Credential/Header/rawResponse 显示。
- 后端、数据库、OpenAPI 和文档未修改。

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

固定 Commit message：`feat: implement iteration 19 integration ui`

```powershell
git commit -m "feat: implement iteration 19 integration ui"
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
- 下一任务：Task 08 — 四 Stage 真实 Runtime 接入。

BLOCKED 仅允许用于：冻结契约真正互斥、必须扩大授权、外部依赖不可恢复、需要破坏性操作或必须修改上游冻结方案。普通代码、SQL、类型、测试、格式、路径、暂存和网络问题必须自行修复到 PASS。
