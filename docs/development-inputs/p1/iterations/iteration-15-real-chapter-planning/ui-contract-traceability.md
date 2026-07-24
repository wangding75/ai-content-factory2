# CF-15-01C — UI / API 契约追踪

**状态：`frozen_cf_15_01c`。** 21 个 Frame 归并为工作区、候选页面及故事线页面；不是 21 条路由。所有响应使用 `data`/`request_id`，错误使用既有 `ErrorEnvelope`。

| Frame | 组件 / 路由 | 动作与 API | 状态、错误、后续任务 |
|---|---|---|---|
| P15_C1_RUNNING_MAINLINE_EXPANSION | 章节工作区 `/projects/{projectId}/chapters` | `getProjectChapterPlanningSummary`、`listProjectChapterPlans`、Runtime `listWorkflowRuns` | loading/running；02B、03A |
| P15_C1_RUNNING_PARTIAL_RANGE | 同上 canonical | 同上 | success/running；03A |
| P15_C1_RUNNING_FULL_PLAN | 同上 | 同上 | success/running；03A |
| P15_C1_RUNNING_PARTIAL_ETA | 同上 | 同上 | running；03A |
| P15_C1_RUNNING_PARTIAL_VALIDATING | 同上 | Runtime detail/events | validating；03A |
| P15_C1_RUNNING_DATA_VARIANT | 同上状态变体 | summary、chapter plans | success；03A |
| P15_C1_FAILED_ATOMIC | 同上状态变体 | Runtime detail/events | failed/output_validation_failed/result_consumption_failed；03A |
| P15_C1_NOT_CONFIGURED | 同上状态变体 | preflight | empty/workflow_not_configured；03A |
| P15_C2_GENERATION_SETTINGS | 生成抽屉 | `preflightChapterPlanRun` | input/loading；03A |
| P15_C2_PREFLIGHT_PROGRESS | 预检弹窗 | `preflightChapterPlanRun` | loading；03A |
| P15_C3_PREFLIGHT_PASS | 预检报告弹窗 | preflight → `createChapterPlanRun` | success/warnings；preflight_token_*；03A |
| P15_C3_PREFLIGHT_BLOCKED | 预检报告弹窗 | preflight | blocked/preflight_blocked/active_run_conflict；03A |
| P15_C3_RUN_CREATED | 创建成功弹窗 | `createChapterPlanRun` | queued/success；03A |
| P15_C4_CANDIDATE_BATCH_LIST | 页面 `/projects/{projectId}/chapter-plan-candidate-batches` | `listChapterPlanCandidateBatches` | loading/empty/error；03B |
| P15_C5_CANDIDATE_BATCH_DETAIL | 页面 `/chapter-plan-candidate-batches/{batchId}` | get Batch、list Candidates | loading/empty/error；03B |
| P15_C6_CANDIDATE_EDIT_DRAWER | 编辑抽屉 | get/PATCH Candidate | success/version_conflict/invalid_candidate_state；03C |
| P15_C7_CANDIDATE_COMPARE_DIALOG | 对比弹窗 | compare/recompare Candidate | success/stale_candidate/version_conflict；03C |
| P15_C8_BATCH_ADOPT_DIALOG | 采用弹窗 | bulk adoptions | itemized adopted/no_change/stale/conflict/failed；03C |
| P15_C9_BATCH_ABANDON_DIALOG | 放弃弹窗 | abandon Batch | success/batch_already_finalized/version_conflict；03C |
| P15_C10_STALE_CONFLICT_DIALOG | stale 弹窗 | compare/recompare/discard Candidate | stale_candidate；禁止强制覆盖；03C |
| P15_S1_STORYLINE_RELATION_READONLY | 故事线 `/projects/{projectId}/storylines` | listStorylines + chapter-planning summary 聚合 | loading/empty/error/read-only；03D |

Candidate Batch 列表与详情是正式页面；编辑是抽屉；比较、采用、放弃和 stale 是弹窗。章节确认继续使用 `confirmProjectChapterPlans`，其每项 `expected_version` 指向 ChapterPlan；不新增 Confirm API。Adopt 同时提交 Candidate 与当前 ChapterPlan 的 expected version；Batch 操作提交 Batch version；所有写命令要求 Idempotency-Key，除 Candidate 编辑与 Recompare 外均持久化可重放。Runtime 的列表、详情、Event、Cancel、Retry 均复用 Iteration 14 `/api/v1/workflow-runs` 接口。
