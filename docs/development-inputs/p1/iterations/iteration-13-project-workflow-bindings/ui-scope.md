# Iteration 13 — 项目四环节工作流绑定 — UI Scope

## 1. 选定原型

采用首次生成的 Iteration 13 Stitch 版本 `P13-01` 至 `P13-10`。该版本的卡片层级、抽屉结构、状态覆盖和商业化完成度作为开发基线。

验收结论：**有条件通过**。

原型中的应用外壳不是现有产品真实壳层，开发时必须复用现有 `ProjectWorkspace`，不得直接复制截图或 HTML 的左侧导航、顶部栏、项目头部和面包屑。

## 2. Frame 映射

| Frame | 区域/状态 | 用途 | 截图 | HTML | 开发要求 |
|---|---|---|---|---|---|
| `P13_01_PROJECT_SETTINGS_ENTRY` | 项目设置入口 | 壳层、内层 Tab 和入口参考 | `ui/frames/P13_01_PROJECT_SETTINGS_ENTRY/screen.png` | `ui/frames/P13_01_PROJECT_SETTINGS_ENTRY/code.html` | 仅参考；不新增基础信息编辑 |
| `P13_02_WORKFLOW_BINDINGS_UNBOUND` | 全部未绑定 | 四环节总览 | `ui/frames/P13_02_WORKFLOW_BINDINGS_UNBOUND/screen.png` | `ui/frames/P13_02_WORKFLOW_BINDINGS_UNBOUND/code.html` | 必须实现 |
| `P13_03_SELECT_WORKFLOW_DRAWER` | 首次绑定 | 搜索、候选和确认 | `ui/frames/P13_03_SELECT_WORKFLOW_DRAWER/screen.png` | `ui/frames/P13_03_SELECT_WORKFLOW_DRAWER/code.html` | 必须实现 |
| `P13_04_WORKFLOW_BINDINGS_PARTIAL` | 部分已绑定 | 已绑定与未绑定混合状态 | `ui/frames/P13_04_WORKFLOW_BINDINGS_PARTIAL/screen.png` | `ui/frames/P13_04_WORKFLOW_BINDINGS_PARTIAL/code.html` | 必须实现 |
| `P13_05_REPLACE_WORKFLOW_DRAWER` | 更换绑定 | 当前绑定与新候选 | `ui/frames/P13_05_REPLACE_WORKFLOW_DRAWER/screen.png` | `ui/frames/P13_05_REPLACE_WORKFLOW_DRAWER/code.html` | 必须实现 |
| `P13_06_UNBIND_CONFIRM_DIALOG` | 解除绑定 | 危险操作确认 | `ui/frames/P13_06_UNBIND_CONFIRM_DIALOG/screen.png` | `ui/frames/P13_06_UNBIND_CONFIRM_DIALOG/code.html` | 必须实现 |
| `P13_07_WORKFLOW_BINDINGS_COMPLETE` | 全部已绑定 | 完整配置状态 | `ui/frames/P13_07_WORKFLOW_BINDINGS_COMPLETE/screen.png` | `ui/frames/P13_07_WORKFLOW_BINDINGS_COMPLETE/code.html` | 必须实现 |
| `P13_08_NO_AVAILABLE_WORKFLOW` | 无候选 | 引导前往全局设置 | `ui/frames/P13_08_NO_AVAILABLE_WORKFLOW/screen.png` | `ui/frames/P13_08_NO_AVAILABLE_WORKFLOW/code.html` | 必须实现 |
| `P13_09_WORKFLOW_BINDING_EXCEPTIONS` | 依赖异常 | 已停用、未接入、连接异常 | `ui/frames/P13_09_WORKFLOW_BINDING_EXCEPTIONS/screen.png` | `ui/frames/P13_09_WORKFLOW_BINDING_EXCEPTIONS/code.html` | 必须实现 |
| `P13_10_BINDING_CONFLICT` | 409 冲突 | 保留选择并加载最新配置 | `ui/frames/P13_10_BINDING_CONFLICT/screen.png` | `ui/frames/P13_10_BINDING_CONFLICT/code.html` | 必须实现 |

## 3. 页面位置与入口

主页面位于现有项目工作区：

`项目 → 当前项目 → 设置 → 工作流绑定`

实现规则：

1. 现有项目工作区负责左侧导航、顶部工具栏、项目标题、项目状态和一级项目 Tab；
2. Iteration 13 组件只能渲染设置内容区，不能再次渲染外壳；
3. 内层 Tab 固定为“基本信息 / 工作流绑定 / 其他设置”，名称与顺序保持一致；
4. `P13-01` 只作为设置内容区与入口参考，本迭代不开发图标上传、项目基础信息保存、权限管理或其他设置；
5. 项目策划页可在策划完成后显示轻量入口；项目概览只动态改变“下一步建议”，两者均跳转到同一个工作流绑定路由，不新增 Frame。

## 4. 必须修正的原型细节

### 4.1 外壳与布局

- 使用现有产品 Logo、品牌副标题、一级导航、搜索框、用户区和项目头部；
- 左侧导航沿用现有术语“流程”，不得复制原型中的“工作流”；
- 不出现双侧栏、双顶部栏、双项目标题、双面包屑或双项目 Tab；
- 内容宽度跟随现有项目工作区；不得硬编码为原型的 1600×1280 画布尺寸；
- 1280px 以上使用 2×2 卡片，较窄桌面宽度允许退化为单列；
- 卡片等高、操作区底部对齐。

### 4.2 术语

| 原型可能出现 | 开发统一文案 |
|---|---|
| 章节策划 | 章节规划 |
| 工作流程绑定 / 工作流设置 | 工作流绑定 |
| 基础信息 | 基本信息 |
| 权限管理 | 其他设置 |
| 尚未绑定工作流 | 尚未绑定 |
| 未启用 | 已停用 |

四个环节固定为：**章节规划、内容生成、审核、改写**。

### 4.3 状态语义

- 绑定状态：未绑定 / 已绑定；
- 全局工作流状态：已启用 / 已停用；
- 集成状态：已集成 / 未接入；
- 连接状态：正常 / 连接异常；
- 状态必须同时使用文字和图标/色彩，不得只靠颜色；
- “已绑定”不能使用表示“执行成功”的视觉或文案；
- 已绑定工作流后续异常时保留绑定并显示原因，不自动解绑。

### 4.4 总览卡片

- 固定顺序：章节规划 → 内容生成 → 审核 → 改写；
- 已绑定卡展示工作流名称、类型、连接名称、启用状态和集成状态；
- 长名称省略并提供完整名称 Tooltip；
- 无连接时显示“无关联连接”，不得显示 `--`；
- 未绑定卡主操作为“选择工作流”；
- 已绑定卡操作为“更换工作流”和“解除绑定”；
- 健康绑定的“更换工作流”不得无原因禁用；
- 解除操作使用克制的危险样式，不能与主操作争夺视觉焦点。

### 4.5 选择与更换抽屉

- 宽度建议 520–560px，最大宽度不超过视口；
- Header 和 Footer 固定，候选列表单独滚动；
- 搜索使用防抖并调用真实服务端查询；
- 候选项必须按 `applicableStage` 过滤；
- 已停用工作流可展示但不可选，并明确原因；
- “未接入”和连接状态作为提示信息，不自动伪装成可执行；
- 更换抽屉必须标记“当前绑定”；
- 未选择新项时“保存更换”禁用；
- 提交期间按钮禁用，防止重复请求；
- 抽屉支持键盘选择、焦点锁定、Escape 关闭和关闭前状态清理。

### 4.6 无候选、异常和冲突

- 无候选状态说明产生原因，并跳转到全局设置的工作流页面；
- 跳转携带当前项目返回地址；
- 异常总数和卡片原因来自真实数据；
- 409 时保留用户当前选择；
- “加载最新配置”重新读取绑定和候选数据，抽屉保持打开；
- 409 不得自动覆盖服务端，也不得把原始错误码、堆栈或后端消息直接展示给用户。

### 4.7 文案与国际化

- 用户可见业务文案进入现有 locale/i18n 资源；
- `n8n`、`Python Script` 等技术类型可保留；
- 环境名属于用户配置数据，可按原值展示；
- 原型中的 `Search...`、`Main Production Env` 等示例不得作为硬编码产品文案；
- 不出现字面量 `\uXXXX`。

## 5. 加载和错误状态

原型未完整覆盖但开发必须实现：

- 总览首次加载 Skeleton；
- 候选列表加载、空结果、请求失败与重试；
- 保存中、解除中和重复提交防护；
- 字段错误、名称/依赖冲突和通用错误区；
- 刷新后从服务端恢复；
- Console 无新增 Error，Network 无非预期 4xx/5xx。

## 6. 验收结论

首版 UI 可以作为 Iteration 13 开发基线，但必须按本文件修正。任何直接复制原型外壳、保留“章节策划”、新增工作流执行能力或把“已绑定”显示为“可执行”的实现均不得通过验收。
