# Iteration 03｜项目策划与素材闭环

## 1. 目标

完成项目策划保存，以及素材本体、项目用途、全局同步、绑定与解绑的完整闭环。

## 2. 前置依赖

Iteration 02

## 3. 闭环链路

1. S02_PROJECT_OVERVIEW → S02_PROJECT_PLANNING，保存项目策划
2. S02_PROJECT_MATERIALS → S02_CREATE_MATERIAL
3. 创建 Material，并自动建立当前项目的 ProjectMaterialUsage
4. S02_MATERIAL_DETAIL → 编辑素材本体或仅编辑项目用途
5. S02_PICK_MATERIAL 绑定全局已有素材，不重复创建 Material
6. S02_UNBIND_MATERIAL 仅解除项目关系，全局素材保留

## 4. UI 范围

- `S02_PROJECT_PLANNING`｜项目策划
- `S02_PROJECT_MATERIALS`｜项目素材列表
- `S02_CREATE_MATERIAL`｜新建并绑定素材
- `S02_MATERIAL_DETAIL`｜素材详情
- `S02_PICK_MATERIAL`｜选择已有素材
- `S02_EDIT_MATERIAL`｜编辑素材
- `S02_EDIT_MATERIAL_USAGE`｜编辑项目用途
- `S02_UNBIND_MATERIAL`｜解除素材绑定

UI 以 `ui/frames/<FRAME_ID>/` 内 Stitch 冻结稿为视觉基线。实现时允许组件化重构，但不得改变页面业务含义、字段、状态和入口。

## 5. API 范围

| 方法 | 路径 / 契约 | 用途 |
|---|---|---|
| GET | /api/v1/projects/{projectId}/planning | 读取项目策划 |
| PUT | /api/v1/projects/{projectId}/planning | 保存项目策划 |
| GET | /api/v1/materials | 全局素材查询 |
| POST | /api/v1/materials | 创建全局素材 |
| GET | /api/v1/materials/{materialId} | 素材详情 |
| PATCH | /api/v1/materials/{materialId} | 编辑素材本体 |
| GET | /api/v1/projects/{projectId}/materials | 项目素材列表 |
| POST | /api/v1/projects/{projectId}/materials | 创建素材并绑定项目 |
| POST | /api/v1/projects/{projectId}/materials/{materialId}/binding | 绑定已有素材 |
| PATCH | /api/v1/projects/{projectId}/materials/{materialId}/usage | 编辑项目用途 |
| DELETE | /api/v1/projects/{projectId}/materials/{materialId}/binding | 解除项目绑定 |

所有 HTTP API 均使用 `/api/v1` 前缀和统一响应：

```json
{
  "data": {},
  "request_id": "req_xxx"
}
```

写接口必须明确校验、状态变化、事务边界和幂等策略。

## 6. 数据模型

- `ProjectPlanning`
- `Material`
- `MaterialType`
- `ProjectMaterialUsage`
- `MaterialReferenceReadModel`

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
| I03-AC01 | 项目内创建自动全局同步 | 在项目内创建素材后，Material 只创建 1 条；项目关系创建 1 条；全局素材查询可见。 |
| I03-AC02 | 素材本体与用途分离 | 编辑 Material 名称会影响所有引用；编辑 ProjectMaterialUsage 只影响当前项目。 |
| I03-AC03 | 绑定不重复 | 绑定已有素材只新增 usage，不新增重复 Material。 |
| I03-AC04 | 解绑不删除全局素材 | 解绑后项目列表不再显示，但 GET /materials/{id} 仍可查询。 |
| I03-AC05 | 策划持久化 | 保存项目策划后刷新页面，字段保持一致。 |
| I03-AC06 | 并发约束 | 同一项目与同一素材不得存在重复有效绑定。 |

## 9. 自动化测试要求

- Domain：状态机、值对象、校验规则单元测试。
- Application：成功、失败、幂等和事务回滚分支。
- Repository：Testcontainers PostgreSQL 集成测试。
- HTTP：OpenAPI 契约测试、错误码和 request_id。
- Web：组件测试、API Mock 测试、关键页面状态测试。
- E2E：至少覆盖本迭代闭环链路，不能仅验证页面可打开。

## 10. 明确排除

- 素材版本历史
- 附件上传
- 批量导入
- AI 抽取素材
- 跨项目批量绑定

## 11. 完成定义

- 本迭代所有核心需求均有可执行验收用例。
- UI、API、数据模型和状态机无未审批漂移。
- 迁移可在空库执行，测试数据可重复创建。
- Go test、Web typecheck/lint、契约测试和本迭代 E2E 通过。
- 文档、实现报告和差异说明已更新。
- Git 工作区干净，并完成单次迭代提交。
