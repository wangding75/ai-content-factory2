# Iteration 18 — Stitch 原型来源映射

**状态：`rebuild_candidate_2026_07_29`。**

## 1. 来源

- 上传包：`stitch_acf_iteration_18_ui_real_content_rewrite_frozen_shell.zip`
- 原始画布：9 张，1600×1280
- 原始设计说明：`acf_iteration_18_frozen_rewrite_ui/DESIGN.md`
- 仓库候选设计：`ui/DESIGN.md`

复制后，仓库内 `screen.png` 和 `code.html` 是 Iteration 18 的唯一原型输入，不再依赖上传 ZIP 路径。

## 2. 映射

| 原始目录 | Frame ID | 页面/状态 | 必要解释 |
|---|---|---|---|
| `01` | `I18_D2_REVIEW_REWRITE_ENTRY` | 审核结果：选择问题并创建重写 | 1600×1280；生产复用现有 AppShell |
| `02` | `I18_D4_CREATE_REWRITE` | 创建正文重写 | 1600×1280；生产复用现有 AppShell |
| `03` | `I18_D4_REWRITE_CONFIG_DRAWER` | 查看项目重写配置 | 1600×1280；生产复用现有 AppShell |
| `04` | `I18_D4_REWRITE_RUNNING` | 正文重写运行中 | 1600×1280；生产复用现有 AppShell |
| `05` | `I18_D5_REWRITE_RESULT` | 正文重写成功候选 | 1600×1280；生产复用现有 AppShell |
| `06` | `I18_D5_SET_CURRENT_CONFIRM` | 设为当前版本确认 | 1600×1280；生产复用现有 AppShell |
| `07` | `I18_D5_RESULT_CONSUMPTION_FAILED` | 重写结果提交失败 | 1600×1280；生产复用现有 AppShell |
| `08` | `I18_D4_REWRITE_FAILED` | 重写任务执行失败 | 1600×1280；生产复用现有 AppShell |
| `09` | `I18_D4_REWRITE_AVAILABILITY` | 未配置、配置失效与空状态 | 1600×1280；生产复用现有 AppShell |

## 3. 必要开发修正

- 01 复用 Iteration 17 Report 页面并启用真实“创建重写任务”，不得复制一套平行审核页。
- 03 是 02 的按需抽屉，不是独立路由。
- 04 同一布局支持 queued/running。
- 06 是 05 上的确认弹窗，不是独立产品页。
- 07 只对应 result_consumption_failed：零候选、仅消费 Retry。
- 08 支持 runtime_failed/output_validation_failed；两者均走 Runtime Retry，但文案不同。
- 09 是互斥状态验收板，最终页面一次只显示真实原因。
- 原型状态列不得扩展 Iteration 17 的 open/ignored 枚举。
- Stitch HTML 只用于结构/文案分析；生产必须使用现有组件、Token、Locale 和 AppShell。
