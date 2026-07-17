# P0 数据模型目录

所有持久化实体至少含 `id, created_at, updated_at`；有并发写入的实体还含 `version`。所有状态变化写入 audit_logs。

## 已有核心实体

`Project(id, name, type, status, description, current_stage)`；`ProjectPlanning(project_id, premise, audience, style, goals_json, constraints_json)`；`Material(id, type, name, summary, content_json, tags_json)`；`ProjectMaterialUsage(id, project_id, material_id, usage_type, role_name, notes, status, UNIQUE(project_id, material_id))`；`PlotLine(id, project_id, parent_id, type, name, summary, status, sort_order)`；`Foreshadowing(id, project_id, title, description, planted_plot_line_id, payoff_plot_line_id, planned_plant_chapter, planned_payoff_chapter, status)`；`ChapterPlan(id, project_id, chapter_no, title, summary, status, source, storyline_refs_json, material_refs_json, foreshadowing_refs_json, confirmed_at, version)`。

## Iteration 06 内容与审核实体

### ContentItem

`id, project_id, chapter_plan_id UNIQUE, pack_key, title, status, current_version_id, reviewed_at, created_at, updated_at`。一个 confirmed ChapterPlan 至多一个 ContentItem。

### ContentVersion

`id, content_item_id, version_no, version, status, source, title, content, summary, word_count, frozen_at, created_at, updated_at`。Iteration 06 仅 v1：`version_no=1`，初始 manual_created/editable_draft。

### ReviewReport / ReviewFinding / ReviewRecommendation

`ReviewReport(id, content_item_id, content_version_id, provider_key, status, conclusion, score_json, summary, created_at)` 固定指向一个已冻结版本；`ReviewFinding(id, review_id, category, severity, title, description, location_json)`；`ReviewRecommendation(id, review_id, priority, title, description, created_at)`。

### WorkflowRun

`id, provider_key, workflow_key, subject_type, subject_id, status, idempotency_key, input_json, output_json, error_code, started_at, finished_at`。相同操作作用域、Idempotency-Key 与 payload 仅产生一个业务结果。

Project/global work read model 从以上实体聚合，不复制业务真值。

## Iteration 08 global Lite projections

`GlobalWorkReadModel` is the cross-project list projection of Iteration 07 `ProjectWorkReadModel` plus a project summary. `GlobalWorkflowRunSummary` projects an existing WorkflowRun, its subject, its owning project when resolvable, lifecycle timestamps, and a safe nullable error summary. `BuiltinWorkflowDefinition`, `CapabilityDescriptor`, and `IntegrationDescriptor` are immutable display descriptors only; no configuration, credential, provider, Work, or settings persistence entity is added.
