# P0 数据模型目录

## 通用字段

所有持久化实体至少包含：

```text
id
created_at
updated_at
created_by（P0 可使用固定系统用户）
version / optimistic_lock（写冲突对象按需）
```

所有状态变化写 `audit_logs`。

## 核心实体

### Project

```text
id, name, type, status, description, current_stage, created_at, updated_at
```

### ProjectPlanning

```text
project_id, premise, audience, style, goals_json, constraints_json, updated_at
```

### Material

```text
id, type, name, summary, content_json, tags_json, created_at, updated_at
```

Material 是唯一全局素材本体。

### ProjectMaterialUsage

```text
id, project_id, material_id, usage_type, role_name, notes, status, created_at, updated_at
UNIQUE(project_id, material_id)
```

项目用途不写入 Material 本体。

### PlotLine

```text
id, project_id, parent_id, type, name, summary, status, sort_order
```

统一承载主线、子线和更深层级。

### Foreshadowing

```text
id, project_id, title, description, planted_plot_line_id,
payoff_plot_line_id, planned_plant_chapter, planned_payoff_chapter, status
```

### ChapterPlan

```text
id, project_id, chapter_no, title, summary, status, source,
storyline_refs_json, material_refs_json, foreshadowing_refs_json, confirmed_at
```

### ContentItem

```text
id, project_id, chapter_plan_id, pack_key, title, status,
current_version_id, review_status, publish_status
```

### ContentVersion

```text
id, content_item_id, version_no, parent_version_id, source,
body, word_count, is_current, created_at
```

### ReviewReport

```text
id, content_item_id, version_id, provider_key, status,
score_json, summary, created_at
```

### ReviewFinding / ReviewRecommendation

```text
review_id, type/category, severity/priority, title, description, location_json
```

### WorkflowRun

```text
id, provider_key, workflow_key, subject_type, subject_id, status,
input_json, output_json, error_json, started_at, finished_at
```

Iteration 07.1A `content_mock_rewrite` additionally fixes the source ContentItem, frozen v1, completed ReviewReport, and nullable target v2 in the run relation. It retains `idempotency_key` and request fingerprint under the existing operation scope. A succeeded run references its v2; a failed run retains safe error information but no partial target version.

`ContentVersion.source` additionally permits `mock_rewrite` only for v2. v2 is `editable_draft`; review and publication remain absent relations rather than creating a second ContentVersion model.

Iteration 07.1B query projections are `ContentVersionListItem` and `ContentVersionDetail`. They use `ContentItem.current_version_id` as the sole current-version predicate, include nullable source v1/ReviewReport/WorkflowRun summaries, and keep the requested ContentVersion immutable. Lists are ordered `version_no DESC, id DESC`.

Iteration 07.1C `ProjectWorkReadModel` is a non-persistent read projection. Its `work_id` is exactly `ContentItem.id`; it aggregates only existing Project, ChapterPlan, ContentItem, ContentVersion, ReviewReport, and WorkflowRun data. List order is `chapter_plan.chapter_no ASC, content_item.id ASC`.

### ProjectWorkReadModel / GlobalWorkReadModel

由 ContentItem、ContentVersion、ReviewReport 聚合，不建议 P0 重复落独立业务真值表。

### CapabilityDescriptor / IntegrationDescriptor

P0 可由配置与静态种子提供；不得伪造连接成功状态。
