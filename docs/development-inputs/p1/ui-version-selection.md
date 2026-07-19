# 第二用户闭环 UI 原型版本选择记录

## 1. 文档目的

本文档记录 AI Content Factory 2.0 第二用户闭环 UI 原型的候选版本、最终开发基准版本及选择依据，用于：

- 固化 Iteration 11 UI 原型选择结果；
- 关联 `docs/development-inputs/p1` 中的开发输入；
- 避免后续开发误用早期候选稿；
- 为 Git 历史、代码评审和 UI 验收提供可追溯依据。

## 2. UI 验收结论

Iteration 11 UI 人工验收结论：

> **有条件通过（CONDITIONAL_PASS）**

已知问题：

- 部分用户可见文案仍为英文，尤其是侧边一级菜单、状态、按钮、表头和辅助说明；
- 开发时必须将用户可见业务文案接入中文 locale/i18n 资源；
- 默认中文环境不得直接照搬原型中的英文业务文案；
- `Run ID`、`Workflow ID`、Schema 版本、模型名称、Provider 名称等必要技术标识可以保留英文。

建议统一的一级菜单中文文案：

- 首页
- 项目
- 素材
- 作品
- 工作流
- 设置

上述问题属于开发修正项，不要求重新生成 Stitch 原型，但属于前端开发和最终人工验收的强制门禁。

## 3. 版本选择原则

原始 Stitch 压缩包中的文件时间戳一致，且未包含 Stitch Screen ID、创建时间、更新时间或明确的版本链。因此，无法仅依赖压缩包元数据证明某个候选是 Stitch 画布中最后生成的版本。

当前开发基准按以下顺序确定：

1. 与 Iteration 10/11 已冻结业务定义一致；
2. 显式版本号更高，例如 `_2`、`_3`、`v2.0`；
3. 页面信息、交互状态和业务闭环更完整；
4. 与其他已选页面的视觉和导航结构更一致；
5. 以当前 `ui-master-manifest.json` 和各迭代 `ui-manifest.json` 实际归档来源为最终依据。

因此，本文中的“选择版本”表示：

> **已验收并归档的推荐开发基准版本，不等同于通过 Stitch 时间元数据证明的最后生成版本。**

## 4. 页面与 UI 版本映射

| UI 页面 | 涉及原型 | 选择的版本 |
|---|---|---|
| `E4_AI_CONNECTIONS_OVERVIEW` AI 与工作流连接总览 | `ai_connections_overview_1`；`ai_connections_overview_2`；`ai_content_factory_workflow_management` | `ai_connections_overview_2` |
| `E4_LLM_PROVIDER_DRAWER` LLM Provider 配置抽屉 | `e4_llm_provider_drawer` | `e4_llm_provider_drawer` |
| `E4_N8N_CONNECTION_DRAWER` n8n 连接配置抽屉 | `e4_n8n_connection_drawer` | `e4_n8n_connection_drawer` |
| `S02_PROJECT_WORKFLOW_SETTINGS` 项目工作流设置 | `project_workflow_settings` | `project_workflow_settings` |
| `S02_BIND_CHAPTER_WORKFLOW_DRAWER` 章节规划工作流绑定 | `s02_bind_chapter_workflow_drawer` | `s02_bind_chapter_workflow_drawer` |
| `S02_BIND_CONTENT_WORKFLOW_DRAWER` 正文生成工作流绑定 | `s02_bind_content_workflow_drawer` | `s02_bind_content_workflow_drawer` |
| `S02_BIND_REVIEW_WORKFLOW_DRAWER` 内容审核工作流绑定 | `s02_bind_review_workflow_drawer` | `s02_bind_review_workflow_drawer` |
| `S02_BIND_REWRITE_WORKFLOW_DRAWER` 正文重写工作流绑定 | `s02_bind_rewrite_workflow_drawer` | `s02_bind_rewrite_workflow_drawer` |
| `E3_WORKFLOW_CENTER_V2` 工作流运行中心 | `workflow_center_monitoring_console`；`workflow_center_dashboard`；`workflow_center_monitoring_console_v2.0`；`workflow_center_console_v2.0` | `workflow_center_console_v2.0` |
| `E3_WORKFLOW_RUN_DETAIL_DRAWER` 工作流运行详情 | `e3_workflow_run_detail_drawer` | `e3_workflow_run_detail_drawer` |
| `E3_RETRY_RUN_CONFIRM_DIALOG` 工作流重试确认 | `retry_run_confirmation_dialog` | `retry_run_confirmation_dialog` |
| `C1_CHAPTER_PLANNING_V2` 章节规划页 | `c1_chapter_planning_v2_chapter_planning_console`；`chapter_planning_console_v2.0` | `chapter_planning_console_v2.0` |
| `C2_GENERATE_CHAPTER_PLAN_DRAWER_V2` 生成章节规划抽屉 | `generate_chapter_plan_drawer` | `generate_chapter_plan_drawer` |
| `D1_EDITOR_V2` 正文编辑器 | `d1_editor_v2_ai_editor_console_1`；`chapter_editor_console_v2.0`；`d1_editor_v2_ai_editor_console_2`；`d1_editor_v2_ai_editor_console_3` | `d1_editor_v2_ai_editor_console_3` |
| `D1_GENERATE_CONTENT_DRAWER` 生成正文抽屉 | `generate_content_drawer_ai_content_factory_2.0` | `generate_content_drawer_ai_content_factory_2.0` |
| `D2_REVIEW_V2` 内容审核页 | `content_review_console_v2.0` | `content_review_console_v2.0` |
| `D2_SUBMIT_REVIEW_DRAWER` 提交审核抽屉 | `submit_content_review_drawer` | `submit_content_review_drawer` |
| `D4_CREATE_REWRITE_V2` 创建重写页 | `d1_editor_v2_create_rewrite_console` | `d1_editor_v2_create_rewrite_console` |
| `D5_REWRITE_RESULT_V2` 重写结果页 | `d5_rewrite_result_v2_rewrite_result_console_1`；`d5_rewrite_result_v2_rewrite_result_console_2` | `d5_rewrite_result_v2_rewrite_result_console_2` |
| `STATE_TASK_RUNNING_BAR` 任务运行状态条 | `component_spec_state_task_running_bar` | `component_spec_state_task_running_bar` |
| `STATE_TASK_FAILED_NOTICE` 任务失败提示 | `component_spec_state_task_failed_notice` | `component_spec_state_task_failed_notice` |
| `STATE_NOT_CONFIGURED_EMPTY` 未配置/空状态 | `component_spec_state_not_configured_empty` | `component_spec_state_not_configured_empty` |

## 5. 多版本页面选择说明

### 5.1 AI 与工作流连接总览

候选：

- `ai_connections_overview_1`
- `ai_connections_overview_2`
- `ai_content_factory_workflow_management`

选择：

- `ai_connections_overview_2`

原因：

- 属于明确递增的第二版；
- 与最终“全局设置仅管理 LLM Provider 与 n8n Connection”的定义更一致；
- 页面层级和信息结构更适合作为连接总览开发基准。

### 5.2 工作流运行中心

候选：

- `workflow_center_monitoring_console`
- `workflow_center_dashboard`
- `workflow_center_monitoring_console_v2.0`
- `workflow_center_console_v2.0`

选择：

- `workflow_center_console_v2.0`

原因：

- 更符合 Workflow Center 作为运行管理页面的定位；
- 覆盖 WorkflowRun 列表、状态、筛选、详情入口和管理动作；
- 与运行详情抽屉、重试确认弹窗的组合关系更完整。

### 5.3 章节规划

候选：

- `c1_chapter_planning_v2_chapter_planning_console`
- `chapter_planning_console_v2.0`

选择：

- `chapter_planning_console_v2.0`

原因：

- 与第二闭环“真实章节规划”的运行和结果展示更匹配；
- 信息层级更完整；
- 与 `C2_GENERATE_CHAPTER_PLAN_DRAWER_V2` 的发起流程衔接更清晰。

### 5.4 正文编辑器

候选：

- `d1_editor_v2_ai_editor_console_1`
- `chapter_editor_console_v2.0`
- `d1_editor_v2_ai_editor_console_2`
- `d1_editor_v2_ai_editor_console_3`

选择：

- `d1_editor_v2_ai_editor_console_3`

原因：

- 属于同命名分支中的最高编号版本；
- 正文编辑、真实生成状态、版本信息和后续审核入口更完整；
- 与正文生成抽屉、审核页面和重写页面的闭环关系更明确。

### 5.5 重写结果

候选：

- `d5_rewrite_result_v2_rewrite_result_console_1`
- `d5_rewrite_result_v2_rewrite_result_console_2`

选择：

- `d5_rewrite_result_v2_rewrite_result_console_2`

原因：

- 属于明确递增的第二版；
- 新旧版本关系、重写结果和后续处理动作表达更完整。

## 6. 未选候选稿处理

以下候选未作为开发基准 Frame，但原始 Stitch 压缩包应继续保留，以支持回溯：

| 未选原型 | 处理结果 |
|---|---|
| `ai_connections_overview_1` | 早期版本，不作为开发基准 |
| `ai_content_factory_workflow_management` | 连接/工作流管理的早期设计分支 |
| `workflow_center_monitoring_console` | 工作流中心早期候选 |
| `workflow_center_dashboard` | 工作流中心仪表盘候选 |
| `workflow_center_monitoring_console_v2.0` | 工作流中心中间版本 |
| `c1_chapter_planning_v2_chapter_planning_console` | 章节规划早期版本 |
| `d1_editor_v2_ai_editor_console_1` | 正文编辑器版本 1 |
| `chapter_editor_console_v2.0` | 正文编辑器另一设计分支 |
| `d1_editor_v2_ai_editor_console_2` | 正文编辑器版本 2 |
| `d5_rewrite_result_v2_rewrite_result_console_1` | 重写结果版本 1 |
| `ai_content_factory_2.0_desktop/DESIGN.md` | Stitch 设计说明，不属于独立 UI 页面 |

## 7. 开发使用规则

1. 开发默认使用各迭代 `ui/frames/<FRAME_ID>/screen.png` 和 `code.html`。
2. `ui-master-manifest.json` 用于查看 Frame 在不同迭代中的复用关系。
3. 各迭代的 `ui-manifest.json` 是该迭代实际使用的原型清单。
4. 不得自行切换到原始压缩包中的未选版本。
5. 如必须变更版本，应同步更新：
   - 本文档；
   - `ui-master-manifest.json`；
   - 对应迭代的 `ui-manifest.json`；
   - 对应 `ui/frames` 下的 `screen.png` 和 `code.html`；
   - UI 验收结论和变更原因。
6. 原型英文文案不构成开发文案标准；用户可见文案以产品中文语义和 locale/i18n 资源为准。

## 8. 归档位置

本文档应提交到：

```text
docs/development-inputs/p1/ui-version-selection.md
```

相关原型与清单位于：

```text
docs/development-inputs/p1/ui-master-manifest.json
docs/development-inputs/p1/iterations/*/ui-manifest.json
docs/development-inputs/p1/iterations/*/ui/frames/
```

## 9. Iteration 12 全局设置 V2 开发基准覆盖

Iteration 12 已使用新 Stitch 项目 `AI Content Factory 2.0｜全局设置 V2` 的上传包重新组织。此前 Iteration 12 使用的 `E4_AI_CONNECTIONS_OVERVIEW`、`E4_LLM_PROVIDER_DRAWER`、`E4_N8N_CONNECTION_DRAWER` 和共享状态 Frame 不再作为 Iteration 12 的直接开发基准；它们仍保留在 Iteration 11 历史冻结记录中。

Iteration 12 当前开发基准为 12 个规范化 Frame：

| 领域 | 空状态 | 列表态 | 抽屉态 |
|---|---|---|---|
| LLM | `GLOBAL_SETTINGS_LLM_EMPTY_V2` | `GLOBAL_SETTINGS_LLM_LIST_V2` | `LLM_CONFIG_DRAWER_V2` |
| 连接 | `GLOBAL_SETTINGS_CONNECTION_EMPTY_V2` | `GLOBAL_SETTINGS_CONNECTION_LIST_V2` | `CONNECTION_DRAWER_V2` |
| 工作流 | `GLOBAL_SETTINGS_WORKFLOW_EMPTY_V2` | `GLOBAL_SETTINGS_WORKFLOW_LIST_V2` | `WORKFLOW_DRAWER_V2` |
| 分发平台 | `GLOBAL_SETTINGS_DISTRIBUTION_EMPTY_V2` | `GLOBAL_SETTINGS_DISTRIBUTION_LIST_V2` | `DISTRIBUTION_DRAWER_V2` |

这些原型的已知问题不再通过重生成 Stitch 修复，而是在开发阶段按 `iteration-12-global-execution-connections/prototype-review-and-development-fixes.md` 和 `acceptance.md` 强制修复。
