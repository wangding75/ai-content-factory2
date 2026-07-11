# Iteration 01｜技术骨架与基础设施

## 1. 目标

完全复用 1.0 技术骨架，建立可启动、可迁移、可测试、可部署的 2.0 P0 工程。

## 2. 前置依赖

Iteration 00

## 3. 闭环链路

1. 启动 PostgreSQL、Redis、Go API、Next.js Web
2. API 健康检查成功
3. Web 加载 ProductShell 与全局导航
4. OpenAPI Client、日志、错误处理、迁移和测试框架可用

## 4. UI 范围

- `S00_HOME`｜首页

UI 以 `ui/frames/<FRAME_ID>/` 内 Stitch 冻结稿为视觉基线。实现时允许组件化重构，但不得改变页面业务含义、字段、状态和入口。

## 5. API 范围

| 方法 | 路径 / 契约 | 用途 |
|---|---|---|
| GET | /healthz | API 存活检查 |
| GET | /readyz | PostgreSQL、Redis 依赖就绪检查 |
| GET | /api/v1/meta | 版本、环境与 P0 能力元信息 |

所有 HTTP API 均使用 `/api/v1` 前缀和统一响应：

```json
{
  "data": {},
  "request_id": "req_xxx"
}
```

写接口必须明确校验、状态变化、事务边界和幂等策略。

## 6. 数据模型

- `AuditLog`
- `IdempotencyRecord（按需）`
- `AppMeta（只读 DTO）`

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
| I01-AC01 | 一键启动 | docker compose 启动 API:8080、Web:3000、PostgreSQL、Redis。 |
| I01-AC02 | 健康检查 | /healthz 返回 200；/readyz 在依赖就绪时返回 200。 |
| I01-AC03 | 数据库迁移 | 空库可正向迁移到最新版本；测试库可重复创建。 |
| I01-AC04 | 前端骨架 | S00_HOME 可访问，全局导航和统一 API Client 已接入。 |
| I01-AC05 | 质量门禁 | Go test、Web lint/typecheck、契约校验、Playwright 基础用例通过。 |

## 9. 自动化测试要求

- Domain：状态机、值对象、校验规则单元测试。
- Application：成功、失败、幂等和事务回滚分支。
- Repository：Testcontainers PostgreSQL 集成测试。
- HTTP：OpenAPI 契约测试、错误码和 request_id。
- Web：组件测试、API Mock 测试、关键页面状态测试。
- E2E：至少覆盖本迭代闭环链路，不能仅验证页面可打开。

## 10. 明确排除

- 任何真实业务写入
- 用户登录与组织权限
- 真实对象存储
- 外部 Provider

## 11. 完成定义

- 本迭代所有核心需求均有可执行验收用例。
- UI、API、数据模型和状态机无未审批漂移。
- 迁移可在空库执行，测试数据可重复创建。
- Go test、Web typecheck/lint、契约测试和本迭代 E2E 通过。
- 文档、实现报告和差异说明已更新。
- Git 工作区干净，并完成单次迭代提交。
