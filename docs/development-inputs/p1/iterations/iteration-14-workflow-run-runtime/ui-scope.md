# Iteration 14 — WorkflowRun 异步运行基础设施 — UI Scope

## 原型关联

| Frame | 区域 | 用途 | 截图 | HTML |
|---|---|---|---|---|
| `E3_WORKFLOW_CENTER_V2` | 流程中心 | 真实运行列表、筛选、统计 | `ui/frames/E3_WORKFLOW_CENTER_V2/screen.png` | `ui/frames/E3_WORKFLOW_CENTER_V2/code.html` |
| `E3_WORKFLOW_RUN_DETAIL_DRAWER` | 流程中心 | 运行详情、失败诊断、绑定快照 | `ui/frames/E3_WORKFLOW_RUN_DETAIL_DRAWER/screen.png` | `ui/frames/E3_WORKFLOW_RUN_DETAIL_DRAWER/code.html` |
| `E3_RETRY_RUN_CONFIRM_DIALOG` | 流程中心 | 重试确认与配置版本选择 | `ui/frames/E3_RETRY_RUN_CONFIRM_DIALOG/screen.png` | `ui/frames/E3_RETRY_RUN_CONFIRM_DIALOG/code.html` |
| `STATE_TASK_RUNNING_BAR` | 共享组件 | 异步运行状态条 | `ui/frames/STATE_TASK_RUNNING_BAR/screen.png` | `ui/frames/STATE_TASK_RUNNING_BAR/code.html` |
| `STATE_TASK_FAILED_NOTICE` | 共享组件 | 安全错误通知与恢复动作 | `ui/frames/STATE_TASK_FAILED_NOTICE/screen.png` | `ui/frames/STATE_TASK_FAILED_NOTICE/code.html` |
| `STATE_NOT_CONFIGURED_EMPTY` | 共享组件 | 未配置、失效与空结果状态 | `ui/frames/STATE_NOT_CONFIGURED_EMPTY/screen.png` | `ui/frames/STATE_NOT_CONFIGURED_EMPTY/code.html` |

## UI 条件通过与开发修正规则

Iteration 11 的人工验收结论为：**有条件通过**。

已知问题：部分 Stitch 原型文案为英文，尤其可能出现在左侧一级菜单、状态标签、表头、按钮、辅助说明和技术占位文案中。原型中的英文不构成最终产品文案冻结。

开发必须满足：

1. 默认中文环境下，用户可见文案全部使用统一中文资源，不得直接复制 HTML 中的英文硬编码；
2. 左侧一级菜单统一为：首页、项目、素材、作品、工作流、设置；
3. 状态统一为：排队中、运行中、已成功、已失败、未验证、验证成功、已停用、配置异常；
4. `Run ID`、`Workflow ID`、Schema 版本、模型名、API 名称等技术标识可以保留英文；
5. 所有业务文案进入前端 i18n/locale 资源；组件不得内嵌不可替换英文；
6. 人工 UI 验收增加“中文文案与术语一致性”专项，发现英文用户文案即不通过。
