# 代码或原型扫描覆盖异常记录表 (Coverage Exceptions)

本表记录了本地原型中存在，但本次 88 项验收清单中未纳入的 Frame 清单及其原因。

| 迭代版本 | 原型 Frame 标识 | 状态 | 排除原因与依据 |
|---|---|---|---|
| iteration-12-global-execution-connections | `CONNECTION_DRAWER_V2` | 排除 | 未包含在任务书指定的88项中，符合冻结清单范围控制原则。 |
| iteration-12-global-execution-connections | `GLOBAL_SETTINGS_CONNECTION_LIST_V2` | 排除 | 未包含在任务书指定的88项中，符合冻结清单范围控制原则。 |
| iteration-12-global-execution-connections | `GLOBAL_SETTINGS_LLM_LIST_V2` | 排除 | 未包含在任务书指定的88项中，符合冻结清单范围控制原则。 |
| iteration-12-global-execution-connections | `GLOBAL_SETTINGS_WORKFLOW_LIST_V2` | 排除 | 未包含在任务书指定的88项中，符合冻结清单范围控制原则。 |
| iteration-12-global-execution-connections | `LLM_CONFIG_DRAWER_V2` | 排除 | 未包含在任务书指定的88项中，符合冻结清单范围控制原则。 |
| iteration-12-global-execution-connections | `WORKFLOW_DRAWER_V2` | 排除 | 未包含在任务书指定的88项中，符合冻结清单范围控制原则。 |
| iteration-13-project-workflow-bindings | `P13_01_PROJECT_SETTINGS_ENTRY` | 排除 | 按任务书第七节明确排除：implementationRequired=false，仅壳层参考。 |
| iteration-13-project-workflow-bindings | `P13_03_SELECT_WORKFLOW_DRAWER` | 排除 | 未包含在任务书指定的88项中，符合冻结清单范围控制原则。 |
| iteration-13-project-workflow-bindings | `P13_05_REPLACE_WORKFLOW_DRAWER` | 排除 | 未包含在任务书指定的88项中，符合冻结清单范围控制原则。 |
| iteration-13-project-workflow-bindings | `P13_07_WORKFLOW_BINDINGS_COMPLETE` | 排除 | 未包含在任务书指定的88项中，符合冻结清单范围控制原则。 |
| iteration-13-project-workflow-bindings | `P13_09_WORKFLOW_BINDING_EXCEPTIONS` | 排除 | 未包含在任务书指定的88项中，符合冻结清单范围控制原则。 |
| iteration-14-workflow-run-runtime | `P14_01_WORKFLOW_RUN_LIST` | 排除 | 未包含在任务书指定的88项中，符合冻结清单范围控制原则。 |
| iteration-14-workflow-run-runtime | `P14_05_WORKFLOW_RUN_DETAIL_SUCCEEDED` | 排除 | 未包含在任务书指定的88项中，符合冻结清单范围控制原则。 |
| iteration-14-workflow-run-runtime | `P14_08_RUN_CONFIRM_DIALOG` | 排除 | 按任务书第七节明确排除：已由 I15～I18 发起 UI 和 I19 共享预检覆盖。 |
| iteration-14-workflow-run-runtime | `P14_09_WORKFLOW_NOT_BOUND_DIALOG` | 排除 | 按任务书第七节明确排除：已由阶段级未配置状态和 I19 共享预检覆盖。 |