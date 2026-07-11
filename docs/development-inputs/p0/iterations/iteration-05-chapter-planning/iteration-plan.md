# Iteration 05｜章节规划与确认闭环

## 1. 目标

完成模拟生成候选章节、编辑/删除/选择、确认，以及确认后才允许进入正文生产。

## 2. 前置依赖

Iteration 04

## 3. 闭环链路

1. C1_CHAPTER_PLANNING → C2_MOCK_PLAN → 生成候选并返回 C1
2. C1 → C3_EDIT_PLAN → 保存并返回 C1
3. 在 C1 选择候选 → C4_CONFIRM_PLAN → 确认并返回 C1
4. 确认后的章节显示“已确认”并出现“进入正文生产”入口

## 4. UI 范围

- `C1_CHAPTER_PLANNING`｜章节规划
- `C2_MOCK_PLAN`｜模拟生成章节规划
- `C3_EDIT_PLAN`｜编辑章节规划
- `C4_CONFIRM_PLAN`｜确认章节规划

UI 以 `ui/frames/<FRAME_ID>/` 内 Stitch 冻结稿为视觉基线。实现时允许组件化重构，但不得改变页面业务含义、字段、状态和入口。

## 5. API 范围

| 方法 | 路径 / 契约 | 用途 |
|---|---|---|
| GET | /api/v1/projects/{projectId}/chapter-plans | 章节规划列表 |
| POST | /api/v1/projects/{projectId}/chapter-plans/mock-generate | 模拟生成候选 |
| GET | /api/v1/chapter-plans/{chapterPlanId} | 章节规划详情 |
| PATCH | /api/v1/chapter-plans/{chapterPlanId} | 编辑候选规划 |
| DELETE | /api/v1/chapter-plans/{chapterPlanId} | 删除未确认候选 |
| POST | /api/v1/projects/{projectId}/chapter-plans/confirm | 批量确认候选 |

所有 HTTP API 均使用 `/api/v1` 前缀和统一响应：

```json
{
  "data": {},
  "request_id": "req_xxx"
}
```

写接口必须明确校验、状态变化、事务边界和幂等策略。

## 6. 数据模型

- `ChapterPlan`
- `ChapterPlanStatus`
- `ChapterPlanSource`
- `ChapterPlanRelation`
- `MockGenerationRun`

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
| I05-AC01 | 模拟生成只产生候选 | 生成后状态为 pending_confirmation，不自动创建正文。 |
| I05-AC02 | 编辑持久化 | 编辑标题、摘要、故事线、素材或伏笔关系后刷新仍一致。 |
| I05-AC03 | 选择性确认 | 仅选中的候选转为 confirmed；未选中的保持待确认。 |
| I05-AC04 | 确认来源保留 | 确认后 source=mock_generated 等来源字段不丢失。 |
| I05-AC05 | 生产门槛 | 未确认章节调用创建正文接口返回 409；已确认章节允许进入 D1_EDITOR。 |
| I05-AC06 | 确认幂等 | 重复提交同一批确认请求不会重复创建章节或破坏状态。 |

## 9. 自动化测试要求

- Domain：状态机、值对象、校验规则单元测试。
- Application：成功、失败、幂等和事务回滚分支。
- Repository：Testcontainers PostgreSQL 集成测试。
- HTTP：OpenAPI 契约测试、错误码和 request_id。
- Web：组件测试、API Mock 测试、关键页面状态测试。
- E2E：至少覆盖本迭代闭环链路，不能仅验证页面可打开。

## 10. 明确排除

- 真实 LLM 规划
- 自动进入正文
- 自动覆盖已有确认章节
- 批量重排章节

## 11. 完成定义

- 本迭代所有核心需求均有可执行验收用例。
- UI、API、数据模型和状态机无未审批漂移。
- 迁移可在空库执行，测试数据可重复创建。
- Go test、Web typecheck/lint、契约测试和本迭代 E2E 通过。
- 文档、实现报告和差异说明已更新。
- Git 工作区干净，并完成单次迭代提交。
