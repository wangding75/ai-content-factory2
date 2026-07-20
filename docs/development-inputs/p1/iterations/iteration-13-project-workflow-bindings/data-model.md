# Iteration 13 — 项目四环节工作流绑定 — 数据模型（冻结）

## 1. 核心模型

### `ProjectWorkflowBinding`

| 字段 | 类型 | 约束 |
|---|---|---|
| `id` | UUID | 主键 |
| `projectId` | UUID | 必填，关联项目 |
| `stage` | string | `chapter_planning` / `content_generation` / `review` / `rewrite`，CHECK 约束 |
| `workflowConfigurationId` | UUID | 必填，关联 Iteration 12 的全局工作流配置 |
| `version` | integer | 必填，乐观锁，首次创建为 1，换绑后加 1 |
| `createdAt` | TIMESTAMPTZ | 必填 |
| `updatedAt` | TIMESTAMPTZ | 必填 |

## 2. 数据库约束

- 表名：`project_workflow_bindings`（snake_case，复数）；
- `id` UUID PRIMARY KEY；
- `UNIQUE(project_id, stage)`：同一项目的同一环节最多存在一个绑定；
- `stage` 使用 TEXT 字段加 CHECK 约束，不新增 PostgreSQL Enum；
- `version` INTEGER NOT NULL DEFAULT 1 CHECK (version >= 1)；
- `created_at`、`updated_at` 使用 TIMESTAMPTZ；
- 项目删除时绑定关系随项目级联删除（`ON DELETE CASCADE` 回 `projects`）；
- 全局工作流配置被绑定时不得物理删除（`ON DELETE RESTRICT` 或无 ON DELETE 子句，与 Iteration 12 全局配置删除策略一致）；
- `workflow_configuration_id` 增加查询索引；
- 解绑采用物理删除，历史由现有 Audit 保留，不增加 `deletedAt`；
- 项目必须存在且当前用户拥有访问权限；
- 全局工作流配置必须存在；
- 创建或更换时，工作流的 `applicableStages` 必须包含目标 `stage`；
- 已停用工作流不得作为新的绑定候选；
- 已存在绑定在全局工作流后续停用、未接入或连接异常时继续保留，不自动解绑；
- 更新和解除必须校验 `expectedVersion`；
- 创建、更换和解除在同一事务内完成业务写入、幂等记录和 Audit。

## 3. 读取模型

GET /projects/{projectId}/workflow-bindings 固定返回四个环节：

- `stage`：固定枚举值；
- `bound`：是否已绑定（独立字段，不合并为"可执行状态"）；
- `binding`：ProjectWorkflowBinding 对象或 null；
- `workflowConfigurationSummary`：完整的 WorkflowConfiguration 或 null；
- `binding.version`：乐观锁版本；
- `workflowConfigurationSummary.enabled`：全局工作流启用状态；
- `workflowConfigurationSummary.applicableStages`：适用环节；
- `workflowConfigurationSummary.integrationStatus`：集成状态；
- `workflowConfigurationSummary.connectionName` / `connectionType`：连接状态。

这些状态来自全局配置，不在 `ProjectWorkflowBinding` 中重复持久化。

## 4. 本迭代不新增的字段和表

以下不属于 Iteration 13 数据模型：

- `parameters`
- `validationStatus`
- `executionStatus`
- `workflowRunId`
- `lastExecutedAt`
- `integrationStatus`
- `connectionStatus`
- `enabled`
- `WorkflowRun`
- `ChapterPlanningParameters`
- `ContentGenerationParameters`
- `ContentReviewParameters`
- `ContentRewriteParameters`
- 项目级参数覆盖表

## 5. Audit

固定 action：

- `project_workflow_binding.create`
- `project_workflow_binding.replace`
- `project_workflow_binding.remove`

Audit 复用现有 `audit_logs` 表和 `audit.Repository`。

Create Audit 至少记录：projectId、stage、bindingId、workflowConfigurationId、newVersion。

Replace Audit 至少记录：projectId、stage、bindingId、oldWorkflowConfigurationId、newWorkflowConfigurationId、oldVersion、newVersion。

Remove Audit 至少记录：projectId、stage、bindingId、oldWorkflowConfigurationId、oldVersion。

Audit 不记录 Secret、Credential、完整 URL、默认参数正文、Idempotency-Key 明文或其他敏感配置。

禁止写 Audit 的情况：GET 查询、参数校验失败、权限失败、version 冲突、幂等重放、相同 workflowConfigurationId 的无变化请求、数据库事务失败。

Audit 与绑定修改必须处于同一事务。

## 6. 冻结状态

数据模型已冻结。后端和前端可依据冻结模型独立开发。后续开发不得自行改变字段、约束或语义。