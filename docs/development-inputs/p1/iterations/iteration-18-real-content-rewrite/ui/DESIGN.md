# ACF Iteration 18 — 真实正文重写 UI 设计基线

**状态：`rebuild_candidate_2026_07_29`。** 本文件与 9 个 `screen.png` 共同构成 Iteration 18 的候选视觉基线；CF-18-01 冻结契约后再转为正式开发输入。

## 1. AppShell 保护

- 生产实现必须复用仓库现有 ACF 桌面 AppShell、左侧导航、顶部工具栏、项目头和项目 Tab，不得从 Stitch HTML 重建第二套 Shell。
- 原型画布为 1600×1280，对应 1280×1024 CSS 视口、1.25 device scale；浏览器验收应采用可比视口。
- 只有中央业务内容允许按 Iteration 18 变化。全局导航名称、顺序、宽度和真实版本号以生产 AppShell 为准。
- 主工作区使用浅灰背景、白色卡片、细边框、紧凑企业 SaaS 密度和现有紫色主操作。
- 禁止渐变、玻璃拟态、插画化后台、第二侧栏、深色导航或截图专用容器。

## 2. 重写业务视觉规则

- 重写从一个明确 ReviewReport、用户选中的 `open` ReviewIssue 和不可变来源 ContentVersion 发起。
- 运行成功并完成结果消费后才创建一个新的候选 ContentVersion；来源版本保持不变。
- 候选默认不是当前版本，不自动触发审核或发布。
- 成功、Runtime 失败、输出校验失败、结果消费失败必须是不同状态。
- `result_consumption_failed` 时不得展示候选版本、编辑器入口或“设为当前版本”，只能展示“重试提交”和执行详情。
- 项目配置抽屉默认关闭，只读显示当前绑定；修改配置统一进入项目工作流设置。
- 结果页只有一个提升操作：“设为当前版本”，随后展示确认弹窗。
- 不增加“下载草稿”“分享预览”“确认并应用 v5”等越界操作。
- 09 是互斥状态验收板，不是最终产品中同时展示四种状态的页面。

## 3. 契约修正优先级

- 原型中的示例 ID、时间、模型、章节、正文和技术版本不得硬编码进生产代码。
- ReviewIssue 持久化处置只允许 `open/ignored`；原型中“待解决/进行中/待处理”等示例标签不得扩展后端枚举，生产文案统一映射为“待处理/已忽略”。
- 原型中的“流程/工作流”、项目版本号等 Shell 文案不覆盖生产 AppShell。
- 技术标识可保留英文；用户可见业务文案必须进入现有 locale。
- Material Symbols 的 icon name 仅用于图标渲染，不得作为可见文字。

## 4. Frame 顺序

01. `I18_D2_REVIEW_REWRITE_ENTRY` — 审核结果选择问题并创建重写
02. `I18_D4_CREATE_REWRITE` — 创建正文重写
03. `I18_D4_REWRITE_CONFIG_DRAWER` — 查看项目重写配置
04. `I18_D4_REWRITE_RUNNING` — 排队/运行中
05. `I18_D5_REWRITE_RESULT` — 成功候选版本
06. `I18_D5_SET_CURRENT_CONFIRM` — 设为当前版本确认
07. `I18_D5_RESULT_CONSUMPTION_FAILED` — 结果消费失败
08. `I18_D4_REWRITE_FAILED` — Runtime/输出校验失败
09. `I18_D4_REWRITE_AVAILABILITY` — 未配置、配置失效与无记录
