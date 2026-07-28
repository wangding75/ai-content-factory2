# Iteration 15 真实章节规划联调与 UI 验收报告

## 1. 执行摘要

- 目标：完成真实章节规划的 Preflight、Create Run、n8n、Candidate、Adopt 闭环，并逐页验收 21 个冻结原型。
- 环境：Web `127.0.0.1:13001`、API `127.0.0.1:18080`、n8n `127.0.0.1:15678`、PostgreSQL `127.0.0.1:15433`。
- 测试项目：`13a13e7e-656e-4174-bc96-c301692ebced`。
- 最终 Commit：本报告所在提交；完整 hash 见任务最终回执。
- 真实链路结论：PASS。正式 API、PostgreSQL、Production Webhook 与 Runtime Consumer 全程使用真实数据；无 Mock 成功链路。
- 原型验收结论：PASS。21/21 清点完成，21 张原型图、21 张真实系统图、21 张并排对比图齐全。
- 中断痕迹处理：复用数据库中已存在且可审计的成功 Run、三类 diff 与采用记录；任务开始前的两个前端改动经审核确认正确并保留。原有 3 张截图均已过期或不合格，已删除并由本报告证据替换，未重复伪造同一执行链。

## 2. 真实链路证据

### 2.1 配置资源

| 资源 | 结果 |
|---|---|
| n8n Workflow | `f4e32de0-15a0-4100-8000-000000000001`，active，同名仅一份 |
| Connection | `8aa40c62-e2cc-42cf-a141-0b2012d5b0eb`，connected/enabled，v2，复用 |
| Workflow Configuration | `c3c90c91-e7ec-4e39-b948-8923bae21d17`，connected/enabled，v6，复用 |
| Binding | `fad8d433-35a6-4c11-874c-dc6142ae27d6`，v1，校验前后未变化 |

`setup-local-n8n-workflow.ps1` 与 `verify-local-n8n-integration.ps1` 均通过；验证脚本确认 Preflight passed、digest 稳定，资源未重复创建。

### 2.2 Preflight

| 场景 | 结果 |
|---|---|
| `auto_balanced` | HTTP 200、passed、token 存在、blockers 为空；digest `0eb0e8dca47ba730392ac550666519bc9b29ee242c932959cf66c3f4f4461f64` |
| `specified` | HTTP 200、passed、选定 Storyline 被正确冻结；digest `8ac460d0da8fb57a7d7855d91bee0125fe54c550f32d1e0a8c1601c9c3ec7ecf` |
| `project_binding_missing` | HTTP 200、blocked、无 token，安全原因与恢复动作齐全 |
| `generation_input_invalid` | HTTP 200、blocked、无 token，非法章节范围被拒绝 |
| `storyline_reference_invalid` | HTTP 200、blocked、无 token，跨项目或不存在的 Storyline 被拒绝 |
| `execution_integration_unavailable` | HTTP 200、blocked、无 token；通过正式 API 恢复后配置为 enabled v6 |

### 2.3 Create Run、n8n 与状态流转

- 核心 `runId`：`00948a40-f1e8-49ef-8d92-784d0ce1f038`。
- 首次 Create Run HTTP 201；相同 Idempotency-Key 与相同请求重放返回同一 `runId`，Run 总数只增加 1；相同 Key 不同请求返回 HTTP 409。
- n8n Production Webhook execution ID：`34`；固定 Workflow ID 正确，execution 为 success，Runtime Consumer 输出通过。
- Run 状态：`queued → running → succeeded`；`startedAt`、`finishedAt` 存在，`errorCode`、`errorMessage` 为空。
- 核心批次：`ef248288-882f-4ee0-b226-29dded4c0f39`，2 个候选。
- 中文输出复验：Run `0a3bec94-aa4e-4b40-8742-e47922a9d778`，Batch `f7b2889d-b5bc-400e-aeeb-ef8d75c4f9a7`，2 个候选；标题、摘要、上下文说明均为中文。

### 2.4 Summary、Batch、Candidate 与 diff

最终 Summary：`activeRun=null`，线上章节 2，待确认 2，已确认 0；批次状态为 ready 3、partiallyAdopted 1、adopted 2、abandoned 0。项目共有 6 个批次、12 个候选。

| diffType | runId | batchId | candidateId | 数据准备与结论 |
|---|---|---|---|---|
| `new` | `2f5f5c7d-a61e-48eb-bc44-98ddf120d55c` | `c5e51dc1-71bb-4c16-9c21-40b47ea5b9a8` | `aecaa9c5-18e6-403c-bf56-ce93662ce8b6` | 中断前正式运行持久化记录；无 base plan，真实计算为 new，已采用 |
| `no_change` | `e2a6592c-6ce4-4b18-b3d0-6b0e0fe6d89c` | `9975632d-1472-4c74-9d21-5da774ea42a8` | `81b9d568-24fa-4c7a-8406-3686635f4699` | 中断前正式运行生成时与 base 一致；后续修改线上版本并正式 recompare，现按预期转为 `stale_conflict` |
| `replace` | `97106b57-e06a-41cb-a94d-5d2ccd041df7` | `d33f7aef-2c41-4919-b936-d175d457324d` | `b50c25d5-9d21-4b3d-a3a2-e165d98b1b15` | 正式运行对已有章节产生替换差异，已采用 |

Recompare 后 Batch 统计由服务事务内重新计算：`pending=0`、`stale=1`，页面刷新与 API 一致。Compare API 只返回公共 DTO，差异值使用名称/数量，不泄漏 UUID 或 Go 内部字段。

### 2.5 Adopt 与隔离

- 最终正式采用 Candidate：`5521d8df-6df2-485b-80ac-dbe06d71417e`。
- 采用结果：`adopted`，Chapter Plan `6803842f-62eb-453e-b6b7-1c808e745804`，Revision `c3eeff62-238e-43e1-b7ee-6b3c443744ed`。
- 相同 SHA-256 幂等键与相同请求重放：返回同一 Plan 与 Revision；Revision 总数仍为 1。
- 跨 Batch Candidate：修复后返回 item failed，Candidate 未写入。
- 跨项目 Candidate：由相同所有权谓词拒绝，并有独立单元测试覆盖。
- 已采用 Candidate 的重放返回正式幂等结果，不产生重复 Revision。

## 3. 冻结原型清单

所有原型来源均为 `ui/frames/<frame>/screen.png`；真实截图使用 100% 缩放的 Chromium。浏览器内容视口因窗口装饰产生的 19px 高度差由证据脚本在白色画布中无裁剪归一化到 1440×1024。

| 编号 | 原型名称 / 来源 frame | 类型 | 实际路由 | 真实数据状态 | 原型图 | 实际截图 | 对比图 | 初始差异 | 修复内容 | 结论 |
|---|---|---|---|---|---|---|---|---|---|---|
| 01 | 运行中·主线扩写 / `P15_C1_RUNNING_MAINLINE_EXPANSION` | 页面 | `/projects/{projectId}/chapter-plans` | 真实已采用中文章节 | [prototype](evidence/01-running-mainline-expansion-prototype.png) | [actual](evidence/01-running-mainline-expansion-actual.png) | [comparison](evidence/01-running-mainline-expansion-comparison.png) | Mock 入口、内部状态文案 | 移除 Mock 入口，统一真实状态展示 | PASS |
| 02 | 运行中·局部范围 / `P15_C1_RUNNING_PARTIAL_RANGE` | 页面 | `/projects/{projectId}/chapter-plans` | 待确认筛选 | [prototype](evidence/02-running-partial-range-prototype.png) | [actual](evidence/02-running-partial-range-actual.png) | [comparison](evidence/02-running-partial-range-comparison.png) | 筛选与状态层级 | 按真实 Summary/Plan 映射 | PASS |
| 03 | 运行中·完整规划 / `P15_C1_RUNNING_FULL_PLAN` | 页面 | `/projects/{projectId}/chapter-plans` | 真实章节选中态 | [prototype](evidence/03-running-full-plan-prototype.png) | [actual](evidence/03-running-full-plan-actual.png) | [comparison](evidence/03-running-full-plan-comparison.png) | 选中反馈 | 对齐冻结交互反馈 | PASS |
| 04 | 运行中·局部 ETA / `P15_C1_RUNNING_PARTIAL_ETA` | 弹窗 | `/projects/{projectId}/chapter-plans` | 真实确认弹窗 | [prototype](evidence/04-running-partial-eta-prototype.png) | [actual](evidence/04-running-partial-eta-actual.png) | [comparison](evidence/04-running-partial-eta-comparison.png) | 操作顺序 | 对齐二次确认与禁用状态 | PASS |
| 05 | 运行中·校验中 / `P15_C1_RUNNING_PARTIAL_VALIDATING` | 抽屉 | `/projects/{projectId}/chapter-plans` | 真实章节编辑 | [prototype](evidence/05-running-partial-validating-prototype.png) | [actual](evidence/05-running-partial-validating-actual.png) | [comparison](evidence/05-running-partial-validating-comparison.png) | 抽屉信息层级 | 保留真实关系数据与错误恢复 | PASS |
| 06 | 运行中·数据变体 / `P15_C1_RUNNING_DATA_VARIANT` | 页面 | `/projects/{projectId}/chapter-plans` | 真实搜索结果 | [prototype](evidence/06-running-data-variant-prototype.png) | [actual](evidence/06-running-data-variant-actual.png) | [comparison](evidence/06-running-data-variant-comparison.png) | 数据变体文案 | 使用安全中文映射 | PASS |
| 07 | 失败·原子失败 / `P15_C1_FAILED_ATOMIC` | 详情 | `/workflow-runs/{runId}` | 真实失败 Run | [prototype](evidence/07-failed-atomic-prototype.png) | [actual](evidence/07-failed-atomic-actual.png) | [comparison](evidence/07-failed-atomic-comparison.png) | 原始错误暴露风险 | 使用安全失败摘要 | PASS |
| 08 | 未配置 / `P15_C1_NOT_CONFIGURED` | 页面/阻断 | `/projects/{projectId}/chapter-plans` | 未绑定项目真实阻断 | [prototype](evidence/08-not-configured-prototype.png) | [actual](evidence/08-not-configured-actual.png) | [comparison](evidence/08-not-configured-comparison.png) | 恢复动作不明确 | 映射 `configure_workflow` 等动作 | PASS |
| 09 | 生成设置 / `P15_C2_GENERATION_SETTINGS` | 抽屉 | `/projects/{projectId}/chapter-plans` | 真实生成参数 | [prototype](evidence/09-generation-settings-prototype.png) | [actual](evidence/09-generation-settings-actual.png) | [comparison](evidence/09-generation-settings-comparison.png) | 原始 ID 输入 | Storyline 改为名称选择器 | PASS |
| 10 | 预检进行中 / `P15_C2_PREFLIGHT_PROGRESS` | 弹窗 | `/projects/{projectId}/chapter-plans` | 并发捕获真实 Preflight | [prototype](evidence/10-preflight-progress-prototype.png) | [actual](evidence/10-preflight-progress-actual.png) | [comparison](evidence/10-preflight-progress-comparison.png) | 加载反馈 | 完整加载遮罩与防重复提交 | PASS |
| 11 | 预检通过 / `P15_C3_PREFLIGHT_PASS` | 弹窗 | `/projects/{projectId}/chapter-plans` | passed 报告 | [prototype](evidence/11-preflight-pass-prototype.png) | [actual](evidence/11-preflight-pass-actual.png) | [comparison](evidence/11-preflight-pass-comparison.png) | 技术字段偏多 | 使用安全业务摘要 | PASS |
| 12 | 预检阻断 / `P15_C3_PREFLIGHT_BLOCKED` | 弹窗 | `/projects/{projectId}/chapter-plans` | `project_binding_missing` | [prototype](evidence/12-preflight-blocked-prototype.png) | [actual](evidence/12-preflight-blocked-actual.png) | [comparison](evidence/12-preflight-blocked-comparison.png) | 英文 raw message | 仅展示 safeReason 与可执行恢复动作 | PASS |
| 13 | Run 已创建 / `P15_C3_RUN_CREATED` | 弹窗/横幅 | `/projects/{projectId}/chapter-plans` | UI 正式创建 queued Run | [prototype](evidence/13-run-created-prototype.png) | [actual](evidence/13-run-created-actual.png) | [comparison](evidence/13-run-created-comparison.png) | 暴露 run UUID | 改为业务状态与友好入口 | PASS |
| 14 | 候选批次列表 / `P15_C4_CANDIDATE_BATCH_LIST` | 页面 | `/projects/{projectId}/chapter-plan-candidate-batches` | 6 个真实批次 | [prototype](evidence/14-candidate-batch-list-prototype.png) | [actual](evidence/14-candidate-batch-list-actual.png) | [comparison](evidence/14-candidate-batch-list-comparison.png) | 原始 Run ID 筛选 | 改为友好来源选择器 | PASS |
| 15 | 候选批次详情 / `P15_C5_CANDIDATE_BATCH_DETAIL` | 页面 | `/projects/{projectId}/chapter-plan-candidate-batches/{batchId}` | 中文真实候选 | [prototype](evidence/15-candidate-batch-detail-prototype.png) | [actual](evidence/15-candidate-batch-detail-actual.png) | [comparison](evidence/15-candidate-batch-detail-comparison.png) | Storyline ID、版本枚举 | 名称选择器与“第 N 版” | PASS |
| 16 | 候选编辑 / `P15_C6_CANDIDATE_EDIT_DRAWER` | 抽屉 | `/projects/{projectId}/chapter-plan-candidate-batches/{batchId}` | pending Candidate | [prototype](evidence/16-candidate-edit-drawer-prototype.png) | [actual](evidence/16-candidate-edit-drawer-actual.png) | [comparison](evidence/16-candidate-edit-drawer-comparison.png) | 内部枚举 | 中文选项与安全冲突提示 | PASS |
| 17 | 候选对比 / `P15_C7_CANDIDATE_COMPARE_DIALOG` | 弹窗 | `/projects/{projectId}/chapter-plan-candidate-batches/{batchId}` | replace Candidate | [prototype](evidence/17-candidate-compare-dialog-prototype.png) | [actual](evidence/17-candidate-compare-dialog-actual.png) | [comparison](evidence/17-candidate-compare-dialog-comparison.png) | UUID、对象、内部 DTO 泄漏 | 公共 DTO、名称/数量映射、中文 purpose | PASS |
| 18 | 批量采用 / `P15_C8_BATCH_ADOPT_DIALOG` | 弹窗 | `/projects/{projectId}/chapter-plan-candidate-batches/{batchId}` | 真实选中候选 | [prototype](evidence/18-batch-adopt-dialog-prototype.png) | [actual](evidence/18-batch-adopt-dialog-actual.png) | [comparison](evidence/18-batch-adopt-dialog-comparison.png) | 显示 Batch UUID | 改为“当前批次”与逐项结果 | PASS |
| 19 | 放弃批次 / `P15_C9_BATCH_ABANDON_DIALOG` | 弹窗 | `/projects/{projectId}/chapter-plan-candidate-batches/{batchId}` | 真实批次 | [prototype](evidence/19-batch-abandon-dialog-prototype.png) | [actual](evidence/19-batch-abandon-dialog-actual.png) | [comparison](evidence/19-batch-abandon-dialog-comparison.png) | 非回滚语义不足 | 明确已采用章节不回滚 | PASS |
| 20 | 过期冲突 / `P15_C10_STALE_CONFLICT_DIALOG` | 弹窗 | `/projects/{projectId}/chapter-plan-candidate-batches/{batchId}` | 正式 recompare 后 stale | [prototype](evidence/20-stale-conflict-dialog-prototype.png) | [actual](evidence/20-stale-conflict-dialog-actual.png) | [comparison](evidence/20-stale-conflict-dialog-comparison.png) | Batch 统计漂移 | 事务内重算统计；只允许重比/刷新 | PASS |
| 21 | Storyline 关系只读 / `P15_S1_STORYLINE_RELATION_READONLY` | 只读详情 | `/projects/{projectId}/chapter-plan-candidate-batches/{batchId}` | 冻结快照关系 | [prototype](evidence/21-storyline-relation-readonly-prototype.png) | [actual](evidence/21-storyline-relation-readonly-actual.png) | [comparison](evidence/21-storyline-relation-readonly-comparison.png) | 关系显示为内部值 | 使用 Storyline 名称与关系文案 | PASS |

## 4. 测试结果

| 门禁 | 结果 |
|---|---|
| 前端 test | PASS，155/155 |
| 前端 typecheck | PASS |
| 前端 lint | PASS，0 error；仓库既有 3 个 warning |
| 前端 production build | PASS |
| `go test ./internal/chapterplan -count=1` | PASS |
| `go test ./internal/workflowrun -count=1` | PASS |
| HTTP 定向 `ChapterPlan\|Workflow\|Preflight\|Candidate\|Adopt` | PASS |
| `go test ./... -count=1` | PASS |
| `go vet ./...` | PASS |
| `go build ./...` | PASS |
| OpenAPI 与 Novel Schema | PASS；OpenAPI 未修改 |
| Docker production | PASS；postgres/api/n8n healthy，web running，migrate exited 0 |
| 浏览器 Console | PASS；最终生产镜像复验 0 error / 0 warning，无 hydration error |
| 浏览器 Network | PASS；目标页面与 API HTTP 200，近 10 分钟服务日志无意外 404/500/CORS/panic/fatal |
| n8n | PASS；核心 execution 34 success，本次资源复验无失败 execution |
| Evidence | PASS；prototype=21、actual=21、comparison=21，actual 无重复 hash |

全量 Go 测试第一次运行时发现测试隔离缺陷：预检副作用测试对整库做快照，与其他包并行测试的合法写入竞态。已将快照收窄到测试 `projectId`，定向测试与全量测试均通过。

## 5. 最终剩余差异

仅剩不同操作系统字体抗锯齿与 Chromium 窗口装饰造成的不可控像素级差异。真实截图内容未裁剪；证据脚本只在白色画布上将内容视口归一化为 1440×1024。无剩余业务、交互、路由、文案或可修复视觉差异。
