# Iteration 13 — 项目四环节工作流绑定 — 数据模型

## 1. 核心模型

### `ProjectWorkflowBinding`

| 字段 | 类型 | 约束 |
|---|---|---|
| `id` | UUID | 主键 |
| `projectId` | UUID | 必填，关联项目 |
| `stage` | enum | `chapter_planning` / `content_generation` / `review` / `rewrite` |
| `workflowConfigurationId` | UUID | 必填，关联 Iteration 12 的全局工作流配置 |
| `version` | integer | 必填，用于乐观锁 |
| `createdAt` | timestamp | 必填 |
| `updatedAt` | timestamp | 必填 |

## 2. 数据库约束

- `UNIQUE(project_id, stage)`：同一项目的同一环节最多存在一个绑定；
- 项目必须存在且当前用户拥有访问权限；
- 全局工作流配置必须存在；
- 创建或更换时，工作流的 `applicableStages` 必须包含目标 `stage`；
- 已停用工作流不得作为新的绑定候选；
- 已存在绑定在全局工作流后续停用、未接入或连接异常时继续保留，不自动解绑；
- 更新和解除必须校验 `expectedVersion`；
- 创建、更换和解除在同一事务内完成业务写入、幂等记录和 Audit。

## 3. 读取模型

绑定列表可组合返回全局工作流的只读展示信息：

- 工作流名称；
- 工作流类型；
- 关联连接名称；
- 启用状态；
- 集成状态；
- 连接状态；
- 当前绑定版本。

这些状态来自全局配置，不在 `ProjectWorkflowBinding` 中重复持久化。

## 4. Audit

建议动作：

- `project_workflow_binding.create`
- `project_workflow_binding.replace`
- `project_workflow_binding.remove`

Audit 不记录 Secret、Credential、完整 URL、默认参数正文或其他敏感配置。

## 5. 本迭代不新增的模型

以下模型不属于 Iteration 13：

- `ChapterPlanningParameters`
- `ContentGenerationParameters`
- `ContentReviewParameters`
- `ContentRewriteParameters`
- `WorkflowRun`
- 运行快照、输出 Schema 和领域结果模型

项目级参数覆盖和工作流执行由后续迭代单独设计。
