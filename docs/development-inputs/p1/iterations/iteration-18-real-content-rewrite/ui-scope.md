# Iteration 18 — 真实正文重写 UI Scope

**状态：`rebuild_candidate_2026_07_29`。**

## 1. Frame 清单

| 顺序 | Frame | 页面/状态 | 生产路由 | 截图 |
|---:|---|---|---|---|
| 1 | `I18_D2_REVIEW_REWRITE_ENTRY` | 审核结果：选择问题并创建重写 | `/projects/{projectId}/works/{workId}/review?reportId={reviewReportId}` | `ui/frames/I18_D2_REVIEW_REWRITE_ENTRY/screen.png` |
| 2 | `I18_D4_CREATE_REWRITE` | 创建正文重写 | `/projects/{projectId}/works/{workId}/rewrite?reportId={reviewReportId}` | `ui/frames/I18_D4_CREATE_REWRITE/screen.png` |
| 3 | `I18_D4_REWRITE_CONFIG_DRAWER` | 查看项目重写配置 | `/projects/{projectId}/works/{workId}/rewrite?reportId={reviewReportId}` | `ui/frames/I18_D4_REWRITE_CONFIG_DRAWER/screen.png` |
| 4 | `I18_D4_REWRITE_RUNNING` | 正文重写运行中 | `/projects/{projectId}/works/{workId}/rewrite?workflowRunId={workflowRunId}` | `ui/frames/I18_D4_REWRITE_RUNNING/screen.png` |
| 5 | `I18_D5_REWRITE_RESULT` | 正文重写成功候选 | `/projects/{projectId}/works/{workId}/rewrite?workflowRunId={workflowRunId}` | `ui/frames/I18_D5_REWRITE_RESULT/screen.png` |
| 6 | `I18_D5_SET_CURRENT_CONFIRM` | 设为当前版本确认 | `/projects/{projectId}/works/{workId}/rewrite?workflowRunId={workflowRunId}` | `ui/frames/I18_D5_SET_CURRENT_CONFIRM/screen.png` |
| 7 | `I18_D5_RESULT_CONSUMPTION_FAILED` | 重写结果提交失败 | `/projects/{projectId}/works/{workId}/rewrite?workflowRunId={workflowRunId}` | `ui/frames/I18_D5_RESULT_CONSUMPTION_FAILED/screen.png` |
| 8 | `I18_D4_REWRITE_FAILED` | 重写任务执行失败 | `/projects/{projectId}/works/{workId}/rewrite?workflowRunId={workflowRunId}` | `ui/frames/I18_D4_REWRITE_FAILED/screen.png` |
| 9 | `I18_D4_REWRITE_AVAILABILITY` | 未配置、配置失效与空状态 | `/projects/{projectId}/works/{workId}/rewrite?reportId={reviewReportId}` | `ui/frames/I18_D4_REWRITE_AVAILABILITY/screen.png` |

## 2. 状态映射

| Summary State | Frame |
|---|---|
| idle | 01 / 02 |
| not_configured | 03 / 09 |
| queued | 04 排队文案变体 |
| running | 04 |
| candidate_ready | 05 / 06 |
| runtime_failed | 08 Runtime 失败变体 |
| output_validation_failed | 08 输出校验变体 |
| result_consumption_failed | 07 |

## 3. UI 强制规则

- 01 扩展 Iteration 17 审核结果页；只有 `open` Issue 可选择。
- 02 的工作流/模型来自项目配置，只读；用户只选择 Issue、策略和补充要求。
- 03/06 是抽屉/弹窗状态，不创建伪路由。
- 04/07/08/09 是同一 Rewrite Route 的持久化状态，不为每个状态创建页面。
- 05 候选不是 current；打开编辑器只能进入只读候选预览，直到显式设为当前。
- 07 不得展示候选、版本比较、编辑器入口或 set-current。
- 08 不得把 output_validation_failed 显示为 result_consumption_failed。
- 09 的卡片互斥显示，不得在生产中同时展示。
- 所有示例数据不得硬编码；业务文案进入现有 locale。
