# Iteration 09｜P0 全链路联调与系统验收

## 1. 目标

按每个迭代核心需求完成真实数据驱动的全链路验收，禁止只做页面与接口冒烟。

## 2. 前置依赖

Iteration 08

## 3. 闭环链路

1. 创建项目
2. 保存策划并创建/绑定素材
3. 创建主线、子线和伏笔
4. 模拟生成、编辑并确认章节规划
5. 进入正文、保存并提交模拟审核
6. 根据审核创建 v2，同时保留 v1
7. 在项目作品与全局页面查看聚合结果
8. 重启环境后复核所有持久化数据

## 4. UI 范围

- `S00_HOME`｜首页
- `S01_PROJECTS`｜项目列表
- `S01_CREATE_PROJECT`｜新建项目
- `S02_PROJECT_OVERVIEW`｜项目概览
- `S02_PROJECT_PLANNING`｜项目策划
- `S02_PROJECT_MATERIALS`｜项目素材列表
- `S02_CREATE_MATERIAL`｜新建并绑定素材
- `S02_MATERIAL_DETAIL`｜素材详情
- `S02_PICK_MATERIAL`｜选择已有素材
- `S02_EDIT_MATERIAL`｜编辑素材
- `S02_EDIT_MATERIAL_USAGE`｜编辑项目用途
- `S02_UNBIND_MATERIAL`｜解除素材绑定
- `B1_STORYLINES`｜故事线工作区
- `B2_CREATE_MAIN_STORYLINE`｜新建主线
- `B3_CREATE_CHILD_STORYLINE`｜新建子故事线
- `B4_CREATE_FORESHADOWING`｜新增伏笔
- `C1_CHAPTER_PLANNING`｜章节规划
- `C2_MOCK_PLAN`｜模拟生成章节规划
- `C3_EDIT_PLAN`｜编辑章节规划
- `C4_CONFIRM_PLAN`｜确认章节规划
- `D1_EDITOR`｜正文编辑器
- `D2_REVIEW`｜审核结果
- `D3_PROJECT_WORKS`｜项目作品
- `D4_CREATE_REWRITE`｜创建重写版本
- `D5_REWRITE_RESULT`｜重写结果
- `E1_GLOBAL_MATERIALS`｜全局素材
- `E2_GLOBAL_WORKS`｜全局作品
- `E3_WORKFLOWS`｜流程中心
- `E4_SETTINGS`｜全局设置

UI 以 `ui/frames/<FRAME_ID>/` 内 Stitch 冻结稿为视觉基线。实现时允许组件化重构，但不得改变页面业务含义、字段、状态和入口。

## 5. API 范围

| 方法 | 路径 / 契约 | 用途 |
|---|---|---|
| ALL | /api/v1/** | 执行全部 P0 契约、集成与回归测试 |

所有 HTTP API 均使用 `/api/v1` 前缀和统一响应：

```json
{
  "data": {},
  "request_id": "req_xxx"
}
```

写接口必须明确校验、状态变化、事务边界和幂等策略。

## 6. 数据模型

- `全部 P0 领域模型、状态机、读模型与审计记录`

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
| I09-AC01 | 完整主链路 | 从空数据库开始，用户可完成项目→素材→故事线→章节→正文→审核→重写→作品。 |
| I09-AC02 | 迭代需求全覆盖 | Iteration 02–08 每个核心需求至少有 1 条可重复执行的 E2E 用例。 |
| I09-AC03 | 持久化回归 | 停止并重新启动 API/Web 后，主链路数据、引用和版本关系不丢失。 |
| I09-AC04 | 契约回归 | OpenAPI、错误码、DTO、数据库迁移和前后端调用无漂移。 |
| I09-AC05 | UI 回归 | 关键页面与冻结 UI 在布局、字段、状态和交互入口上无业务性漂移。 |
| I09-AC06 | 边界回归 | 真实 AI、外部工作流和发布平台仍保持禁用，不产生伪成功数据。 |
| I09-AC07 | 质量门禁 | Go 单测/集成测试、Web typecheck/unit、Playwright、契约测试全部通过。 |

## 9. 自动化测试要求

- Domain：状态机、值对象、校验规则单元测试。
- Application：成功、失败、幂等和事务回滚分支。
- Repository：Testcontainers PostgreSQL 集成测试。
- HTTP：OpenAPI 契约测试、错误码和 request_id。
- Web：组件测试、API Mock 测试、关键页面状态测试。
- E2E：至少覆盖本迭代闭环链路，不能仅验证页面可打开。

## 10. 明确排除

- P1/P2 功能
- 性能压测结论
- 生产部署
- 真实第三方平台联调

## 11. 完成定义

- 本迭代所有核心需求均有可执行验收用例。
- UI、API、数据模型和状态机无未审批漂移。
- 迁移可在空库执行，测试数据可重复创建。
- Go test、Web typecheck/lint、契约测试和本迭代 E2E 通过。
- 文档、实现报告和差异说明已更新。
- Git 工作区干净，并完成单次迭代提交。
