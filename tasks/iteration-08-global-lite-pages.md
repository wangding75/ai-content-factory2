# Iteration 08｜全局 Lite 页面与能力展示

## 1. 目标

完成全局素材、全局作品、流程中心和全局设置的只读/轻交互聚合能力。

## 2. 前置依赖

Iteration 07

## 3. 闭环链路

1. 全局导航 → E1_GLOBAL_MATERIALS，查看跨项目素材及引用
2. 全局导航 → E2_GLOBAL_WORKS，打开源项目、正文或审核
3. 全局导航 → E3_WORKFLOWS，查看内置模拟流程及产物
4. 全局导航 → E4_SETTINGS，查看模拟能力、真实 AI 与外部集成状态

## 4. UI 范围

- `E1_GLOBAL_MATERIALS`｜全局素材
- `E2_GLOBAL_WORKS`｜全局作品
- `E3_WORKFLOWS`｜流程中心
- `E4_SETTINGS`｜全局设置

UI 以 `ui/frames/<FRAME_ID>/` 内 Stitch 冻结稿为视觉基线。实现时允许组件化重构，但不得改变页面业务含义、字段、状态和入口。

## 5. API 范围

| 方法 | 路径 / 契约 | 用途 |
|---|---|---|
| GET | /api/v1/materials?scope=global | 全局素材聚合 |
| GET | /api/v1/works?scope=global | 全局作品聚合 |
| GET | /api/v1/workflows/builtin | 内置模拟流程 |
| GET | /api/v1/capabilities | 能力状态 |
| GET | /api/v1/integrations | 外部集成状态 |

所有 HTTP API 均使用 `/api/v1` 前缀和统一响应：

```json
{
  "data": {},
  "request_id": "req_xxx"
}
```

写接口必须明确校验、状态变化、事务边界和幂等策略。

## 6. 数据模型

- `GlobalMaterialReadModel`
- `GlobalWorkReadModel`
- `BuiltinWorkflowDefinition`
- `CapabilityDescriptor`
- `IntegrationDescriptor`

详细字段与关系见本目录 `data-model.md`，全局定义见 `../../baselines/p0-data-model-catalog.md`。

## 7. 开发任务

### 后端

- 按 1.0 的 `interfaces → application → domain` 分层实现。
- Handler 不直接访问 Repository，不在 Handler 编写业务规则。
- Repository Interface 定义在 domain，PostgreSQL 实现在 infrastructure。
- 状态变化写入 AuditLog；核心写操作必须有事务。
- P0 的生成、审核、重写统一通过内置 `mock` WorkflowProvider，禁止散落硬编码。

### 前端

- 复用 1.0 的 Next.js、feature 目录、API Client、Query 和统一错误处理。
- 页面路由使用 Frame ID 对应的稳定 route key。
- 先以 fixture 对齐冻结 UI，再接入真实 API。
- 加载、空态、错误态、禁用态必须实现；不得只实现成功态。
- 页面不得直接拼接未经契约定义的 DTO。

### 契约与数据库

- 先更新 OpenAPI / JSON Schema，再实现代码。
- 每个新表必须有 migration 和 repository integration test。
- 不允许为内容类型差异复制一整套 Core；Novel 差异进入 Novel Pack 或 payload/metadata。
- API、数据库、UI 字段命名保持可追踪。

## 8. 验收方案

| 用例 ID | 场景 | 通过标准 |
|---|---|---|
| I08-AC01 | 全局素材聚合 | 项目内创建的素材在 E1 可见，并展示正确引用项目与用途摘要。 |
| I08-AC02 | 全局作品聚合 | E2 可跨项目展示作品，并能定位到源项目、正文和审核。 |
| I08-AC03 | 流程中心边界 | E3 只显示内置模拟流程，不执行 n8n/Coze/ComfyUI。 |
| I08-AC04 | 设置状态准确 | 模拟能力=已启用；真实 AI=暂未配置；发布与外部工作流=暂未开放。 |
| I08-AC05 | 无虚假配置 | P0 页面不得出现 API Key、OAuth、连接成功或真实调用记录。 |

## 9. 自动化测试要求

- Domain：状态机、值对象、校验规则单元测试。
- Application：成功、失败、幂等和事务回滚分支。
- Repository：Testcontainers PostgreSQL 集成测试。
- HTTP：OpenAPI 契约测试、错误码和 request_id。
- Web：组件测试、API Mock 测试、关键页面状态测试。
- E2E：至少覆盖本迭代闭环链路，不能仅验证页面可打开。

## 10. 明确排除

- 创建全局素材
- 真实 Provider 配置
- 外部工作流连接
- 发布平台授权
- 账单订阅

## 11. 完成定义

- 本迭代所有核心需求均有可执行验收用例。
- UI、API、数据模型和状态机无未审批漂移。
- 迁移可在空库执行，测试数据可重复创建。
- Go test、Web typecheck/lint、契约测试和本迭代 E2E 通过。
- 文档、实现报告和差异说明已更新。
- Git 工作区干净，并完成单次迭代提交。
