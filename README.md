# AI Content Factory 2.0

AI Content Factory 2.0 是面向长篇内容生产的模块化单体应用。当前分支为 `feature/second-user-loop`，已完成 Iteration 15～18 的业务实现及浏览器 UI 验收；Iteration 19 的最终全链路联调与验收尚未执行，第二用户闭环尚未关闭。

## 当前进度

| 迭代 | 当前状态 | 主要能力 |
|---|---|---|
| Iteration 15 | 实现完成，浏览器 UI 验收通过 | 章节规划预检、真实 WorkflowRun、候选结果与 Adopt |
| Iteration 16 | 实现完成，浏览器 UI 验收通过 | 正文生成、候选版本、当前版本切换与结果消费恢复 |
| Iteration 17 | 实现完成，浏览器 UI 验收通过 | 内容审核、审核报告、问题列表与审核状态 |
| Iteration 18 | 实现完成，浏览器 UI 验收通过 | 基于审核结果的重写、候选版本与重写历史 |
| Iteration 19 | 尚未开始最终验收 | n8n、LLM、前后端和数据库的最终全链路联调与关闭 |

当前已经具备 ACF WorkflowRun、n8n Webhook 调用框架以及章节规划、正文生成、审核、重写的业务实现。仓库中的浏览器验收报告只证明 Iteration 15～18 的页面和 UI 链路通过，不代表真实 LLM 端到端闭环已经完成。

## 能力边界

### 已实现

- 项目、素材、故事线、章节规划和作品管理。
- 全局执行配置与项目工作流绑定。
- WorkflowRun 运行记录和流程中心。
- 章节规划、正文生成、内容审核和内容重写业务链路。
- OpenAPI、JSON Schema、PostgreSQL Migration、Go API 和 Next.js Web。
- Iteration 15～18 浏览器 UI 对比验收。

### 尚未完成

- 真实 LLM Provider 的端到端调用闭环。
- 四阶段真实 n8n 工作流的最终统一联调。
- Iteration 19 异常恢复、最终一致性和关闭验收。
- 生产发布级认证、授权和完整持续集成门禁。

确定性本地 n8n 测试工作流用于验证 ACF 与工作流运行框架，不能等同于真实 AI 内容生成。

## 技术栈

- Go 模块化单体 API 与后台 Worker
- Next.js App Router Web
- PostgreSQL
- OpenAPI 与 JSON Schema
- Docker Compose
- Playwright
- n8n 工作流编排

## 数据库冻结规则

ACF 只使用一个数据库：`ai_content_factory`。

- 所有迭代在现有数据库上顺序执行增量 Migration。
- 已提交的历史 Migration 和 SQL 文件不得修改、重排、合并或重新生成。
- 数据库结构变化只能通过新增 Migration 完成。
- 日常测试与验收不得创建开发库、测试库、迭代库或验收库。
- 不以清空数据库、Migration Down 或重新初始化数据库作为常规验收方式。
- 验收关注当前数据库的最终结构、历史数据可读性、幂等性和领域关系一致性。

## 本地运行

### Docker Compose（推荐）

```bash
docker compose up -d --build
```

服务地址：

- Web：`http://localhost:13001`
- API：`http://localhost:18080`
- PostgreSQL：`localhost:15433`，数据库 `ai_content_factory`

查看状态和日志：

```bash
docker compose ps
docker compose logs -f --tail=200
```

停止服务：

```bash
docker compose down
```

正常开发和验收不得执行 `docker compose down -v`。该命令会删除唯一数据库的持久化 Volume 和全部数据。

### 可选 n8n 环境

```bash
docker compose -f compose.n8n.yml up -d
```

n8n 地址：`http://localhost:15678`。n8n 环境只提供工作流运行基础设施；工作流导入、激活和真实 LLM 连接仍需按对应迭代文档配置。

## 开发命令

安装依赖：

```bash
pnpm install
cd apps/api && go mod download
```

前端：

```bash
pnpm dev:web
pnpm test:web
pnpm typecheck:web
pnpm lint:web
pnpm build:web
```

后端：

```bash
cd apps/api
go test ./...
go run ./cmd/api
```

`Makefile` 中现有 `check-contracts` 仍是占位目标，不能把 `make verify` 视为完整项目总门禁。契约和迭代校验应使用仓库中实际存在的专项脚本。

## 目录

- `apps/api`：Go API、Worker、Repository 和 Migration
- `apps/web`：Next.js Web
- `packages/contracts`：OpenAPI 与 JSON Schema
- `infra/n8n`：本地 n8n 工作流和相关制品
- `docs/development-inputs/p1`：第二用户闭环冻结输入和迭代文档
- `docs/testing`：测试策略、质量门禁和 E2E 基线
- `report/iteration-15-18-browser-acceptance`：Iteration 15～18 浏览器 UI 验收证据

## 当前状态说明

项目当前完成到 Iteration 18。Iteration 19 尚未执行，第二用户闭环尚未正式关闭，仓库当前状态不能解释为完整生产发布就绪。
