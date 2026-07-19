# Iteration 13 — 项目四环节工作流绑定

## 1. 目标

为章节规划、正文生成、内容审核和正文重写分别保存项目级工作流绑定与默认参数。

本迭代只管理绑定关系，不验证第三方连接，不启用工作流，不创建 WorkflowRun。

## 2. 前置条件

- Iteration 12 已完成全局工作流配置 CRUD；
- 工作流配置可以处于 `not_connected`；
- 项目绑定不要求工作流已经完成真实第三方验证；
- 未接入状态必须在 UI 中明确展示。

## 3. 用户闭环

项目设置 → 查看四环节绑定总览 → 打开某环节抽屉 → 选择已保存的全局工作流 → 设置默认参数 → 保存 → 返回总览。

规则：

- 四个环节独立绑定；
- 一个环节最多一个当前绑定；
- 可选择 Iteration 12 已保存且未被归档的工作流；
- 保存绑定不触发连接验证；
- 保存绑定不触发工作流执行；
- 若所选工作流处于 `not_connected`，显示“已绑定，尚未接入执行能力”；
- 后续 Iteration 14 完成 n8n 验证和启用后，绑定可被运行时使用。

## 4. 数据模型

- `ProjectWorkflowBinding`
- `ChapterPlanningParameters`
- `ContentGenerationParameters`
- `ContentReviewParameters`
- `ContentRewriteParameters`

本迭代不创建 `WorkflowRun`、运行事件或绑定快照。详细约束见 `data-model.md`。

## 5. API

- `GET /api/v1/projects/{projectId}/workflow-bindings`
- `GET /api/v1/projects/{projectId}/workflow-bindings/{stage}`
- `PUT /api/v1/projects/{projectId}/workflow-bindings/{stage}`

本迭代不包含：

- validate；
- enable；
- disable；
- run；
- retry。

冻结范围见 `api-scope.yaml`。

## 6. UI 与原型

- `S02_PROJECT_WORKFLOW_SETTINGS`
- `S02_BIND_CHAPTER_WORKFLOW_DRAWER`
- `S02_BIND_CONTENT_WORKFLOW_DRAWER`
- `S02_BIND_REVIEW_WORKFLOW_DRAWER`
- `S02_BIND_REWRITE_WORKFLOW_DRAWER`
- `STATE_NOT_CONFIGURED_EMPTY`

开发必须：

- 使用中文 locale；
- 展示关联工作流、连接类型和接入状态；
- 对 `not_connected` 显示警告，但允许保存绑定；
- 不展示运行中、重试、日志或执行结果。

## 7. 实施顺序

1. 冻结绑定 CRUD 契约；
2. 实现数据模型、迁移、Repository 和 Service；
3. 实现四环节绑定 API；
4. 前端基于 Mock 完成全部页面状态；
5. 人工 UI 验收；
6. 接入真实绑定 CRUD API；
7. 局部测试 → 分组测试 → 一次总门禁；
8. Code Review 和 Git 验收。

## 8. 不在范围

- 不验证 LLM、n8n 或分发平台；
- 不实现任何 Adapter；
- 不发起第三方网络请求；
- 不启用或停用全局连接/工作流；
- 不创建 WorkflowRun；
- 不保存绑定快照；
- 不执行章节规划、正文生成、审核或重写；
- 不实现日志、失败诊断或重试；
- 不允许业务页面临时切换工作流。

## 9. 完成定义

- [ ] 四环节绑定 CRUD 完成；
- [ ] 默认参数按环节 Schema 保存；
- [ ] 绑定页面显示工作流接入状态；
- [ ] `not_connected` 工作流可以绑定但明确不可执行；
- [ ] 没有第三方网络请求；
- [ ] 没有 WorkflowRun；
- [ ] OpenAPI、数据模型和 UI 一致；
- [ ] 局部、分组和最终验收通过。
