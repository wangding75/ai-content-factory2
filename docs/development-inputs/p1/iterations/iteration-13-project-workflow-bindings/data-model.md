# Iteration 13 — 项目四环节工作流绑定 — 数据模型

## 1. ProjectWorkflowBinding

建议字段：

| 字段 | 类型 | 约束 |
|---|---|---|
| `id` | uuid | PK |
| `project_id` | uuid | FK, not null |
| `stage` | varchar(40) | `chapter_planning / content_generation / review / rewrite` |
| `workflow_configuration_id` | uuid | FK → `workflow_configurations.id`, not null |
| `default_parameters` | jsonb | 按环节本地 Schema 校验 |
| `version` | integer | 乐观锁 |
| `created_at` | timestamptz | not null |
| `updated_at` | timestamptz | not null |

唯一约束：

- `(project_id, stage)` 唯一。

## 2. 环节参数

- `ChapterPlanningParameters`
- `ContentGenerationParameters`
- `ContentReviewParameters`
- `ContentRewriteParameters`

参数只做本地 Schema 校验，不访问工作流平台。

## 3. 派生字段

查询 DTO 可以返回：

- 工作流名称；
- 关联连接名称；
- 连接类型；
- `integrationStatus`；
- `enabled`；
- `runtimeReady`。

Iteration 13 中：

- `runtimeReady=false`，除非后续迭代已经完成并回填真实接入状态；
- 该字段只用于展示，不在保存绑定时触发验证。

## 4. 约束

- 工作流配置必须存在；
- 工作流配置必须关联连接；
- 连接或工作流处于 `not_connected` 时仍允许保存绑定；
- UI 必须显示“尚未接入执行能力”；
- 本迭代不复制密钥；
- 本迭代不创建绑定快照；
- 本迭代不创建 WorkflowRun；
- 删除或归档被绑定工作流的行为不在本迭代实现。

## 5. 审计

记录：

- `binding_create`；
- `binding_replace`；
- `default_parameters_update`。

不得记录：

- 凭证；
- 第三方请求；
- 运行结果；
- 虚构验证状态。
