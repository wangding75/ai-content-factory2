# 数据架构

## 1. 核心关系

```text
Project 1—1 ProjectPlanning
Project 1—N ProjectMaterialUsage N—1 Material
Project 1—N PlotLine
Project 1—N Foreshadowing
Project 1—N ChapterPlan
ChapterPlan 1—0..1 ContentItem
ContentItem 1—N ContentVersion
ContentVersion 1—N ReviewReport
WorkflowRun N—1 Subject
```

## 2. 数据原则

- PostgreSQL 是业务真值库。
- Redis 只承载缓存、队列和短期运行状态，不保存唯一真值。
- JSONB 用于内容类型扩展字段，不替代关键关系和状态列。
- 外键、唯一约束和检查约束必须承载可在数据库保证的规则。

## 3. 关键约束

- `UNIQUE(project_id, material_id)`。
- 一个 ChapterPlan 最多对应一个主 ContentItem。
- `ContentVersion(content_item_id, version_no)` 唯一。
- 任何 ReviewReport 必须关联同一 ContentItem 的合法版本。
- parent_version_id 必须属于同一 ContentItem。

## 4. 事务边界

必须原子执行：

- 创建 Project + AuditLog。
- 创建 Material + Usage + AuditLog。
- 确认一组 ChapterPlan。
- 创建 ContentItem + v1。
- 创建 ReviewReport + Findings + Recommendations + 状态更新。
- 创建重写版本 + WorkflowRun 完成状态。

## 5. 迁移原则

- 每个变更包含 up/down SQL。
- Migration 不依赖手工数据库状态。
- 迁移在空库和前一版本库均可验证。
- 禁止直接修改已发布迁移，必须新增迁移。
