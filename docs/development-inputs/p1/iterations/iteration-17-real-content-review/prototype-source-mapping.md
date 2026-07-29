# Iteration 17 — Stitch 原型来源映射

**状态：`frozen_cf_17_01`。**

## 1. 来源

- 上传包：`stitch_acf_iteration_17_ui_final_rebuild_frozen_existing_shell.zip`
- 原始画布：8 张，1600 × 1280
- AppShell 规则：上传包 `acf_existing_desktop_shell_immutable/DESIGN.md`
- 仓库冻结设计：`ui/DESIGN.md`

复制后，仓库内 `screen.png` 与 `code.html` 是唯一开发原型输入，不再依赖 ZIP 路径。

## 2. 一一映射

| 原始目录 | Frame ID | 页面/状态 | 必要修正 |
|---|---|---|---|
| `01_final_qa` | `I17_D1_EDITOR_REVIEW_ENTRY` | 编辑器审核入口 | 增加 Frame 元数据 |
| `02_final_qa` | `D2_SUBMIT_REVIEW_DRAWER` | 发起审核抽屉 | 增加 Frame 元数据 |
| `03_final_qa` | `STATE_TASK_RUNNING_BAR` | 审核运行中 | 项目标识统一为 `acf17-review-001` |
| `04_final_qa` | `D2_REVIEW_V2` | 结果总览 | 项目标识统一；重写入口禁用并标注 I18 |
| `05_final_qa` | `I17_D2_REVIEW_ISSUE_DETAIL` | 全文定位 | 重写入口禁用并标注 I18 |
| `06_final_qa` | `STATE_TASK_FAILED_NOTICE` | 审核失败 | 删除原始 JSON/内部节点，改为脱敏技术摘要 |
| `07_final_qa` | `STATE_NOT_CONFIGURED_EMPTY` | 未配置 | 增加 Frame 元数据 |
| `08_final_qa` | `I17_D2_REVIEW_HISTORY` | 审核历史 | 增加 Frame 元数据 |

## 3. 使用规则

- `screen.png` 冻结 AppShell、主要布局、信息层级、状态语义与操作位置。
- `code.html` 用于读取组件结构和文案，不代表生产技术栈。
- 示例数据、ID、日期和内容不得硬编码进生产代码。
- 03 的同一布局支持 queued/running 文案变体；06 支持三类失败变体。
- 04/05 的重写入口在 Iteration 17 为禁用交接提示，Iteration 18 再启用。
- 不以像素差作为唯一验收；AppShell 漂移、状态错误、可执行越界和安全泄漏必须修复。
