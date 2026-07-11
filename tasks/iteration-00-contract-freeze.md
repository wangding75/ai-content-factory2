# Iteration 00｜P0 契约与基线冻结

## 1. 目标

冻结页面 ID、闭环链路、API、领域对象、状态机与验收追踪关系，后续迭代不得无审批漂移。

## 2. 前置依赖

无

## 3. 闭环链路

1. 读取 P0 业务链路、页面链路、UI Frame 与 API 契约
2. 建立 Page → Action → API → Model → Acceptance 追踪矩阵
3. 冻结统一响应、错误码、分页、ID、时间与审计字段
4. 冻结 Novel Content Pack 与内置 Mock Provider 的 P0 边界

## 4. UI 范围

- 本迭代不实现业务页面，仅冻结全局页面注册表。

UI 以 `ui/frames/<FRAME_ID>/` 内 Stitch 冻结稿为视觉基线。实现时允许组件化重构，但不得改变页面业务含义、字段、状态和入口。

## 5. API 范围

| 方法 | 路径 / 契约 | 用途 |
|---|---|---|
| CONTRACT | packages/contracts/openapi/openapi.yaml | 冻结 P0 OpenAPI |
| CONTRACT | packages/contracts/common/*.schema.json | 冻结统一响应、错误和分页 |
| CONTRACT | packages/contracts/content-packs/novel/* | 冻结 Novel Pack Schema |

所有 HTTP API 均使用 `/api/v1` 前缀和统一响应：

```json
{
  "data": {},
  "request_id": "req_xxx"
}
```

写接口必须明确校验、状态变化、事务边界和幂等策略。

## 6. 数据模型

- `通用 ID 值对象与审计字段`
- `ProjectStatus`
- `MaterialType`
- `PlotLineType`
- `ForeshadowingStatus`
- `ChapterPlanStatus`
- `ContentStatus`
- `ReviewStatus`
- `WorkflowRunStatus`
- `统一 API Error`

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
| I00-AC01 | 页面注册完整 | 29 个 P0 Frame 均存在唯一 ID、中文名称和归属迭代。 |
| I00-AC02 | 接口追踪完整 | 每个 P0 可点击业务动作均可追踪到接口或明确标记为纯导航。 |
| I00-AC03 | 模型追踪完整 | 每个写接口都能追踪到领域对象、状态变化和持久化表。 |
| I00-AC04 | OpenAPI 有效 | OpenAPI 可解析；operationId 唯一；统一响应包含 request_id。 |
| I00-AC05 | 边界冻结 | P0 明确只实现 Novel Pack、内置 Mock Provider；真实 AI、外部工作流、发布集成不执行。 |

## 9. 自动化测试要求

- Domain：状态机、值对象、校验规则单元测试。
- Application：成功、失败、幂等和事务回滚分支。
- Repository：Testcontainers PostgreSQL 集成测试。
- HTTP：OpenAPI 契约测试、错误码和 request_id。
- Web：组件测试、API Mock 测试、关键页面状态测试。
- E2E：至少覆盖本迭代闭环链路，不能仅验证页面可打开。

## 10. 明确排除

- 业务功能实现
- 数据库业务表落地
- 真实 AI 接入
- 外部工作流调用
- 发布平台接入

## 11. 完成定义

- 本迭代所有核心需求均有可执行验收用例。
- UI、API、数据模型和状态机无未审批漂移。
- 迁移可在空库执行，测试数据可重复创建。
- Go test、Web typecheck/lint、契约测试和本迭代 E2E 通过。
- 文档、实现报告和差异说明已更新。
- Git 工作区干净，并完成单次迭代提交。
