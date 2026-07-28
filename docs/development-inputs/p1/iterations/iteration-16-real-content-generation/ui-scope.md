# Iteration 16 — 真实正文生成 — UI Scope

**UI 状态：`APPROVED_SOURCE_20260728`。** Stitch 定稿共 11 个 Frame，全部基于现有 ACF 桌面框架。`screen.png` 是视觉和信息层级权威输入，`code.html` 仅辅助理解结构和文案。

## 1. 正式路由

- 编辑器：`/projects/{projectId}/works/{workId}`
- `workId`：`ContentItem.id`
- Run 详情：复用 `/workflow-runs/{runId}`
- 项目工作流设置：复用 `/projects/{projectId}/settings`
- 全局连接/配置：复用现有 `/settings` 和工作流配置入口

11 个 Frame 不是 11 条路由。抽屉、状态条、失败和未配置状态都在编辑器正式路由内由真实状态触发。

## 2. Frame 清单

| 编号 | Frame | 类型 | 触发方式 | 权威 PNG | HTML |
|---|---|---|---|---|---|
| 01 | `I16_D1_EDITOR_CHAPTER_GOAL` 正文编辑器｜章节目标 | `editor_state` | 打开正文编辑器，右侧选择“章节目标” | `ui/frames/I16_D1_EDITOR_CHAPTER_GOAL/screen.png` | `ui/frames/I16_D1_EDITOR_CHAPTER_GOAL/code.html` |
| 02 | `I16_D1_EDITOR_STORY_CONTEXT` 正文编辑器｜故事情报 | `editor_state` | 在右侧上下文面板选择“故事情报” | `ui/frames/I16_D1_EDITOR_STORY_CONTEXT/screen.png` | `ui/frames/I16_D1_EDITOR_STORY_CONTEXT/code.html` |
| 03 | `I16_D1_EDITOR_MATERIALS` 正文编辑器｜素材库 | `editor_state` | 在右侧上下文面板选择“素材库” | `ui/frames/I16_D1_EDITOR_MATERIALS/screen.png` | `ui/frames/I16_D1_EDITOR_MATERIALS/code.html` |
| 04 | `I16_D2_GENERATE_CONFIRM` 生成正文抽屉｜运行前确认 | `drawer` | 点击“生成正文”，打开运行前确认抽屉 | `ui/frames/I16_D2_GENERATE_CONFIRM/screen.png` | `ui/frames/I16_D2_GENERATE_CONFIRM/code.html` |
| 05 | `I16_D2_GENERATE_REQUIREMENTS` 生成正文抽屉｜已填写要求 | `drawer` | 在生成抽屉填写“本次补充要求” | `ui/frames/I16_D2_GENERATE_REQUIREMENTS/screen.png` | `ui/frames/I16_D2_GENERATE_REQUIREMENTS/code.html` |
| 06 | `I16_D3_RUN_QUEUED` 正文生成任务｜排队中 | `async_state` | 创建正文生成 Run 后显示 queued 状态条 | `ui/frames/I16_D3_RUN_QUEUED/screen.png` | `ui/frames/I16_D3_RUN_QUEUED/code.html` |
| 07 | `I16_D3_RUN_RUNNING` 正文生成任务｜运行中 | `async_state` | WorkflowRun 进入 running 并显示安全进度信息 | `ui/frames/I16_D3_RUN_RUNNING/screen.png` | `ui/frames/I16_D3_RUN_RUNNING/code.html` |
| 08 | `I16_D3_RUN_SUCCEEDED` 正文生成任务｜已成功 | `async_state` | 结果消费成功并创建非当前候选 ContentVersion | `ui/frames/I16_D3_RUN_SUCCEEDED/screen.png` | `ui/frames/I16_D3_RUN_SUCCEEDED/code.html` |
| 09 | `I16_D4_CANDIDATE_VERSION` 正文编辑器｜候选版本 | `candidate_state` | 打开候选版本，比较并选择“设为当前版本” | `ui/frames/I16_D4_CANDIDATE_VERSION/screen.png` | `ui/frames/I16_D4_CANDIDATE_VERSION/code.html` |
| 10 | `I16_D3_RUN_FAILED` 正文生成任务｜已失败 | `error_state` | Runtime、输出校验或结果消费失败 | `ui/frames/I16_D3_RUN_FAILED/screen.png` | `ui/frames/I16_D3_RUN_FAILED/code.html` |
| 11 | `I16_D5_NOT_CONFIGURED` 未配置正文生成工作流 | `empty_state` | 项目缺少 content_generation 工作流绑定或执行连接不可用 | `ui/frames/I16_D5_NOT_CONFIGURED/screen.png` | `ui/frames/I16_D5_NOT_CONFIGURED/code.html` |

## 3. 布局冻结

- 保持 ACF 顶部应用栏、项目面包屑和现有导航；
- 工作区为左侧章节目录、中间正文编辑、右侧上下文面板；
- 异步状态条位于面包屑下、三栏工作区上方；
- 生成设置使用右侧抽屉；
- 版本切换和候选操作位于正文标题区；
- 底部状态栏显示字数、当前/候选版本、保存状态和最近保存时间。

## 4. 开发修正规则

- 用户可见业务文案统一为中文；
- 原型 HTML 中的 `arrow_drop_down`、`history`、`auto_awesome`、`check_circle` 等是图标语义，不得作为文本显示；
- 使用内联 SVG 或现有图标组件，不能依赖外部 ligature 字体才能正确显示；
- `Run ID`、版本号、模型名和 Schema 等技术标识可保留英文；
- 不要求像素级一致；必须保证主要结构、状态、内容层级和操作闭环一致；
- 禁止用原型图片作为页面背景或创建仅截图可用的路由；
- 未配置、失败和候选状态必须来自正式 API/契约夹具，不得在生产代码硬编码。

## 5. Canonical 与变体

- Canonical 编辑器：`I16_D1_EDITOR_CHAPTER_GOAL`；
- `I16_D1_EDITOR_STORY_CONTEXT`、`I16_D1_EDITOR_MATERIALS` 是右侧面板状态；
- `I16_D2_*` 是同一编辑器中的生成抽屉；
- `I16_D3_*` 是同一编辑器中的异步状态；
- `I16_D4_CANDIDATE_VERSION` 是版本选择状态；
- `I16_D5_NOT_CONFIGURED` 是绑定缺失/不可用状态，不是独立页面。
