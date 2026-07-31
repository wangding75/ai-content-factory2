# Iteration 19 — Stitch 原型来源映射

**状态：`final_freeze_rebuild_2026_07_31`。**

## 1. 来源

- 上传包：`stitch_acf_iteration_19_ui_final_freeze_rebuild(1).zip`；
- 原始正式画布：15 个业务编号，另有 `04_n8n_2` 重复候选；
- 画布尺寸：全部 1600×1280；
- 设计说明：`acf_enterprise_desktop/DESIGN.md`，归档为 `ui/source/DESIGN.md`。

仓库内 Frame 是唯一开发输入，不再依赖上传 ZIP 路径。

## 2. 映射

| 原始目录 | Frame ID | 页面/状态 | 选择说明 |
|---|---|---|---|
| `01_llm_provider` | `I19_01_LLM_PROVIDER_LIST` | Provider 列表 | 正式 |
| `02_llm_provider` | `I19_02_LLM_PROVIDER_DRAWER` | Provider 抽屉 | 正式 |
| `03_n8n` | `I19_03_N8N_CONNECTION_LIST` | n8n 列表 | 正式 |
| `04_n8n_1` | `I19_04_N8N_CONNECTION_DRAWER` | n8n 抽屉 | 选择该版；实例根 URL 和依赖影响表达完整 |
| `05_workflow_configuration` | `I19_05_WORKFLOW_CONFIGURATION_LIST` | Workflow 列表 | 正式 |
| `06_workflow_configuration` | `I19_06_WORKFLOW_CONFIGURATION_DRAWER` | Workflow 抽屉 | 正式 |
| `07` | `I19_07_PROJECT_BINDINGS_EXECUTABLE` | 绑定正常态 | 正式 |
| `08` | `I19_08_PROJECT_BINDINGS_DEPENDENCY_INVALID` | 绑定失效态 | 正式 |
| `09` | `I19_09_SELECT_WORKFLOW_DRAWER` | 共享换绑抽屉 | 正式 |
| `10` | `I19_10_WORKFLOW_RUN_LIST` | 流程中心列表 | 正式，生产调整筛选密度 |
| `11_workflowrun` | `I19_11_WORKFLOW_RUN_DETAIL_PAGE` | 独立详情页 | 正式，替代旧 Drawer 形态 |
| `12` | `I19_12_RETRY_CONFIRM_DIALOG` | 重试弹窗 | 正式 |
| `13_workflows` | `I19_13_LEGACY_WORKFLOWS_GUIDANCE` | 旧流程页引导 | 正式，生产修正页头上下文 |
| `14` | `I19_14_SHARED_PREFLIGHT_BLOCKED` | 共享 Preflight | 正式 |
| `15` | `I19_15_SHARED_RUNTIME_RECOVERY` | 共享异常恢复 | 正式 |

## 3. 未选候选

`04_n8n_2` 与 04 重复，且示例 Base URL 携带 `/webhook`，容易把 Connection 实例根地址与 Workflow 引用混淆；不进入 Manifest，不作为开发基准。

## 4. 使用规则

- HTML 只用于结构和交互分析；生产使用现有 React 组件、Token、Locale 和 AppShell。
- Frame 中英文壳层不是最终文案标准。
- 长抽屉截图未展示下方区域时，必须使用滚动核对，不得据截图判断内容缺失。
- 任何原型替换必须同步更新本文件、`ui-manifest.json`、`ui-scope.md` 和 `ui-master-manifest.json`。
