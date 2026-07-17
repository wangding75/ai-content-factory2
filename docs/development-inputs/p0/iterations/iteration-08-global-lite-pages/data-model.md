# 数据模型范围

- `GlobalMaterialReadModel`
- `GlobalWorkReadModel`
- `BuiltinWorkflowDefinition`
- `CapabilityDescriptor`
- `IntegrationDescriptor`

`GlobalWorkReadModel` reuses the Iteration 07 `ProjectWorkReadModel` fields with an added read-only project summary; it does not create a Work model. `GlobalWorkflowRunSummary` is a projection of existing WorkflowRun/subject/project relations and contains only safe error text. `BuiltinWorkflowDefinition`, `CapabilityDescriptor`, and `IntegrationDescriptor` are immutable P0 descriptors; no settings entity or credential is introduced.

字段与关系以 `../../baselines/p0-data-model-catalog.md` 为全局基线。
