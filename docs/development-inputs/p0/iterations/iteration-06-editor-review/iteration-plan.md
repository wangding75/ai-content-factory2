# Iteration 06.1 — 正文编辑与模拟审核契约冻结

## 目标与边界

本轮冻结 D1_EDITOR（正文创建、编辑、保存草稿、确定性 Mock Generate）与 D2_REVIEW（确定性 Mock Review、审核列表和详情）的文档、状态机、OpenAPI 和 Novel Schema。`/content-items` 是正式领域命名。

不实现 Migration、后端或前端；不包含真实 LLM、流式生成、Prompt 编排、重写版本、工作流编排引擎、发布及 Iteration 07 能力。

## 冻结的产品规则

- 一个 `confirmed` ChapterPlan 最多对应一个 ContentItem；非 `confirmed` 创建返回 `409 chapter_plan_not_confirmed`。
- 首次创建原子地创建 ContentItem 与空白 `ContentVersion` v1（`source=manual_created`、`status=editable_draft`、`current_version_id=v1`），返回 201；再次创建返回既有对象与 v1，返回 200。
- v1 在本轮唯一，`version_no=1`。`version` 是乐观锁号，`expected_version` 是客户端提交条件，三者绝不混用。
- 草稿可重复保存；请求仅更新已提交字段，省略字段不变、`null` 清空可空字段、空字符串作为显式空值保存。成功递增 `version`；冻结或已审核版本返回 `409 content_version_locked`。
- Mock Generate 同步、确定性、可复现且不调用 AI；只更新当前 editable v1，不创建新版本。请求需 `expected_version`、`Idempotency-Key` 和 D1 冻结生成参数。首次有效执行创建 WorkflowRun；相同键与相同 payload 重试返回原结果，不增加版本或 WorkflowRun；不同 payload 返回 409。
- Mock Review 同步，明确传入当前 `content_version_id`、`expected_version` 与 `Idempotency-Key`。内部可暂态 `in_review`，成功响应时对象已为 `reviewed`。成功时冻结 v1、创建固定关联 v1 的 ReviewReport/Finding/Recommendation、创建 succeeded WorkflowRun 并写入 `reviewed_at`。失败时回到 `draft`，不保留部分审核数据，记录 failed WorkflowRun。已审核版本使用新键再审返回 `409 content_version_already_reviewed`。

## API

| 方法 | 路径 | 成功 |
|---|---|---|
| POST | `/api/v1/chapter-plans/{chapterPlanId}/content` | 201 / 已存在 200 |
| GET | `/api/v1/content-items/{contentItemId}` | 200 |
| PUT | `/api/v1/content-items/{contentItemId}/draft` | 200 |
| POST | `/api/v1/content-items/{contentItemId}/mock-generate` | 200 |
| POST | `/api/v1/content-items/{contentItemId}/reviews/mock` | 200 |
| GET | `/api/v1/content-items/{contentItemId}/reviews` | 200 |
| GET | `/api/v1/reviews/{reviewId}` | 200 |

所有响应使用 `{ data, request_id }` envelope；错误不暴露 SQL、堆栈或内部实现。审核列表按 `created_at DESC, id DESC` 稳定排序。

## UI 对齐

D1 从 confirmed ChapterPlan 进入，允许创建/打开、编辑、保存、Mock Generate、取消和提交审核。D2 成功页显示“已审核”；冻结 UI 中“创建重写版本”位置可保留，但 Iteration 06 必须禁用并标明 Iteration 07 开放，不能调用重写 API 或创建 v2。所有异步页面均须隔离项目切换前的旧响应。

## 完成条件

OpenAPI、Novel Schema、数据模型、状态机、业务规则、任务和验收文档一致；验证脚本通过；不产生实现、Migration 或 Iteration 07 表面。
