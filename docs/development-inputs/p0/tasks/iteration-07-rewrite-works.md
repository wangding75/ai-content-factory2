# Iteration 07｜重写版本与项目作品闭环

## 1. 目标

根据审核结果创建重写版本 v2，保留 v1，并在项目作品页集中查看正文、审核和版本。

## 2. 前置依赖

Iteration 06

## 3. 闭环链路

1. D2_REVIEW 或 D3_PROJECT_WORKS → D4_CREATE_REWRITE
2. 确认创建 → 内置 Mock Provider 生成 v2 → D5_REWRITE_RESULT
3. v1 保留，v2 不自动设为当前、不自动提交审核、不自动发布
4. D5 可返回 D1、D2 或 D3 查看对应数据

## 4. UI 范围

- `D3_PROJECT_WORKS`｜项目作品
- `D4_CREATE_REWRITE`｜创建重写版本
- `D5_REWRITE_RESULT`｜重写结果

UI 以 `ui/frames/<FRAME_ID>/` 内 Stitch 冻结稿为视觉基线。实现时允许组件化重构，但不得改变页面业务含义、字段、状态和入口。

## 5. API 范围

| 方法 | 路径 / 契约 | 用途 |
|---|---|---|
| POST | /api/v1/content-items/{contentItemId}/rewrites/mock | 创建模拟重写任务 |
| GET | /api/v1/workflow-runs/{workflowRunId} | 查询模拟任务 |
| GET | /api/v1/content-items/{contentItemId}/versions | 版本列表 |
| GET | /api/v1/content-versions/{versionId} | 版本详情 |
| GET | /api/v1/projects/{projectId}/works | 项目作品列表 |
| GET | /api/v1/works/{workId} | 项目作品聚合详情 |

所有 HTTP API 均使用 `/api/v1` 前缀和统一响应：

```json
{
  "data": {},
  "request_id": "req_xxx"
}
```

写接口必须明确校验、状态变化、事务边界和幂等策略。

## 6. 数据模型

- `RewriteRequest`
- `WorkflowRun`
- `WorkflowRunStatus`
- `ContentVersion`
- `ProjectWorkReadModel`

详细字段与关系见本目录 `data-model.md`，全局定义见 `../../baselines/p0-data-model-catalog.md`。

## 07.1A 冻结契约增量

本子轮只冻结 `POST /api/v1/content-items/{contentItemId}/rewrites/mock`（`mockRewriteContentItem`）和 `GET /api/v1/workflow-runs/{workflowRunId}`（`getWorkflowRun`）。来源必须为 frozen v1 及同版本 completed ReviewReport，`expected_version` 是 v1 乐观锁。成功原子创建 `source=mock_rewrite` 的 editable v2 和 succeeded `content_mock_rewrite` WorkflowRun；v1 继续 frozen，`current_version_id` 不变，v2 不自动审核或发布。相同 key/payload 返回原结果，同键异 payload 与 stale version 均为 409；失败只保留安全的 failed WorkflowRun，无部分 v2。D4 Cancel/Close 不发请求，D5 只展示新 v2，不把它设为当前。

## 07.1B 冻结版本查询契约

`listContentItemVersions` 按 `version_no DESC, id DESC` 返回 v2、v1 和共享分页字段；`is_current` 只来自 `ContentItem.current_version_id`。`getContentVersion` 返回指定 versionId 的完整历史快照，不以当前版本替换。两个读取接口均含 nullable 来源 v1、ReviewReport、rewrite WorkflowRun 摘要，按项目隔离且不包含 ProjectWork。

## 07.1C 冻结项目作品查询契约

`work_id` 稳定映射到 `ContentItem.id`，ProjectWorkReadModel 仅聚合既有数据而不持久化。列表使用共享分页和 `chapter_plan.chapter_no ASC, content_item.id ASC`；详情复用 07.1B 版本历史。当前版本只用 `current_version_id`，并返回 version_count、稳定的最近审核/运行摘要以及 D1/D2/D4/D5 导航 ID。

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
| I07-AC01 | 创建 v2 保留 v1 | 重写成功后 versions 数量增加 1，v1 内容和记录仍存在。 |
| I07-AC02 | 不自动切换当前版本 | 生成 v2 后 current_version_id 仍保持原值，除非用户后续明确选择。 |
| I07-AC03 | 不自动审核与发布 | v2 初始 review_status=not_submitted，publish_status=not_published。 |
| I07-AC04 | 取消无副作用 | D4 取消时不创建 WorkflowRun 或 ContentVersion。 |
| I07-AC05 | 项目作品聚合 | D3 可显示章节、当前版本、审核状态和可用版本数量。 |
| I07-AC06 | 运行可追踪 | WorkflowRun 输入、输出摘要、开始/结束时间和错误信息可查询。 |

## 9. 自动化测试要求

- Domain：状态机、值对象、校验规则单元测试。
- Application：成功、失败、幂等和事务回滚分支。
- Repository：Testcontainers PostgreSQL 集成测试。
- HTTP：OpenAPI 契约测试、错误码和 request_id。
- Web：组件测试、API Mock 测试、关键页面状态测试。
- E2E：至少覆盖本迭代闭环链路，不能仅验证页面可打开。

## 10. 明确排除

- 版本差异专页
- 自动设为当前版本
- 真实 AI 重写
- 发布流程
- 版本删除

## 11. 完成定义

- 本迭代所有核心需求均有可执行验收用例。
- UI、API、数据模型和状态机无未审批漂移。
- 迁移可在空库执行，测试数据可重复创建。
- Go test、Web typecheck/lint、契约测试和本迭代 E2E 通过。
- 文档、实现报告和差异说明已更新。
- Git 工作区干净，并完成单次迭代提交。
