# Iteration 17 — 真实内容审核 UI Scope

**状态：`frozen_cf_17_01`。**

## 1. Frame 映射

| Order | Frame | 页面/状态 | 正式路由 | Screenshot | HTML |
|---:|---|---|---|---|---|
| 1 | `I17_D1_EDITOR_REVIEW_ENTRY` | 正文编辑器与提交审核入口 | `/projects/{projectId}/works/{workId}` | `ui/frames/I17_D1_EDITOR_REVIEW_ENTRY/screen.png` | `ui/frames/I17_D1_EDITOR_REVIEW_ENTRY/code.html` |
| 2 | `D2_SUBMIT_REVIEW_DRAWER` | 发起内容审核抽屉 | 编辑器路由上的 Drawer | `ui/frames/D2_SUBMIT_REVIEW_DRAWER/screen.png` | `ui/frames/D2_SUBMIT_REVIEW_DRAWER/code.html` |
| 3 | `STATE_TASK_RUNNING_BAR` | queued/running 审核任务 | `/projects/{projectId}/works/{workId}/review` | `ui/frames/STATE_TASK_RUNNING_BAR/screen.png` | `ui/frames/STATE_TASK_RUNNING_BAR/code.html` |
| 4 | `D2_REVIEW_V2` | 审核结果总览 | `/projects/{projectId}/works/{workId}/review?reportId={reviewId}` | `ui/frames/D2_REVIEW_V2/screen.png` | `ui/frames/D2_REVIEW_V2/code.html` |
| 5 | `I17_D2_REVIEW_ISSUE_DETAIL` | 问题详情与全文定位 | `/projects/{projectId}/works/{workId}/review?reportId={reviewId}&issueId={issueId}&view=source` | `ui/frames/I17_D2_REVIEW_ISSUE_DETAIL/screen.png` | `ui/frames/I17_D2_REVIEW_ISSUE_DETAIL/code.html` |
| 6 | `STATE_TASK_FAILED_NOTICE` | 三类失败与恢复 | `/projects/{projectId}/works/{workId}/review` | `ui/frames/STATE_TASK_FAILED_NOTICE/screen.png` | `ui/frames/STATE_TASK_FAILED_NOTICE/code.html` |
| 7 | `STATE_NOT_CONFIGURED_EMPTY` | 未配置/配置失效 | `/projects/{projectId}/works/{workId}/review` | `ui/frames/STATE_NOT_CONFIGURED_EMPTY/screen.png` | `ui/frames/STATE_NOT_CONFIGURED_EMPTY/code.html` |
| 8 | `I17_D2_REVIEW_HISTORY` | 审核历史 | `/projects/{projectId}/works/{workId}/review/history` | `ui/frames/I17_D2_REVIEW_HISTORY/screen.png` | `ui/frames/I17_D2_REVIEW_HISTORY/code.html` |

## 2. 开发约束

- 01/02 复用 Iteration 16 编辑器，不新建截图专用编辑器页面。
- 03–07 是同一 Review Route 的持久化状态，不创建 queued/failed/not-configured 伪路由。
- 04/05 使用同一 Report/Issue 数据；全文定位必须读取来源版本快照。
- 06 依据 Summary 失败类型显示不同 Retry；技术详情必须脱敏。
- 08 基于 WorkflowRun 历史，失败/运行中记录的 Report 可为空。
- 所有业务文案进入现有 locale；示例数据不得写死进生产代码。
- “创建重写任务”在 Iteration 17 必须禁用；Iteration 18 再接入真实命令。

## 3. 状态复用

| Summary State | Frame |
|---|---|
| `idle` | 01 / 02 |
| `not_configured` | 07 |
| `queued` | 03 的排队文案变体 |
| `running` | 03 |
| `review_ready` | 04 / 05 / 08 |
| `runtime_failed` | 06 Runtime Retry 变体 |
| `output_validation_failed` | 06 Runtime Retry 变体 |
| `result_consumption_failed` | 06 仅消费 Retry 变体 |
