# Iteration 02｜首页与项目创建闭环

## 1. 目标

用户可以从首页进入项目列表，创建小说项目并进入项目概览；刷新后数据仍存在。

## 2. 前置依赖

Iteration 01

## 3. 闭环链路

1. S00_HOME → S01_PROJECTS
2. S01_PROJECTS → S01_CREATE_PROJECT
3. 填写项目名称、项目类型与基础信息
4. 确认创建 → S02_PROJECT_OVERVIEW
5. 返回 S01_PROJECTS，新项目仍可见

## 4. UI 范围

- `S00_HOME`｜首页
- `S01_PROJECTS`｜项目列表
- `S01_CREATE_PROJECT`｜新建项目
- `S02_PROJECT_OVERVIEW`｜项目概览

UI 以 `ui/frames/<FRAME_ID>/` 内 Stitch 冻结稿为视觉基线。实现时允许组件化重构，但不得改变页面业务含义、字段、状态和入口。

## 5. API 范围

| 方法 | 路径 / 契约 | 用途 |
|---|---|---|
| GET | /api/v1/home | 首页聚合数据 |
| GET | /api/v1/projects | 项目列表 |
| POST | /api/v1/projects | 创建项目 |
| GET | /api/v1/projects/{projectId} | 项目详情 |
| PATCH | /api/v1/projects/{projectId} | 更新项目基础信息 |
| GET | /api/v1/projects/{projectId}/workspace | 项目工作区聚合数据 |

所有 HTTP API 均使用 `/api/v1` 前缀和统一响应：

```json
{
  "data": {},
  "request_id": "req_xxx"
}
```

写接口必须明确校验、状态变化、事务边界和幂等策略。

## 6. 数据模型

- `Project`
- `ProjectType`
- `ProjectStatus`
- `ProjectWorkspaceReadModel`

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
| I02-AC01 | 创建并进入项目 | 提交合法项目后创建成功，并进入同一 projectId 的 S02_PROJECT_OVERVIEW。 |
| I02-AC02 | 列表持久化 | 创建后重新进入或刷新 S01_PROJECTS，刚创建的项目仍显示。 |
| I02-AC03 | 校验失败无脏数据 | 名称为空或类型非法时返回 400，数据库不新增 Project。 |
| I02-AC04 | 概览一致性 | 项目概览名称、类型、状态与 GET /projects/{id} 一致。 |
| I02-AC05 | 首页最近项目 | 创建或更新项目后，S00_HOME 最近项目按更新时间正确展示。 |

## 9. 自动化测试要求

- Domain：状态机、值对象、校验规则单元测试。
- Application：成功、失败、幂等和事务回滚分支。
- Repository：Testcontainers PostgreSQL 集成测试。
- HTTP：OpenAPI 契约测试、错误码和 request_id。
- Web：组件测试、API Mock 测试、关键页面状态测试。
- E2E：至少覆盖本迭代闭环链路，不能仅验证页面可打开。

## 10. 明确排除

- 删除项目
- 项目复制
- 成员协作
- 项目设置页面
- 非小说项目实现

## 11. 完成定义

- 本迭代所有核心需求均有可执行验收用例。
- UI、API、数据模型和状态机无未审批漂移。
- 迁移可在空库执行，测试数据可重复创建。
- Go test、Web typecheck/lint、契约测试和本迭代 E2E 通过。
- 文档、实现报告和差异说明已更新。
- Git 工作区干净，并完成单次迭代提交。
