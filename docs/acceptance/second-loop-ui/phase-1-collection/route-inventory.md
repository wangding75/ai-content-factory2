# Next.js App Router 路由分析清单 (Route Inventory)

本清单罗列了项目 App Router 下的相关路由路径、路由源码位置以及是否纳入本次第二闭环 UI 验收的依据说明。

| 路由路径 | 路由源码文件 | 状态 | 纳入/排除依据 |
|---|---|---|---|
| `/settings` | `apps/web/src/app/settings/page.tsx` | **已纳入** | UI-001 ~ UI-012 全局设置页面 (LLM, connections, workflows, distribution) |
| `/projects/{projectId}/settings` | `apps/web/src/app/projects/[projectId]/settings/page.tsx` | **已纳入** | UI-013 ~ UI-021 项目级别工作流绑定与冲突状态 |
| `/workflow-runs` | `apps/web/src/app/workflow-runs/page.tsx` | **已纳入** | UI-022 ~ UI-025 全量流程运行列表及项目筛选 |
| `/workflow-runs/{runId}` | `apps/web/src/app/workflow-runs/[runId]/page.tsx` | **已纳入** | UI-026 ~ UI-029 流程运行详情及诊断、重试状态 |
| `/workflows` | `apps/web/src/app/workflows/page.tsx` | **已纳入** | UI-030 旧工作流模块只读引导页面 |
| `/projects/{projectId}` | `apps/web/src/app/projects/[projectId]/page.tsx` | **已纳入** | UI-031 项目概览大盘与运行摘要 |
| `/projects/{projectId}/chapter-plans` | `apps/web/src/app/projects/[projectId]/chapter-plans/page.tsx` | **已纳入** | UI-032 ~ UI-044 章节规划主流程、预检报告与状态 |
| `/projects/{projectId}/chapter-plan-candidate-batches` | `apps/web/src/app/projects/[projectId]/chapter-plan-candidate-batches/page.tsx` | **已纳入** | UI-045 规划候选批次列表 |
| `/projects/{projectId}/chapter-plan-candidate-batches/{batchId}` | `apps/web/src/app/projects/[projectId]/chapter-plan-candidate-batches/[batchId]/page.tsx` | **已纳入** | UI-046 ~ UI-051 批次详情、对比、采用/放弃及失效冲突 |
| `/projects/{projectId}/storylines` | `apps/web/src/app/projects/[projectId]/storylines/page.tsx` | **已纳入** | UI-052 故事线与章节关联只读展示 |
| `/projects/{projectId}/works/{workId}` | `apps/web/src/app/projects/[projectId]/works/[workId]/page.tsx` | **已纳入** | UI-053 ~ UI-065 正文编辑器 Tab 切换、生成参数及状态 |
| `/projects/{projectId}/works/{workId}/review` | `apps/web/src/app/projects/[projectId]/works/[workId]/review/page.tsx` | **已纳入** | UI-066 ~ UI-070 审核工作区、定位及状态 |
| `/projects/{projectId}/works/{workId}/review/history` | `apps/web/src/app/projects/[projectId]/works/[workId]/review/history/page.tsx` | **已纳入** | UI-071 内容质量历史报告记录 |
| `/projects/{projectId}/works/{workId}/rewrite` | `apps/web/src/app/projects/[projectId]/works/[workId]/rewrite/page.tsx` | **已纳入** | UI-073 ~ UI-080 重写工作区参数、状态及应用切换 |
| `/projects/{projectId}/settings` | - | **排除** | `P13_01_PROJECT_SETTINGS_ENTRY` 仅作为壳层结构参考，无直接验收点，予以排除 |
| `/workflow-runs` (未授权修改) | - | **排除** | `P14_08_RUN_CONFIRM_DIALOG` / `P14_09_WORKFLOW_NOT_BOUND_DIALOG` 已被各环节发起 UI 与共享预检阻断覆盖，予以排除 |