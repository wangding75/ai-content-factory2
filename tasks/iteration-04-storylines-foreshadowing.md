# Iteration 04｜故事线与伏笔闭环

## 1. 目标

完成主线、子故事线和跨故事线伏笔的创建与持久化。

## 2. 前置依赖

Iteration 03

## 3. 闭环链路

1. B1_STORYLINES → B2_CREATE_MAIN_STORYLINE → 返回 B1
2. 选中主线 → B3_CREATE_CHILD_STORYLINE → 返回 B1
3. B1_STORYLINES → B4_CREATE_FORESHADOWING → 返回 B1
4. 刷新后树结构与伏笔种下/回收关系保持

## 4. UI 范围

- `B1_STORYLINES`｜故事线工作区
- `B2_CREATE_MAIN_STORYLINE`｜新建主线
- `B3_CREATE_CHILD_STORYLINE`｜新建子故事线
- `B4_CREATE_FORESHADOWING`｜新增伏笔

UI 以 `ui/frames/<FRAME_ID>/` 内 Stitch 冻结稿为视觉基线。实现时允许组件化重构，但不得改变页面业务含义、字段、状态和入口。

## 5. API 范围

| 方法 | 路径 / 契约 | 用途 |
|---|---|---|
| GET | /api/v1/projects/{projectId}/storylines | 故事线树 |
| POST | /api/v1/projects/{projectId}/storylines | 创建主线 |
| POST | /api/v1/storylines/{storylineId}/children | 创建子故事线 |
| PATCH | /api/v1/storylines/{storylineId} | 编辑故事线 |
| GET | /api/v1/projects/{projectId}/foreshadowings | 伏笔列表 |
| POST | /api/v1/projects/{projectId}/foreshadowings | 创建伏笔 |
| PATCH | /api/v1/foreshadowings/{foreshadowingId} | 编辑伏笔与状态 |

所有 HTTP API 均使用 `/api/v1` 前缀和统一响应：

```json
{
  "data": {},
  "request_id": "req_xxx"
}
```

写接口必须明确校验、状态变化、事务边界和幂等策略。

## 6. 数据模型

- `PlotLine`
- `PlotLineType`
- `PlotLineRelation`
- `Foreshadowing`
- `ForeshadowingStatus`

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
| I04-AC01 | 创建主线 | 创建主线后 B1 左侧树出现新根节点，刷新后仍存在。 |
| I04-AC02 | 创建子线 | 子故事线 parent_id 指向所选主线，并在正确父节点下展示。 |
| I04-AC03 | 跨线伏笔 | 伏笔可分别关联种下故事线和计划回收故事线。 |
| I04-AC04 | 非法父级校验 | 不存在或跨项目的 parent_id 返回 400/404，不产生数据。 |
| I04-AC05 | 树顺序稳定 | 同级 sort_order 可持久化，刷新后顺序不漂移。 |

## 9. 自动化测试要求

- Domain：状态机、值对象、校验规则单元测试。
- Application：成功、失败、幂等和事务回滚分支。
- Repository：Testcontainers PostgreSQL 集成测试。
- HTTP：OpenAPI 契约测试、错误码和 request_id。
- Web：组件测试、API Mock 测试、关键页面状态测试。
- E2E：至少覆盖本迭代闭环链路，不能仅验证页面可打开。

## 10. 明确排除

- 拖拽排序 UI
- 故事线删除级联
- AI 自动生成故事线
- 复杂图谱可视化

## 11. 完成定义

- 本迭代所有核心需求均有可执行验收用例。
- UI、API、数据模型和状态机无未审批漂移。
- 迁移可在空库执行，测试数据可重复创建。
- Go test、Web typecheck/lint、契约测试和本迭代 E2E 通过。
- 文档、实现报告和差异说明已更新。
- Git 工作区干净，并完成单次迭代提交。
