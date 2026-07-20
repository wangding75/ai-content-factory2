# Iteration 13 — 项目四环节工作流绑定

## 1. 状态

- **当前阶段：范围与 UI 契约已冻结（CF-13-01）。**
- **OpenAPI、数据模型、错误结构、幂等、乐观锁、Audit 和并发契约将在 CF-13-02 冻结。**
- **当前不得开始后端或前端开发。**
- UI 基线：采用首次生成的 `P13-01` 至 `P13-10` Stitch 版本。
- UI 验收：**有条件通过**；开发时必须执行 `ui-scope.md` 中的修正规则。
- 本迭代只保存项目与全局工作流的绑定关系，不执行任何工作流。

## 2. 目标

为一个项目的以下四个业务环节分别选择、保存、更换和解除全局工作流绑定：

| 环节 | 枚举 |
|---|---|
| 章节规划 | `chapter_planning` |
| 内容生成 | `content_generation` |
| 审核 | `review` |
| 改写 | `rewrite` |

完成后，用户可以刷新页面并从真实 API 与 PostgreSQL 恢复四个环节的绑定状态。

## 3. 用户闭环

```text
进入项目
→ 打开“设置”
→ 进入“工作流绑定”
→ 查看四个固定环节
→ 选择适用于当前环节的全局工作流
→ 保存绑定
→ 更换或解除绑定
→ 刷新后恢复服务端状态
```

辅助入口：

- 项目策划完成且存在未配置环节时，可显示轻量“配置项目工作流”入口，跳转到项目设置的工作流绑定页；
- 项目概览的“下一步建议”保持现有 UI，仅动态切换建议内容：策划已完成但工作流未配置完整时，建议“配置项目工作流”；
- 辅助入口不新增独立页面壳层，不改变项目概览现有布局。

## 4. 本迭代范围

### 4.1 包含

- `ProjectWorkflowBinding` 数据模型和迁移；
- 查询项目四个环节的绑定状态；
- 按环节查询可选的全局工作流；
- 创建绑定；
- 更换绑定；
- 解除绑定；
- 幂等、乐观锁和 409 冲突处理；
- 绑定创建、更换、解除的 Audit；
- 未绑定、部分绑定、全部绑定、无候选、依赖异常和冲突 UI；
- 真实 API 与真实 PostgreSQL 联调。

### 4.2 不包含

- 工作流验证、启用或停用；
- n8n、Webhook 或其他第三方调用；
- `WorkflowRun`、异步队列、执行日志和重试；
- 项目级默认参数覆盖；
- 章节规划、正文生成、审核或改写的实际执行；
- 新增项目基础信息、权限管理或其他项目设置功能；
- 修改全局工作流配置本体。

## 5. 数据模型

本迭代只新增或修改 `ProjectWorkflowBinding`。详细字段和约束见 `data-model.md`。

## 6. API

拟冻结范围：

- `GET /projects/{projectId}/workflow-bindings`
- `PUT /projects/{projectId}/workflow-bindings/{stage}`
- `DELETE /projects/{projectId}/workflow-bindings/{stage}`
- 复用 Iteration 12 的全局工作流列表 API，并在 Iteration 13 契约中增量增加可选 `applicableStage` 查询参数；候选列表保留已停用项用于状态说明，但已停用项不可选择，绑定命令必须再次校验适用环节与启用状态。

精确请求体、响应体、错误码和并发字段在 OpenAPI 冻结阶段确定，范围约束见 `api-scope.yaml`。

## 7. UI 基线

| Frame | 用途 |
|---|---|
| `P13_01_PROJECT_SETTINGS_ENTRY` | 项目设置壳层与工作流入口参考；基础信息编辑不在本迭代 |
| `P13_02_WORKFLOW_BINDINGS_UNBOUND` | 四个环节全部未绑定 |
| `P13_03_SELECT_WORKFLOW_DRAWER` | 首次选择工作流 |
| `P13_04_WORKFLOW_BINDINGS_PARTIAL` | 部分已绑定 |
| `P13_05_REPLACE_WORKFLOW_DRAWER` | 更换工作流 |
| `P13_06_UNBIND_CONFIRM_DIALOG` | 解除绑定确认 |
| `P13_07_WORKFLOW_BINDINGS_COMPLETE` | 全部已绑定 |
| `P13_08_NO_AVAILABLE_WORKFLOW` | 当前环节无可用工作流 |
| `P13_09_WORKFLOW_BINDING_EXCEPTIONS` | 已绑定依赖异常 |
| `P13_10_BINDING_CONFLICT` | 409 并发冲突 |

原型只冻结内容结构、状态层级和交互意图。项目工作区外壳、导航、术语、响应式尺寸和可访问性必须按现有产品实现修正，详见 `ui-scope.md`。

## 8. 实施顺序

1. 契约冻结：冻结范围、数据模型、OpenAPI、状态码、Audit、UI Frame 和不做内容；
2. 后端开发：Migration、Domain、Repository、Service、Handler、幂等、并发与真实数据库测试；
3. 前端与 UI：按冻结 Frame 开发所有状态，复用现有项目工作区壳层；
4. 前后端联调：真实 API、真实 PostgreSQL、浏览器回归与最终审核。

每个阶段按小任务执行，自验 PASS 后同轮 Commit、Push。

## 9. 完成定义

- [ ] 四个固定环节可独立绑定、换绑和解绑；
- [ ] 候选工作流按适用环节过滤；
- [ ] 刷新后绑定关系恢复；
- [ ] 不发生跨项目数据串联；
- [ ] PUT/DELETE 幂等与乐观锁通过；
- [ ] 409 保留用户选择并支持加载最新配置；
- [ ] 已停用、未接入和连接异常不会被误显示为“可执行”；
- [ ] 不产生任何第三方出站请求；
- [ ] 页面无重复工作区壳层；
- [ ] 自动测试、真实数据库验证、人工 UI 验收和完整审核包通过。
