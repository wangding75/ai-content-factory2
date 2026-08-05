# 本地原型 Frame 分析清单 (Prototype Inventory)

本清单分析了 Iteration 12 ~ 19 下的本地 UI 原型 Frame、有效性以及升级替代关系。

| 迭代版本 | 原型 Frame 标识 | 有效性 | 升级替代关系与说明 |
|---|---|---|---|
| Iteration 12 | `GLOBAL_SETTINGS_LLM_LIST_V2` | 升级替代 | 被 Iteration 19 的 `I19_01_LLM_PROVIDER_LIST` 替代 |
| Iteration 12 | `GLOBAL_SETTINGS_CONNECTION_LIST_V2` | 升级替代 | 被 Iteration 19 的 `I19_03_N8N_CONNECTION_LIST` 替代 |
| Iteration 12 | `GLOBAL_SETTINGS_WORKFLOW_LIST_V2` | 升级替代 | 被 Iteration 19 的 `I19_05_WORKFLOW_CONFIGURATION_LIST` 替代 |
| Iteration 12 | `LLM_CONFIG_DRAWER_V2` | 升级替代 | 被 Iteration 19 的 `I19_02_LLM_PROVIDER_DRAWER` 替代 |
| Iteration 12 | `CONNECTION_DRAWER_V2` | 升级替代 | 被 Iteration 19 的 `I19_04_N8N_CONNECTION_DRAWER` 替代 |
| Iteration 12 | `WORKFLOW_DRAWER_V2` | 升级替代 | 被 Iteration 19 的 `I19_06_WORKFLOW_CONFIGURATION_DRAWER` 替代 |
| Iteration 13 | `P13_03_SELECT_WORKFLOW_DRAWER` | 升级替代 | 被 Iteration 19 的 `I19_09_SELECT_WORKFLOW_DRAWER` 共享抽屉替代 |
| Iteration 13 | `P13_05_REPLACE_WORKFLOW_DRAWER` | 升级替代 | 被 Iteration 19 的 `I19_09_SELECT_WORKFLOW_DRAWER` 共享抽屉替代 |
| Iteration 13 | `P13_07_WORKFLOW_BINDINGS_COMPLETE` | 升级替代 | 被 Iteration 19 的 `I19_07_PROJECT_BINDINGS_EXECUTABLE` 替代 |
| Iteration 13 | `P13_09_WORKFLOW_BINDING_EXCEPTIONS` | 升级替代 | 被 Iteration 19 的 `I19_08_PROJECT_BINDINGS_DEPENDENCY_INVALID` 替代 |
| Iteration 14 | `P14_01_WORKFLOW_RUN_LIST` | 升级替代 | 被 Iteration 19 的 `I19_10_WORKFLOW_RUN_LIST` 替代 |
| Iteration 14 | `P14_05_WORKFLOW_RUN_DETAIL_SUCCEEDED` | 升级替代 | 被 Iteration 19 的 `I19_11_WORKFLOW_RUN_DETAIL_PAGE` 替代 |
| Iteration 15 | `P15_C3_PREFLIGHT_BLOCKED` | 升级替代 | 被 Iteration 19 的 `I19_14_SHARED_PREFLIGHT_BLOCKED` 共享阻断替代 |
| Iteration 16 | `I16_D2_GENERATE_CONFIRM` | 升级替代 | 被 Iteration 19 的 `I19_14_SHARED_PREFLIGHT_BLOCKED` 共享阻断替代 |
| Iteration 17 | `D2_SUBMIT_REVIEW_DRAWER` | 升级替代 | 被 Iteration 19 的 `I19_14_SHARED_PREFLIGHT_BLOCKED` 共享阻断替代 |
| Iteration 18 | `I18_D4_REWRITE_AVAILABILITY` | 升级替代 | 被 Iteration 19 的 `I19_14_SHARED_PREFLIGHT_BLOCKED` 共享阻断替代 |
| Iteration 19 | `I19_01` ~ `I19_15` 系列 | **有效** | 最新冻结的集成验收帧，覆盖最新 UI 升级 |
| Iteration 12 | `GLOBAL_SETTINGS_LLM_EMPTY_V2` | **有效** | LLM 配置空状态 |
| Iteration 12 | `GLOBAL_SETTINGS_CONNECTION_EMPTY_V2` | **有效** | Connections 配置空状态 |
| Iteration 12 | `GLOBAL_SETTINGS_WORKFLOW_EMPTY_V2` | **有效** | Workflows 配置空状态 |
| Iteration 12 | `GLOBAL_SETTINGS_DISTRIBUTION_LIST_V2` | **有效** | 分发平台列表 |
| Iteration 12 | `DISTRIBUTION_DRAWER_V2` | **有效** | 分发平台新增/编辑抽屉 |
| Iteration 12 | `GLOBAL_SETTINGS_DISTRIBUTION_EMPTY_V2` | **有效** | 分发平台空状态 |
| Iteration 13 | `P13_02_WORKFLOW_BINDINGS_UNBOUND` | **有效** | 绑定未配置状态 |
| Iteration 13 | `P13_04_WORKFLOW_BINDINGS_PARTIAL` | **有效** | 部分绑定状态 |
| Iteration 13 | `P13_06_UNBIND_CONFIRM_DIALOG` | **有效** | 解绑确认弹窗 |
| Iteration 13 | `P13_08_NO_AVAILABLE_WORKFLOW` | **有效** | 绑定页面无候选状态 |
| Iteration 13 | `P13_10_BINDING_CONFLICT` | **有效** | 绑定保存并发冲突 |
| Iteration 14 | `P14_02_PROJECT_FILTERED` | **有效** | 项目过滤列表 |
| Iteration 14 | `P14_03_WORKFLOW_RUN_LIST_EMPTY` | **有效** | 运行记录列表空状态 |
| Iteration 14 | `P14_04_WORKFLOW_RUN_LIST_ERROR` | **有效** | 运行记录加载失败状态 |
| Iteration 14 | `P14_06_WORKFLOW_RUN_DETAIL_RUNNING` | **有效** | 运行记录运行中详情 |
| Iteration 14 | `P14_07_WORKFLOW_RUN_DETAIL_FAILED` | **有效** | 运行记录失败详情 |
| Iteration 14 | `P14_10_PROJECT_WORKFLOW_RUN_SUMMARY`| **有效** | 项目概览中流程摘要 |
| Iteration 15 | `P15_C1` ~ `P15_C10` 系列 | **有效** | 章节规划相关主流程状态、差异对比、弹窗等 |
| Iteration 16 | `I16_D1` ~ `I16_D5` 系列 | **有效** | 正文生成编辑器 Tab、运行中/排队中/失败/候选对比状态 |
| Iteration 17 | `I17_D1` ~ `I17_D2` 系列 | **有效** | 质量评审入口、总览、定位及历史记录 |
| Iteration 18 | `I18_D2` ~ `I18_D5` 系列 | **有效** | 正文重写入口、创建、运行、成功改写及消费失败 |