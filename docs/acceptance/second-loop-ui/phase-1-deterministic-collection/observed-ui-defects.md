# Observed UI Defects

Generated: 2026-08-06T05:48:09.286Z

## UI-016
- **页面**: 项目设置 / 工作流绑定 / 绑定保留但依赖失效
- **Scene**: BINDINGS_INVALID
- **实际状态**: AI Content Factory 内容创作平台 首页 项目 素材 作品 流程 设置 CA 创作管理员 项目 / ACF UI Acceptance Project ACF UI Acceptance Project 小说 策划中 更新于 2026年1月1日 16:00 Deterministic project used only for second-loop UI acceptance scenes. 概览 策划 素材 故事线 章节规划 审核 作品 设置 基本信息 工作流绑定 其他设置 工作流绑定 为项目的四个环节分别选择可复用的工作流。绑定不代表该工作流已经验证或可以执行。 4 / 
- **截图**: screenshots/UI-016_I19_08_PROJECT_BINDINGS_DEPENDENCY_INVALID.png
- **capture_status**: COLLECTED_DEFECT
- **备注**: 选择现有配置失效项目；不可复现则记录 ; failures: None of required texts found: [配置已变更, 不可执行]
- **是否阻止后续精确原型对比**: 否

## UI-020
- **页面**: 项目设置 / 工作流绑定 / 当前阶段无可用工作流
- **Scene**: BINDINGS_NO_AVAILABLE
- **实际状态**: AI Content Factory 内容创作平台 首页 项目 素材 作品 流程 设置 CA 创作管理员 项目 / ACF UI Acceptance Project ACF UI Acceptance Project 小说 策划中 更新于 2026年1月1日 16:00 Deterministic project used only for second-loop UI acceptance scenes. 概览 策划 素材 故事线 章节规划 审核 作品 设置 基本信息 工作流绑定 其他设置 工作流绑定 为项目的四个环节分别选择可复用的工作流。绑定不代表该工作流已经验证或可以执行。 3 / 
- **截图**: screenshots/UI-020_P13_08_NO_AVAILABLE_WORKFLOW.png
- **capture_status**: COLLECTED_DEFECT
- **备注**: 使用现有无可用候选状态；不可复现则记录 ; failures: None of required texts found: [暂无可用工作流, 前往全局设置]
- **是否阻止后续精确原型对比**: 否

## UI-021
- **页面**: 项目设置 / 工作流绑定 / 保存并发冲突
- **Scene**: BINDINGS_CONFLICT_BASE
- **实际状态**: AI Content Factory 内容创作平台 首页 项目 素材 作品 流程 设置 CA 创作管理员 项目 / ACF UI Acceptance Project ACF UI Acceptance Project 小说 策划中 更新于 2026年1月1日 16:00 Deterministic project used only for second-loop UI acceptance scenes. 概览 策划 素材 故事线 章节规划 审核 作品 设置 基本信息 工作流绑定 其他设置 工作流绑定 为项目的四个环节分别选择可复用的工作流。绑定不代表该工作流已经验证或可以执行。 4 / 
- **截图**: screenshots/UI-021_P13_10_BINDING_CONFLICT.png
- **capture_status**: COLLECTED_DEFECT
- **备注**: 在隔离验收库中通过受控版本递增触发真实 409 并发冲突；只修改固定验收绑定。 ; failures: None of required texts found: [配置已被更新, 冲突, 重新加载] | Target component not visible for state_kind=dialog
- **是否阻止后续精确原型对比**: 否

## UI-024
- **页面**: 流程中心 / WorkflowRun / 列表空状态
- **Scene**: BASELINE
- **实际状态**: AI Content Factory 内容创作平台 首页 项目 素材 作品 流程 设置 CA 创作管理员 流程中心 / 运行记录 流程中心 统一查看所有项目的工作流运行情况。 搜索 所属项目 全部项目 ACF UI Acceptance Project ACF Fixture Novel ACF Fixture Short Film 业务环节 全部环节 章节规划 内容生成 审核 改写 状态 全部状态 等待执行 运行中 已成功 失败 已取消 时间范围 全部时间 今天 最近 7 天 最近 30 天 重置筛选 运行编号 项目 业务环节 状态 触发来源 创建时间 更新时间 操作 ACF-UI-DYNA
- **截图**: screenshots/UI-024_P14_03_WORKFLOW_RUN_LIST_EMPTY.png
- **capture_status**: COLLECTED_DEFECT
- **备注**: 用无结果筛选复现，不修改数据 ; failures: None of required texts found: [暂无运行记录, 清除筛选]
- **是否阻止后续精确原型对比**: 否

## UI-025
- **页面**: 流程中心 / WorkflowRun / 列表错误状态
- **Scene**: API_STOPPED
- **实际状态**: AI Content Factory 内容创作平台 首页 项目 素材 作品 流程 设置 CA 创作管理员 流程中心 / 运行记录 流程中心 统一查看所有项目的工作流运行情况。 搜索 所属项目 全部项目 业务环节 全部环节 章节规划 内容生成 审核 改写 状态 全部状态 等待执行 运行中 已成功 失败 已取消 时间范围 全部时间 今天 最近 7 天 最近 30 天 重置筛选 运行记录加载失败 暂时无法获取工作流运行记录，请稍后重试。 重新加载
- **截图**: screenshots/UI-025_P14_04_WORKFLOW_RUN_LIST_ERROR.png
- **capture_status**: COLLECTED_DEFECT
- **备注**: 由场景脚本停止 API，截图错误态后立即切换回 BASELINE。 ; failures: Target component not visible for state_kind=list
- **是否阻止后续精确原型对比**: 否

## UI-028
- **页面**: 流程中心 / WorkflowRun / 失败详情
- **Scene**: BASELINE
- **实际状态**: AI Content Factory 内容创作平台 首页 项目 素材 作品 流程 设置 CA 创作管理员 ← 返回运行记录 流程中心 / 运行记录 运行详情 执行失败 内容生成 · 手动触发 基本信息 运行编号 ACF-FIX-FAILED-0001 业务环节 内容生成 触发来源 手动触发 当前版本 v1 创建时间 2026年1月1日 15:20 更新时间 2026年1月1日 15:20 输入参数 {} 输出结果 暂无信息 安全错误信息 {} 配置快照 {} 事件时间线 已创建运行 等待执行 · 2026年1月1日 15:20 {} 开始执行 运行中 · 2026年1月1日 15:20 {} 
- **截图**: screenshots/UI-028_P14_07_WORKFLOW_RUN_DETAIL_FAILED.png
- **capture_status**: COLLECTED_DEFECT
- **备注**: 选择 failed Run ; demoted: screenshot hash not unique across capture_ids
- **是否阻止后续精确原型对比**: 否

## UI-029
- **页面**: 流程中心 / WorkflowRun / Retry 确认弹窗
- **Scene**: BASELINE
- **实际状态**: AI Content Factory 内容创作平台 首页 项目 素材 作品 流程 设置 CA 创作管理员 ← 返回运行记录 流程中心 / 运行记录 运行详情 执行失败 内容生成 · 手动触发 基本信息 运行编号 ACF-FIX-FAILED-0001 业务环节 内容生成 触发来源 手动触发 当前版本 v1 创建时间 2026年1月1日 15:20 更新时间 2026年1月1日 15:20 输入参数 {} 输出结果 暂无信息 安全错误信息 {} 配置快照 {} 事件时间线 已创建运行 等待执行 · 2026年1月1日 15:20 {} 开始执行 运行中 · 2026年1月1日 15:20 {} 
- **截图**: screenshots/UI-029_I19_12_RETRY_CONFIRM_DIALOG.png
- **capture_status**: COLLECTED_DEFECT
- **备注**: 在可重试 Run 打开确认弹窗，不提交 ; failures: None of required texts found: [重试, 当前配置, 原配置] | Target component not visible for state_kind=dialog | Required click not found: click_missed:重试,click_missed:重试
- **是否阻止后续精确原型对比**: 否

## UI-037
- **页面**: 章节规划 / 章节规划 / 章节状态与统计数据变体
- **Scene**: BASELINE
- **实际状态**: AI Content Factory 内容创作平台 首页 项目 素材 作品 流程 设置 CA 创作管理员 项目 / ACF Fixture Novel ACF Fixture Novel 小说 生产中 更新于 2026年1月1日 16:00 Deterministic end-to-end fixture project. 概览 策划 素材 故事线 章节规划 审核 作品 设置 章节规划 新增章节 auto_awesome 生成章节规划 当前章节 2 候选批次 1 article 全部章节 2 pending_actions 待确认 0 check_circle 已确认 2 edit_docum
- **截图**: screenshots/UI-037_P15_C1_RUNNING_DATA_VARIANT.png
- **capture_status**: COLLECTED_DEFECT
- **备注**: 使用现有数据变体 ; demoted: screenshot hash not unique across capture_ids
- **是否阻止后续精确原型对比**: 否

## UI-038
- **页面**: 章节规划 / 章节规划 / 失败且候选零写入
- **Scene**: CP_FAILED
- **实际状态**: AI Content Factory 内容创作平台 首页 项目 素材 作品 流程 设置 CA 创作管理员 项目 / ACF Fixture Novel ACF Fixture Novel 小说 生产中 更新于 2026年1月1日 16:00 Deterministic end-to-end fixture project. 概览 策划 素材 故事线 章节规划 审核 作品 设置 章节规划 新增章节 auto_awesome 生成章节规划 当前章节 2 候选批次 1 article 全部章节 2 pending_actions 待确认 0 check_circle 已确认 2 edit_docum
- **截图**: screenshots/UI-038_P15_C1_FAILED_ATOMIC.png
- **capture_status**: COLLECTED_DEFECT
- **备注**: 选择对应失败记录 ; failures: None of required texts found: [生成失败, 未写入]
- **是否阻止后续精确原型对比**: 否

## UI-039
- **页面**: 章节规划 / 章节规划 / 工作流未配置
- **Scene**: CP_NOT_CONFIGURED
- **实际状态**: AI Content Factory 内容创作平台 首页 项目 素材 作品 流程 设置 CA 创作管理员 项目 / ACF UI Acceptance Project ACF UI Acceptance Project 小说 策划中 更新于 2026年1月1日 16:00 Deterministic project used only for second-loop UI acceptance scenes. 概览 策划 素材 故事线 章节规划 审核 作品 设置 章节规划 新增章节 auto_awesome 生成章节规划 当前章节 0 候选批次 0 article 全部章节 0 pending
- **截图**: screenshots/UI-039_P15_C1_NOT_CONFIGURED.png
- **capture_status**: COLLECTED_DEFECT
- **备注**: 选择现有未配置项目 ; failures: None of required texts found: [未配置, 工作流]
- **是否阻止后续精确原型对比**: 否

## UI-040
- **页面**: 章节规划 / 章节规划 / 生成参数抽屉
- **Scene**: BASELINE
- **实际状态**: AI Content Factory 内容创作平台 首页 项目 素材 作品 流程 设置 CA 创作管理员 项目 / ACF Fixture Novel ACF Fixture Novel 小说 生产中 更新于 2026年1月1日 16:00 Deterministic end-to-end fixture project. 概览 策划 素材 故事线 章节规划 审核 作品 设置 章节规划 新增章节 auto_awesome 生成章节规划 当前章节 2 候选批次 1 article 全部章节 2 pending_actions 待确认 0 check_circle 已确认 2 edit_docum
- **截图**: screenshots/UI-040_P15_C2_GENERATION_SETTINGS.png
- **capture_status**: COLLECTED_DEFECT
- **备注**: 打开生成抽屉，不提交 ; demoted: screenshot hash not unique across capture_ids
- **是否阻止后续精确原型对比**: 否

## UI-041
- **页面**: 章节规划 / 章节规划 / 预检进度弹窗
- **Scene**: BINDINGS_READY
- **实际状态**: AI Content Factory 内容创作平台 首页 项目 素材 作品 流程 设置 CA 创作管理员 项目 / ACF Fixture Novel ACF Fixture Novel 小说 生产中 更新于 2026年1月1日 16:00 Deterministic end-to-end fixture project. 概览 策划 素材 故事线 章节规划 审核 作品 设置 章节规划 新增章节 auto_awesome 生成章节规划 当前章节 2 候选批次 1 article 全部章节 2 pending_actions 待确认 0 check_circle 已确认 2 edit_docum
- **截图**: screenshots/UI-041_P15_C2_PREFLIGHT_PROGRESS.png
- **capture_status**: COLLECTED_DEFECT
- **备注**: 打开生成抽屉后启动受控表锁，再发起预检；截图后必须 RELEASE_HOLD。 ; failures: None of required texts found: [预检中, 正在检查, 请稍候]
- **是否阻止后续精确原型对比**: 否

## UI-042
- **页面**: 章节规划 / 章节规划 / 预检通过报告
- **Scene**: BINDINGS_READY
- **实际状态**: AI Content Factory 内容创作平台 首页 项目 素材 作品 流程 设置 CA 创作管理员 项目 / ACF Fixture Novel ACF Fixture Novel 小说 生产中 更新于 2026年1月1日 16:00 Deterministic end-to-end fixture project. 概览 策划 素材 故事线 章节规划 审核 作品 设置 章节规划 新增章节 auto_awesome 生成章节规划 当前章节 2 候选批次 1 article 全部章节 2 pending_actions 待确认 0 check_circle 已确认 2 edit_docum
- **截图**: screenshots/UI-042_P15_C3_PREFLIGHT_PASS.png
- **capture_status**: COLLECTED_DEFECT
- **备注**: 运行只读预检或使用现有状态，不创建 Run ; failures: None of required texts found: [预检通过, 可以开始]
- **是否阻止后续精确原型对比**: 否

## UI-043
- **页面**: 章节规划 / 章节规划 / 预检阻断报告
- **Scene**: PREFLIGHT_BLOCK_CP
- **实际状态**: AI Content Factory 内容创作平台 首页 项目 素材 作品 流程 设置 CA 创作管理员 项目 / ACF Fixture Novel ACF Fixture Novel 小说 生产中 更新于 2026年1月1日 16:00 Deterministic end-to-end fixture project. 概览 策划 素材 故事线 章节规划 审核 作品 设置 章节规划 新增章节 auto_awesome 生成章节规划 当前章节 2 候选批次 1 article 全部章节 2 pending_actions 待确认 0 check_circle 已确认 2 edit_docum
- **截图**: screenshots/UI-043_P15_C3_PREFLIGHT_BLOCKED.png
- **capture_status**: COLLECTED_DEFECT
- **备注**: 使用现有阻断状态 ; failures: None of required texts found: [预检未通过, 阻断原因]
- **是否阻止后续精确原型对比**: 否

## UI-044
- **页面**: 章节规划 / 章节规划 / 任务创建成功提示
- **Scene**: BINDINGS_READY
- **实际状态**: AI Content Factory 内容创作平台 首页 项目 素材 作品 流程 设置 CA 创作管理员 项目 / ACF Fixture Novel ACF Fixture Novel 小说 生产中 更新于 2026年1月1日 16:00 Deterministic end-to-end fixture project. 概览 策划 素材 故事线 章节规划 审核 作品 设置 章节规划 新增章节 auto_awesome 生成章节规划 当前章节 2 候选批次 1 article 全部章节 2 pending_actions 待确认 0 check_circle 已确认 2 edit_docum
- **截图**: screenshots/UI-044_P15_C3_RUN_CREATED.png
- **capture_status**: COLLECTED_DEFECT
- **备注**: 允许在隔离验收数据中创建一条真实章节规划 Run；下一场景会完整重载 Seed 并移除该临时 Run。 ; failures: None of required texts found: [任务已创建, 查看运行] | Required click not found: click_missed:创建任务|开始生成|确认生成,click_missed:创建任务|开始生成|确认生成
- **是否阻止后续精确原型对比**: 否

## UI-051
- **页面**: 章节规划 / 候选批次 / 候选基线过期冲突
- **Scene**: CANDIDATE_STALE
- **实际状态**: AI Content Factory 内容创作平台 首页 项目 素材 作品 流程 设置 CA 创作管理员 暂时无法加载项目
- **截图**: screenshots/UI-051_P15_C10_STALE_CONFLICT_DIALOG.png
- **capture_status**: ROUTE_FAILED
- **备注**: 仅使用现有冲突状态；不得制造写冲突 ; failures: Scene switch failed: CANDIDATE_STALE
- **是否阻止后续精确原型对比**: 否

## UI-053
- **页面**: 正文生成 / 正文编辑器 / 章节目标 Tab
- **Scene**: BASELINE
- **实际状态**: AI Content Factory 内容创作平台 首页 项目 素材 作品 流程 设置 CA 创作管理员 项目 / ACF Fixture Novel ACF Fixture Novel 小说 生产中 更新于 2026年1月1日 16:00 Deterministic end-to-end fixture project. 概览 策划 素材 故事线 章节规划 审核 作品 设置 正文生成未完成 Run ID: ACF-UI-DYNAMIC-CG 正文生成任务未完成 查看详情 重新执行 Runtime 章节导航 第 2 章 The Red Index Card 查看章节规划 第 2 章《The R
- **截图**: screenshots/UI-053_I16_D1_EDITOR_CHAPTER_GOAL.png
- **capture_status**: COLLECTED_DEFECT
- **备注**: 打开编辑器并选择章节目标 ; demoted: screenshot hash not unique across capture_ids
- **是否阻止后续精确原型对比**: 否

## UI-056
- **页面**: 正文生成 / 正文编辑器 / 生成正文运行前确认抽屉
- **Scene**: BASELINE
- **实际状态**: AI Content Factory 内容创作平台 首页 项目 素材 作品 流程 设置 CA 创作管理员 项目 / ACF Fixture Novel ACF Fixture Novel 小说 生产中 更新于 2026年1月1日 16:00 Deterministic end-to-end fixture project. 概览 策划 素材 故事线 章节规划 审核 作品 设置 正文生成未完成 Run ID: ACF-UI-DYNAMIC-CG 正文生成任务未完成 查看详情 重新执行 Runtime 章节导航 第 2 章 The Red Index Card 查看章节规划 第 2 章《The R
- **截图**: screenshots/UI-056_I16_D2_GENERATE_CONFIRM.png
- **capture_status**: COLLECTED_DEFECT
- **备注**: 打开生成正文抽屉，不提交 ; failures: Required click not found: click_missed:生成正文
- **是否阻止后续精确原型对比**: 否

## UI-057
- **页面**: 正文生成 / 正文编辑器 / 生成正文已填写要求
- **Scene**: BASELINE
- **实际状态**: AI Content Factory 内容创作平台 首页 项目 素材 作品 流程 设置 CA 创作管理员 项目 / ACF Fixture Novel ACF Fixture Novel 小说 生产中 更新于 2026年1月1日 16:00 Deterministic end-to-end fixture project. 概览 策划 素材 故事线 章节规划 审核 作品 设置 正文生成未完成 Run ID: ACF-UI-DYNAMIC-CG 正文生成任务未完成 查看详情 重新执行 Runtime 章节导航 第 2 章 The Red Index Card 查看章节规划 第 2 章《The R
- **截图**: screenshots/UI-057_I16_D2_GENERATE_REQUIREMENTS.png
- **capture_status**: COLLECTED_DEFECT
- **备注**: 使用现有填写状态；不得提交 ; failures: None of required texts found: [保持克制的悬疑语气, 1800]
- **是否阻止后续精确原型对比**: 否

## UI-061
- **页面**: 正文生成 / 正文编辑器 / 正文生成失败
- **Scene**: CG_FAILED
- **实际状态**: AI Content Factory 内容创作平台 首页 项目 素材 作品 流程 设置 CA 创作管理员 项目 / ACF Fixture Novel ACF Fixture Novel 小说 生产中 更新于 2026年1月1日 16:00 Deterministic end-to-end fixture project. 概览 策划 素材 故事线 章节规划 审核 作品 设置 正文生成未完成 Run ID: ACF-UI-DYNAMIC-CG 正文生成任务未完成 收起详情 重新执行 Runtime 任务失败2026/1/1 19:10:00 章节导航 第 2 章 The Red Index C
- **截图**: screenshots/UI-061_I16_D3_RUN_FAILED.png
- **capture_status**: COLLECTED_DEFECT
- **备注**: 选择失败记录 ; failures: None of required texts found: [生成失败, 重试]
- **是否阻止后续精确原型对比**: 否

## UI-062
- **页面**: 正文生成 / 正文编辑器 / 正文候选版本比较
- **Scene**: BASELINE
- **实际状态**: AI Content Factory 内容创作平台 首页 项目 素材 作品 流程 设置 CA 创作管理员 项目 / ACF Fixture Novel ACF Fixture Novel 小说 生产中 更新于 2026年1月1日 16:00 Deterministic end-to-end fixture project. 概览 策划 素材 故事线 章节规划 审核 作品 设置 正文生成未完成 Run ID: ACF-UI-DYNAMIC-CG 正文生成任务未完成 查看详情 重新执行 Runtime 章节导航 第 2 章 The Red Index Card 查看章节规划 第 2 章《The R
- **截图**: screenshots/UI-062_I16_D4_CANDIDATE_VERSION.png
- **capture_status**: COLLECTED_DEFECT
- **备注**: 打开已有候选版本 ; failures: None of required texts found: [当前版本, 候选版本, 比较] | Required click not found: click_missed:查看候选|比较版本,click_missed:查看候选|比较版本
- **是否阻止后续精确原型对比**: 否

## UI-063
- **页面**: 正文生成 / 正文编辑器 / 正文生成工作流未配置
- **Scene**: CG_NOT_CONFIGURED
- **实际状态**: AI Content Factory 内容创作平台 首页 项目 素材 作品 流程 设置 CA 创作管理员 项目 / ACF Fixture Novel ACF Fixture Novel 小说 生产中 更新于 2026年1月1日 16:00 Deterministic end-to-end fixture project. 概览 策划 素材 故事线 章节规划 审核 作品 设置 正文生成未完成 Run ID: ACF-UI-DYNAMIC-CG 正文生成任务未完成 查看详情 重新执行 Runtime 章节导航 第 2 章 The Red Index Card 查看章节规划 第 2 章《The R
- **截图**: screenshots/UI-063_I16_D5_NOT_CONFIGURED.png
- **capture_status**: COLLECTED_DEFECT
- **备注**: 选择未配置项目 ; failures: None of required texts found: [未配置, 正文生成工作流]
- **是否阻止后续精确原型对比**: 否

## UI-064
- **页面**: 内容审核 / 审核入口 / 正文编辑器提交审核入口
- **Scene**: BASELINE
- **实际状态**: AI Content Factory 内容创作平台 首页 项目 素材 作品 流程 设置 CA 创作管理员 项目 / ACF Fixture Novel ACF Fixture Novel 小说 生产中 更新于 2026年1月1日 16:00 Deterministic end-to-end fixture project. 概览 策划 素材 故事线 章节规划 审核 作品 设置 正文生成未完成 Run ID: ACF-UI-DYNAMIC-CG 正文生成任务未完成 查看详情 重新执行 Runtime 章节导航 第 2 章 The Red Index Card 查看章节规划 第 2 章《The R
- **截图**: screenshots/UI-064_I17_D1_EDITOR_REVIEW_ENTRY.png
- **capture_status**: COLLECTED_DEFECT
- **备注**: 打开已保存正文 ; failures: None of required texts found: [提交审核]
- **是否阻止后续精确原型对比**: 否

## UI-065
- **页面**: 内容审核 / 审核入口 / 发起内容审核抽屉
- **Scene**: BASELINE
- **实际状态**: AI Content Factory 内容创作平台 首页 项目 素材 作品 流程 设置 CA 创作管理员 项目 / ACF Fixture Novel ACF Fixture Novel 小说 生产中 更新于 2026年1月1日 16:00 Deterministic end-to-end fixture project. 概览 策划 素材 故事线 章节规划 审核 作品 设置 正文生成未完成 Run ID: ACF-UI-DYNAMIC-CG 正文生成任务未完成 查看详情 重新执行 Runtime 章节导航 第 2 章 The Red Index Card 查看章节规划 第 2 章《The R
- **截图**: screenshots/UI-065_D2_SUBMIT_REVIEW_DRAWER.png
- **capture_status**: COLLECTED_DEFECT
- **备注**: 打开审核抽屉，不提交 ; failures: None of required texts found: [发起审核, 审核范围] | Required click not found: click_missed:提交审核|发起审核,click_missed:提交审核|发起审核
- **是否阻止后续精确原型对比**: 否

## UI-067
- **页面**: 内容审核 / 审核工作区 / 审核结果总览
- **Scene**: REVIEW_SUCCEEDED
- **实际状态**: This page couldn’t load Reload to try again, or go back. Reload Back
- **截图**: screenshots/UI-067_D2_REVIEW_V2.png
- **capture_status**: COLLECTED_DEFECT
- **备注**: 选择 review_ready 报告 ; failures: None of required texts found: [审核结果, 问题列表]
- **是否阻止后续精确原型对比**: 否

## UI-068
- **页面**: 内容审核 / 审核工作区 / 问题详情与全文定位
- **Scene**: REVIEW_SUCCEEDED
- **实际状态**: This page couldn’t load Reload to try again, or go back. Reload Back
- **截图**: screenshots/UI-068_I17_D2_REVIEW_ISSUE_DETAIL.png
- **capture_status**: COLLECTED_DEFECT
- **备注**: 选择有定位信息的 Issue ; failures: None of required texts found: [问题详情, 全文定位]
- **是否阻止后续精确原型对比**: 否

## UI-072
- **页面**: 正文重写 / 重写工作区 / 选择问题并创建重写入口
- **Scene**: BASELINE
- **实际状态**: This page couldn’t load Reload to try again, or go back. Reload Back
- **截图**: screenshots/UI-072_I18_D2_REVIEW_REWRITE_ENTRY.png
- **capture_status**: COLLECTED_DEFECT
- **备注**: 选择可处理 Issue，不创建任务 ; failures: None of required texts found: [创建重写, 已选择问题]
- **是否阻止后续精确原型对比**: 否

## UI-073
- **页面**: 正文重写 / 重写工作区 / 创建正文重写
- **Scene**: BASELINE
- **实际状态**: AI Content Factory 内容创作平台 首页 项目 素材 作品 流程 设置 CA 创作管理员 暂时无法读取重写状态 暂时无法读取重写状态，请稍后重试。 重试
- **截图**: screenshots/UI-073_I18_D4_CREATE_REWRITE.png
- **capture_status**: COLLECTED_DEFECT
- **备注**: 打开创建页，不提交 ; failures: None of required texts found: [创建正文重写, 所选问题]
- **是否阻止后续精确原型对比**: 否

## UI-074
- **页面**: 正文重写 / 重写工作区 / 查看项目重写配置抽屉
- **Scene**: BASELINE
- **实际状态**: AI Content Factory 内容创作平台 首页 项目 素材 作品 流程 设置 CA 创作管理员 暂时无法读取重写状态 暂时无法读取重写状态，请稍后重试。 重试
- **截图**: screenshots/UI-074_I18_D4_REWRITE_CONFIG_DRAWER.png
- **capture_status**: COLLECTED_DEFECT
- **备注**: 打开配置抽屉 ; failures: None of required texts found: [重写配置, 工作流] | Required click not found: click_missed:查看配置|重写配置,click_missed:查看配置|重写配置
- **是否阻止后续精确原型对比**: 否

## UI-075
- **页面**: 正文重写 / 重写工作区 / 正文重写运行中
- **Scene**: REWRITE_RUNNING
- **实际状态**: AI Content Factory 内容创作平台 首页 项目 素材 作品 流程 设置 CA 创作管理员 运行中 重写任务正在运行 正在依据固定来源版本生成候选正文。 固定来源版本 V2 · The Red Index Card 审核报告 已固定 已选问题 1 个 当前运行 ACF-UI-DYNAMIC-REWRITE 执行进度 未知运行事件 · 2026年1月1日 19:30 取消运行 刷新状态
- **截图**: screenshots/UI-075_I18_D4_REWRITE_RUNNING.png
- **capture_status**: COLLECTED_DEFECT
- **备注**: 选择 queued/running 重写 ; failures: None of required texts found: [重写中, 运行详情]
- **是否阻止后续精确原型对比**: 否

## UI-076
- **页面**: 正文重写 / 重写工作区 / 正文重写成功候选
- **Scene**: REWRITE_SUCCEEDED
- **实际状态**: AI Content Factory 内容创作平台 首页 项目 素材 作品 流程 设置 CA 创作管理员 配置检查 项目重写配置不可用 前往项目设置检查工作流与连接。 返回审核结果 刷新可用性
- **截图**: screenshots/UI-076_I18_D5_REWRITE_RESULT.png
- **capture_status**: COLLECTED_DEFECT
- **备注**: 选择 candidate_ready 重写 ; failures: None of required texts found: [重写候选, 设为当前版本]
- **是否阻止后续精确原型对比**: 否

## UI-077
- **页面**: 正文重写 / 重写工作区 / 设为当前版本确认
- **Scene**: REWRITE_SUCCEEDED
- **实际状态**: AI Content Factory 内容创作平台 首页 项目 素材 作品 流程 设置 CA 创作管理员 配置检查 项目重写配置不可用 前往项目设置检查工作流与连接。 返回审核结果 刷新可用性
- **截图**: screenshots/UI-077_I18_D5_SET_CURRENT_CONFIRM.png
- **capture_status**: COLLECTED_DEFECT
- **备注**: 打开确认弹窗，不确认 ; failures: None of required texts found: [设为当前版本, 确认] | Target component not visible for state_kind=dialog | Required click not found: click_missed:设为当前版本,click_missed:设为当前版本
- **是否阻止后续精确原型对比**: 否

## UI-078
- **页面**: 正文重写 / 重写工作区 / 重写结果提交失败
- **Scene**: REWRITE_RESULT_CONSUMPTION_FAILED
- **实际状态**: AI Content Factory 内容创作平台 首页 项目 素材 作品 流程 设置 CA 创作管理员 暂时无法读取重写状态 暂时无法读取重写状态，请稍后重试。 重试
- **截图**: screenshots/UI-078_I18_D5_RESULT_CONSUMPTION_FAILED.png
- **capture_status**: COLLECTED_DEFECT
- **备注**: 选择 result_consumption_failed ; demoted: screenshot hash not unique across capture_ids
- **是否阻止后续精确原型对比**: 否

## UI-079
- **页面**: 正文重写 / 重写工作区 / 重写任务执行失败
- **Scene**: REWRITE_FAILED
- **实际状态**: AI Content Factory 内容创作平台 首页 项目 素材 作品 流程 设置 CA 创作管理员 暂时无法读取重写状态 暂时无法读取重写状态，请稍后重试。 重试
- **截图**: screenshots/UI-079_I18_D4_REWRITE_FAILED.png
- **capture_status**: COLLECTED_DEFECT
- **备注**: 选择 runtime/output validation failed ; failures: None of required texts found: [重写失败, 错误原因]
- **是否阻止后续精确原型对比**: 否

## UI-080
- **页面**: 正文重写 / 重写工作区 / 未配置/配置失效/空状态
- **Scene**: REWRITE_UNAVAILABLE
- **实际状态**: AI Content Factory 内容创作平台 首页 项目 素材 作品 流程 设置 CA 创作管理员 暂时无法读取重写状态 暂时无法读取重写状态，请稍后重试。 重试
- **截图**: screenshots/UI-080_I18_D4_REWRITE_AVAILABILITY.png
- **capture_status**: COLLECTED_DEFECT
- **备注**: 选择对应现有不可用状态 ; failures: None of required texts found: [不可执行, 配置已变更]
- **是否阻止后续精确原型对比**: 否

## UI-081
- **页面**: 共享升级状态 / 章节规划 / 共享预检阻断
- **Scene**: PREFLIGHT_BLOCK_CP
- **实际状态**: AI Content Factory 内容创作平台 首页 项目 素材 作品 流程 设置 CA 创作管理员 项目 / ACF Fixture Novel ACF Fixture Novel 小说 生产中 更新于 2026年1月1日 16:00 Deterministic end-to-end fixture project. 概览 策划 素材 故事线 章节规划 审核 作品 设置 章节规划 新增章节 auto_awesome 生成章节规划 当前章节 2 候选批次 1 article 全部章节 2 pending_actions 待确认 0 check_circle 已确认 2 edit_docum
- **截图**: screenshots/UI-081_I19_14_SHARED_PREFLIGHT_BLOCKED.png
- **capture_status**: COLLECTED_DEFECT
- **备注**: 打开章节规划发起区的阻断状态 ; demoted: shared screenshot hash with UI-040
- **是否阻止后续精确原型对比**: 否

## UI-082
- **页面**: 共享升级状态 / 正文生成 / 共享预检阻断
- **Scene**: PREFLIGHT_BLOCK_CG
- **实际状态**: AI Content Factory 内容创作平台 首页 项目 素材 作品 流程 设置 CA 创作管理员 项目 / ACF Fixture Novel ACF Fixture Novel 小说 生产中 更新于 2026年1月1日 16:00 Deterministic end-to-end fixture project. 概览 策划 素材 故事线 章节规划 审核 作品 设置 正文生成未完成 Run ID: ACF-UI-DYNAMIC-CG 正文生成任务未完成 查看详情 重新执行 Runtime 章节导航 第 2 章 The Red Index Card 查看章节规划 第 2 章《The R
- **截图**: screenshots/UI-082_I19_14_SHARED_PREFLIGHT_BLOCKED.png
- **capture_status**: COLLECTED_DEFECT
- **备注**: 打开正文生成抽屉的阻断状态 ; demoted: shared screenshot hash with UI-053
- **是否阻止后续精确原型对比**: 否

## UI-083
- **页面**: 共享升级状态 / 内容审核 / 共享预检阻断
- **Scene**: PREFLIGHT_BLOCK_REVIEW
- **实际状态**: AI Content Factory 内容创作平台 首页 项目 素材 作品 流程 设置 CA 创作管理员 项目 / ACF Fixture Novel ACF Fixture Novel 小说 生产中 更新于 2026年1月1日 16:00 Deterministic end-to-end fixture project. 概览 策划 素材 故事线 章节规划 审核 作品 设置 正文生成未完成 Run ID: ACF-UI-DYNAMIC-CG 正文生成任务未完成 查看详情 重新执行 Runtime 章节导航 第 2 章 The Red Index Card 查看章节规划 第 2 章《The R
- **截图**: screenshots/UI-083_I19_14_SHARED_PREFLIGHT_BLOCKED.png
- **capture_status**: COLLECTED_DEFECT
- **备注**: 打开审核抽屉的阻断状态 ; failures: None of required texts found: [预检未通过, 内容审核]
- **是否阻止后续精确原型对比**: 否

## UI-084
- **页面**: 共享升级状态 / 正文重写 / 共享预检阻断
- **Scene**: PREFLIGHT_BLOCK_REWRITE
- **实际状态**: AI Content Factory 内容创作平台 首页 项目 素材 作品 流程 设置 CA 创作管理员 暂时无法读取重写状态 暂时无法读取重写状态，请稍后重试。 重试
- **截图**: screenshots/UI-084_I19_14_SHARED_PREFLIGHT_BLOCKED.png
- **capture_status**: COLLECTED_DEFECT
- **备注**: 打开重写页的阻断状态 ; failures: None of required texts found: [预检未通过, 正文重写]
- **是否阻止后续精确原型对比**: 否

## UI-085
- **页面**: 共享升级状态 / 章节规划 / 共享运行恢复
- **Scene**: CP_TIMED_OUT
- **实际状态**: AI Content Factory 内容创作平台 首页 项目 素材 作品 流程 设置 CA 创作管理员 项目 / ACF Fixture Novel ACF Fixture Novel 小说 生产中 更新于 2026年1月1日 16:00 Deterministic end-to-end fixture project. 概览 策划 素材 故事线 章节规划 审核 作品 设置 章节规划 新增章节 auto_awesome 生成章节规划 当前章节 2 候选批次 1 article 全部章节 2 pending_actions 待确认 0 check_circle 已确认 2 edit_docum
- **截图**: screenshots/UI-085_I19_15_SHARED_RUNTIME_RECOVERY.png
- **capture_status**: COLLECTED_DEFECT
- **备注**: 选择取消/超时/校验或消费失败状态 ; failures: None of required texts found: [超时, 恢复]
- **是否阻止后续精确原型对比**: 否

## UI-086
- **页面**: 共享升级状态 / 正文生成 / 共享运行恢复
- **Scene**: CG_RESULT_CONSUMPTION_FAILED
- **实际状态**: AI Content Factory 内容创作平台 首页 项目 素材 作品 流程 设置 CA 创作管理员 项目 / ACF Fixture Novel ACF Fixture Novel 小说 生产中 更新于 2026年1月1日 16:00 Deterministic end-to-end fixture project. 概览 策划 素材 故事线 章节规划 审核 作品 设置 正文生成未完成 Run ID: ACF-UI-DYNAMIC-CG 正文生成结果处理失败 查看详情 重试结果消费 章节导航 第 2 章 The Red Index Card 查看章节规划 第 2 章《The Red In
- **截图**: screenshots/UI-086_I19_15_SHARED_RUNTIME_RECOVERY.png
- **capture_status**: COLLECTED_DEFECT
- **备注**: 选择取消/超时/校验或消费失败状态 ; failures: None of required texts found: [结果消费失败, 恢复]
- **是否阻止后续精确原型对比**: 否

## UI-088
- **页面**: 共享升级状态 / 正文重写 / 共享运行恢复
- **Scene**: REWRITE_TIMED_OUT
- **实际状态**: AI Content Factory 内容创作平台 首页 项目 素材 作品 流程 设置 CA 创作管理员 暂时无法读取重写状态 暂时无法读取重写状态，请稍后重试。 重试
- **截图**: screenshots/UI-088_I19_15_SHARED_RUNTIME_RECOVERY.png
- **capture_status**: COLLECTED_DEFECT
- **备注**: 选择取消/超时/校验或消费失败状态 ; failures: None of required texts found: [超时, 恢复]
- **是否阻止后续精确原型对比**: 否

