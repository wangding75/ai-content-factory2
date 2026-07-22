# Iteration 14 — WorkflowRun 运行时 — UI Scope

## 1. UI 冻结结论

人工 UI 验收：**PASS**。

冻结来源：

- Stitch 项目：`AI Content Factory 2.0 - Iteration 14 Runtime UI R2`
- Project ID：`16382408539250329714`
- 设计系统：`AI Content Factory 2.0 Commercial SaaS R2`
- 设计规范：`ui/design-system/DESIGN.md`

旧 Iteration 14 原型中的“工作流配置 Tab”“项目级运行记录页面”和独立未配置页面全部废弃，不得作为开发依据。

## 2. 信息架构

### 流程中心

- 唯一路由：`/workflow-runs`
- 直接展示运行记录；
- 不设置任何 Tab；
- 不包含工作流配置；
- 左侧全局导航高亮“流程中心”。

### 运行详情

- 路由：`/workflow-runs/{runId}`
- 面包屑：`流程中心 / 运行记录 / {runNumber}`
- 所属项目显示为可点击字段；
- 不使用“项目 / 项目名 / 工作流运行”作为父级路由；
- 返回列表时保留来源筛选。

### 项目侧

- 项目概览只显示运行摘要和最近三条运行；
- “查看全部运行记录”跳转 `/workflow-runs?projectId={projectId}`；
- “查看该环节记录”增加 `stage`；
- 项目侧不复制列表、详情或筛选器。

## 3. Frame 映射

| Frame | 页面/状态 | 路由或触发 | 截图 | HTML |
|---|---|---|---|---|
| `P14_01_WORKFLOW_RUN_LIST` | 全局运行记录列表 | `/workflow-runs` | `ui/frames/P14_01_WORKFLOW_RUN_LIST/screen.png` | `ui/frames/P14_01_WORKFLOW_RUN_LIST/code.html` |
| `P14_02_WORKFLOW_RUN_LIST_PROJECT_FILTERED` | 项目筛选后的列表 | `/workflow-runs?projectId={projectId}` | `ui/frames/P14_02_WORKFLOW_RUN_LIST_PROJECT_FILTERED/screen.png` | `ui/frames/P14_02_WORKFLOW_RUN_LIST_PROJECT_FILTERED/code.html` |
| `P14_03_WORKFLOW_RUN_LIST_EMPTY` | 全局空状态 | `/workflow-runs` | `ui/frames/P14_03_WORKFLOW_RUN_LIST_EMPTY/screen.png` | `ui/frames/P14_03_WORKFLOW_RUN_LIST_EMPTY/code.html` |
| `P14_04_WORKFLOW_RUN_LIST_ERROR` | 列表加载失败 | `/workflow-runs` | `ui/frames/P14_04_WORKFLOW_RUN_LIST_ERROR/screen.png` | `ui/frames/P14_04_WORKFLOW_RUN_LIST_ERROR/code.html` |
| `P14_05_WORKFLOW_RUN_DETAIL_SUCCEEDED` | 成功详情 | `/workflow-runs/{runId}` | `ui/frames/P14_05_WORKFLOW_RUN_DETAIL_SUCCEEDED/screen.png` | `ui/frames/P14_05_WORKFLOW_RUN_DETAIL_SUCCEEDED/code.html` |
| `P14_06_WORKFLOW_RUN_DETAIL_RUNNING` | 运行中详情 | `/workflow-runs/{runId}` | `ui/frames/P14_06_WORKFLOW_RUN_DETAIL_RUNNING/screen.png` | `ui/frames/P14_06_WORKFLOW_RUN_DETAIL_RUNNING/code.html` |
| `P14_07_WORKFLOW_RUN_DETAIL_FAILED` | 失败详情 | `/workflow-runs/{runId}` | `ui/frames/P14_07_WORKFLOW_RUN_DETAIL_FAILED/screen.png` | `ui/frames/P14_07_WORKFLOW_RUN_DETAIL_FAILED/code.html` |
| `P14_08_RUN_CONFIRM_DIALOG` | 运行确认弹窗 | 已绑定且可运行时点击“运行工作流” | `ui/frames/P14_08_RUN_CONFIRM_DIALOG/screen.png` | `ui/frames/P14_08_RUN_CONFIRM_DIALOG/code.html` |
| `P14_09_WORKFLOW_NOT_BOUND_DIALOG` | 未绑定提示弹窗 | 未绑定时点击“运行工作流” | `ui/frames/P14_09_WORKFLOW_NOT_BOUND_DIALOG/screen.png` | `ui/frames/P14_09_WORKFLOW_NOT_BOUND_DIALOG/code.html` |
| `P14_10_PROJECT_WORKFLOW_RUN_SUMMARY` | 项目概览摘要与入口 | `/projects/{projectId}` | `ui/frames/P14_10_PROJECT_WORKFLOW_RUN_SUMMARY/screen.png` | `ui/frames/P14_10_PROJECT_WORKFLOW_RUN_SUMMARY/code.html` |

## 4. 流程中心列表

必须包含：

- 全部运行、运行中、成功、失败四项统计；
- 项目、业务环节、工作流、状态、时间范围和运行编号筛选；
- 运行编号、项目、业务环节、工作流名称、状态、触发来源、开始时间、耗时和操作；
- 分页；
- 筛选口径与统计口径一致。

项目侧跳转后：

- 项目筛选预选；
- 显示可清除的项目筛选标签；
- 统计和表格只展示该项目；
- 清除后返回全局列表。

Loading：

- 使用统计卡片和表格 Skeleton；
- 不使用全页遮罩；
- 保持筛选器和表头稳定。

Error：

- 错误只出现在列表内容区域；
- 统计值显示占位；
- 筛选暂不可用；
- 提供重新加载；
- 不把失败显示成空状态。

## 5. 运行详情

三个状态复用同一页面结构：

- Header；
- 基础信息；
- 执行时间线；
- 运行摘要；
- 输入参数；
- 输出或错误结果。

成功：

- 展示业务结果摘要；
- 提供结果页面入口；
- 允许重新运行。

运行中：

- 当前步骤使用蓝色活跃状态；
- 不显示虚假百分比；
- 提供刷新状态；
- 允许取消时显示“取消运行”。

失败：

- 只在状态、图标和局部边框使用低饱和红色；
- 提供错误阶段、错误编号、处理建议；
- 技术详情默认折叠；
- 提供重新运行和查看工作流配置。

## 6. 运行确认弹窗

P14_08：

- 动态展示当前项目、环节、工作流和绑定版本；
- 展示业务输入摘要，不展示原始 JSON；
- 明确说明会创建新的运行记录；
- “开始运行”提交期间防重复；
- 成功后直接进入运行详情；
- 关闭或取消不创建运行。

## 7. 未绑定提示弹窗

P14_09：

- 由用户点击“运行工作流”触发；
- 不是独立页面；
- 背景保留原业务页面；
- 动态显示项目和环节；
- 主操作进入项目设置的工作流绑定页；
- 取消后保留原页面状态；
- 不使用黄色警告大底或红色错误视觉。

## 8. 项目概览变化

P14_10 是 Iteration 14 对项目侧的必要改造：

- 增加“工作流运行”摘要卡；
- 展示总运行、运行中、最近失败和最近运行；
- 展示最近三条运行；
- 提供查看全部、查看详情、查看环节记录；
- 页面只做摘要，不承担运行记录管理；
- 数据来自项目运行摘要 API；
- 无运行记录时显示轻量空态，不隐藏整个模块。

## 9. 开发修正规则

- 复用现有全局 Shell 和 ProjectWorkspace，不直接复制原型外壳；
- 左侧导航术语沿用当前产品正式术语；
- 用户可见文案全部使用中文 locale/i18n；
- 技术标识如 Run Number、Workflow ID、错误码可以保留；
- 不出现“工作流配置”Tab；
- 不出现 `/projects/{projectId}/workflow-runs` 第二套列表；
- 不把未绑定状态实现为完整章节规划替代页；
- 列表和详情适配现有内容宽度，不硬编码 2560px 原型画布；
- 长名称省略并提供 Tooltip；
- 状态同时使用文案与视觉，不只依赖颜色；
- 页面、弹窗和滚动区域必须完成键盘、焦点、Escape 和可访问性处理。

## 10. 人工验收门禁

开发后的人工 UI 验收至少覆盖：

1. 全局列表；
2. 项目筛选列表；
3. 空状态；
4. 加载失败；
5. 成功详情；
6. 运行中详情；
7. 失败详情；
8. 运行确认；
9. 未绑定提示；
10. 项目概览摘要；
11. Loading；
12. 取消与重试交互；
13. 项目筛选清除和返回保留；
14. 所有滚动区域滚到底。
