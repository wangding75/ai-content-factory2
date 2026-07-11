# Iteration 06｜正文编辑与模拟审核闭环

## 1. 目标

从已确认章节创建正文、保存草稿、模拟生成 v1、提交模拟审核，并保证审核不覆盖正文。

## 2. 前置依赖

Iteration 05

## 3. 闭环链路

1. C1 已确认章节 → D1_EDITOR
2. 创建或模拟生成正文 v1
3. 人工编辑并保存草稿
4. 提交审核 → D2_REVIEW
5. 返回 D1_EDITOR，正文内容和版本保持不变

## 4. UI 范围

- `D1_EDITOR`｜正文编辑器
- `D2_REVIEW`｜审核结果

UI 以 `ui/frames/<FRAME_ID>/` 内 Stitch 冻结稿为视觉基线。实现时允许组件化重构，但不得改变页面业务含义、字段、状态和入口。

## 5. API 范围

| 方法 | 路径 / 契约 | 用途 |
|---|---|---|
| POST | /api/v1/chapter-plans/{chapterPlanId}/content | 为已确认章节创建内容单元 |
| GET | /api/v1/content-items/{contentItemId} | 正文与当前版本详情 |
| PUT | /api/v1/content-items/{contentItemId}/draft | 保存草稿 |
| POST | /api/v1/content-items/{contentItemId}/mock-generate | 模拟生成正文 |
| POST | /api/v1/content-items/{contentItemId}/reviews/mock | 执行模拟审核 |
| GET | /api/v1/content-items/{contentItemId}/reviews | 审核历史 |
| GET | /api/v1/reviews/{reviewId} | 审核详情 |

所有 HTTP API 均使用 `/api/v1` 前缀和统一响应：

```json
{
  "data": {},
  "request_id": "req_xxx"
}
```

写接口必须明确校验、状态变化、事务边界和幂等策略。

## 6. 数据模型

- `ContentItem`
- `ContentVersion`
- `ReviewReport`
- `ReviewFinding`
- `ReviewRecommendation`
- `ReviewStatus`

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
| I06-AC01 | 创建正文门槛 | 只有 confirmed ChapterPlan 才能创建 ContentItem。 |
| I06-AC02 | 草稿保存 | 编辑正文并保存后刷新，正文、字数和更新时间一致。 |
| I06-AC03 | 模拟生成 v1 | 首次模拟生成创建 v1，并记录 source=mock_generated。 |
| I06-AC04 | 审核结果完整 | 模拟审核产生可展示的问题与建议集合，D2 可正确渲染。 |
| I06-AC05 | 审核不覆盖正文 | 创建 ReviewReport 前后 ContentVersion 正文哈希不变。 |
| I06-AC06 | 返回编辑器 | 从 D2 返回 D1 后加载同一 contentItemId 和同一版本。 |

## 9. 自动化测试要求

- Domain：状态机、值对象、校验规则单元测试。
- Application：成功、失败、幂等和事务回滚分支。
- Repository：Testcontainers PostgreSQL 集成测试。
- HTTP：OpenAPI 契约测试、错误码和 request_id。
- Web：组件测试、API Mock 测试、关键页面状态测试。
- E2E：至少覆盖本迭代闭环链路，不能仅验证页面可打开。

## 10. 明确排除

- 真实 LLM 正文生成
- 流式生成
- 多人实时协作
- 评论批注
- 审核通过/发布

## 11. 完成定义

- 本迭代所有核心需求均有可执行验收用例。
- UI、API、数据模型和状态机无未审批漂移。
- 迁移可在空库执行，测试数据可重复创建。
- Go test、Web typecheck/lint、契约测试和本迭代 E2E 通过。
- 文档、实现报告和差异说明已更新。
- Git 工作区干净，并完成单次迭代提交。
