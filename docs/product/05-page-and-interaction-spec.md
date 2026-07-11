# AI Content Factory 2.0｜页面与交互规范

## 1. 页面注册表

| Frame ID | 页面 | 上下文 | 核心职责 |
|---|---|---|---|
| S00_HOME | 首页 | 全局 | 业务概览、最近项目和主要入口 |
| S01_PROJECTS | 项目列表 | 全局 | 查询和进入项目 |
| S01_CREATE_PROJECT | 创建项目 | 全局 | 创建 Novel 项目 |
| S02_PROJECT_OVERVIEW | 项目概览 | 项目 | 当前项目摘要和生产入口 |
| S02_PROJECT_PLANNING | 项目策划 | 项目 | 编辑项目策划字段 |
| S02_PROJECT_MATERIALS | 项目素材 | 项目 | 管理项目素材用途 |
| S02_CREATE_MATERIAL | 创建素材 | 项目 | 创建全局素材并绑定项目 |
| S02_MATERIAL_DETAIL | 素材详情 | 项目 | 展示本体与项目用途 |
| S02_PICK_MATERIAL | 选择素材 | 项目 | 绑定已有全局素材 |
| S02_EDIT_MATERIAL | 编辑素材 | 项目 | 编辑全局素材本体 |
| S02_EDIT_MATERIAL_USAGE | 编辑用途 | 项目 | 编辑当前项目用途 |
| S02_UNBIND_MATERIAL | 解除绑定 | 项目 | 删除 Usage，不删除 Material |
| B1_STORYLINES | 故事线 | 项目 | 展示故事线树和伏笔 |
| B2_CREATE_MAIN_STORYLINE | 创建主线 | 项目 | 创建根 PlotLine |
| B3_CREATE_CHILD_STORYLINE | 创建子线 | 项目 | 创建子 PlotLine |
| B4_CREATE_FORESHADOWING | 新增伏笔 | 项目 | 创建 Foreshadowing |
| C1_CHAPTER_PLANNING | 章节规划 | 项目 | 候选、确认和生产入口 |
| C2_MOCK_PLAN | 模拟规划 | 项目 | 触发 Mock 候选生成 |
| C3_EDIT_PLAN | 编辑规划 | 项目 | 编辑 pending 候选 |
| C4_CONFIRM_PLAN | 确认规划 | 项目 | 确认候选，不生成正文 |
| D1_EDITOR | 正文编辑器 | 项目 | 编辑当前版本和提交审核 |
| D2_REVIEW | 审核结果 | 项目 | 展示审核报告和重写入口 |
| D3_PROJECT_WORKS | 项目作品 | 项目 | 聚合项目内容、版本和审核 |
| D4_CREATE_REWRITE | 创建重写 | 项目 | 配置并启动 Mock 重写 |
| D5_REWRITE_RESULT | 重写结果 | 项目 | 展示新版本及返回入口 |
| E1_GLOBAL_MATERIALS | 全局素材 | 全局 | 聚合素材及引用项目 |
| E2_GLOBAL_WORKS | 全局作品 | 全局 | 聚合作品及源项目 |
| E3_WORKFLOWS | 流程中心 | 全局 | 内置流程和运行记录 |
| E4_SETTINGS | 设置 | 全局 | 能力、集成和禁用状态 |

## 2. 页面状态

所有列表页至少包含：加载、空、正常、筛选无结果、加载失败。

所有表单页至少包含：初始、输入中、字段错误、提交中、提交失败、提交成功。

所有异步任务页至少包含：queued、running、succeeded、failed。

## 3. 禁用能力

真实 AI、外部工作流和发布适配器必须：

- 显示真实状态：未配置或未开放。
- 禁止点击后伪造执行成功。
- 可展示未来说明，但不得创建虚假 WorkflowRun。

## 4. 视觉实现原则

- 冻结 Frame 决定页面结构、信息层级和主要控件语义。
- 可以组件化和响应式重构，但不得删除业务字段和入口。
- 任何视觉变更不得改变状态机或操作结果。
- UI 评审以截图对比和真实链路验收共同判断，不能只看静态像素。
