# Iteration 16 — Stitch 原型来源映射

## 1. 来源

- 上传包：`stitch_acf_iteration_16_real_content_generation_ui_optimization.zip`
- 定稿日期：2026-07-28
- 原始画布：11 张，均为 1600 × 1280
- 设计系统：`ui/DESIGN.md`

脚本复制后，仓库内具体 `screen.png` 是唯一视觉输入；不得再引用 ZIP、GitHub 目录或只到 Frame 目录的地址。

## 2. 一一映射

| 原始编号 | Frame ID | 页面/状态 | 仓库 PNG | 正式路由 |
|---|---|---|---|---|
| `01` | `I16_D1_EDITOR_CHAPTER_GOAL` | 正文编辑器｜章节目标 | `ui/frames/I16_D1_EDITOR_CHAPTER_GOAL/screen.png` | `/projects/{projectId}/works/{workId}` |
| `02` | `I16_D1_EDITOR_STORY_CONTEXT` | 正文编辑器｜故事情报 | `ui/frames/I16_D1_EDITOR_STORY_CONTEXT/screen.png` | `/projects/{projectId}/works/{workId}` |
| `03` | `I16_D1_EDITOR_MATERIALS` | 正文编辑器｜素材库 | `ui/frames/I16_D1_EDITOR_MATERIALS/screen.png` | `/projects/{projectId}/works/{workId}` |
| `04` | `I16_D2_GENERATE_CONFIRM` | 生成正文抽屉｜运行前确认 | `ui/frames/I16_D2_GENERATE_CONFIRM/screen.png` | `/projects/{projectId}/works/{workId}` |
| `05` | `I16_D2_GENERATE_REQUIREMENTS` | 生成正文抽屉｜已填写要求 | `ui/frames/I16_D2_GENERATE_REQUIREMENTS/screen.png` | `/projects/{projectId}/works/{workId}` |
| `06` | `I16_D3_RUN_QUEUED` | 正文生成任务｜排队中 | `ui/frames/I16_D3_RUN_QUEUED/screen.png` | `/projects/{projectId}/works/{workId}` |
| `07` | `I16_D3_RUN_RUNNING` | 正文生成任务｜运行中 | `ui/frames/I16_D3_RUN_RUNNING/screen.png` | `/projects/{projectId}/works/{workId}` |
| `08` | `I16_D3_RUN_SUCCEEDED` | 正文生成任务｜已成功 | `ui/frames/I16_D3_RUN_SUCCEEDED/screen.png` | `/projects/{projectId}/works/{workId}` |
| `09` | `I16_D4_CANDIDATE_VERSION` | 正文编辑器｜候选版本 | `ui/frames/I16_D4_CANDIDATE_VERSION/screen.png` | `/projects/{projectId}/works/{workId}` |
| `10` | `I16_D3_RUN_FAILED` | 正文生成任务｜已失败 | `ui/frames/I16_D3_RUN_FAILED/screen.png` | `/projects/{projectId}/works/{workId}` |
| `11` | `I16_D5_NOT_CONFIGURED` | 未配置正文生成工作流 | `ui/frames/I16_D5_NOT_CONFIGURED/screen.png` | `/projects/{projectId}/works/{workId}` |

## 3. 使用规则

1. `screen.png` 冻结布局、信息层级、状态语义和主要操作；
2. `code.html` 只辅助读取文字和组件结构；
3. HTML 中 Material Symbols 名称不是用户文案；
4. 业务字段与主 OpenAPI 冲突时，以冻结业务和 OpenAPI 为准，并在验收报告记录非阻断视觉差异；
5. 所有 Frame 在同一编辑器路由复现，禁止为状态创建独立页面；
6. 原型中示例数据不得写死进生产代码；测试可用契约兼容 fixture；
7. 不执行像素差阈值验收，主要结构、状态和操作错误仍必须修复。
