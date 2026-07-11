# AI Content Factory 2.0｜系统架构

## 1. 技术栈

- Web：TypeScript、React、Next.js App Router。
- API：Go 模块化单体。
- 数据库：PostgreSQL。
- 缓存与任务：Redis、Asynq 边界。
- 契约：OpenAPI 3.1、JSON Schema。
- 测试：Go test、Testcontainers、Web 单测、Playwright。
- 部署：Docker Compose 起步。

## 2. 系统上下文

```text
Creator
→ Next.js Web
→ Go API
→ PostgreSQL
→ Redis / Worker
→ Mock Workflow Provider
```

P0 没有真实外部依赖。LLM、n8n、Coze、ComfyUI 和发布平台都只存在于未来适配边界。

## 3. 部署单元

```text
web
api
worker
postgres
redis
```

API 和 Worker 共享 Go module，但必须使用不同 cmd 入口和进程。

## 4. 关键架构决策

- 使用模块化单体，不做微服务拆分。
- 使用契约优先，API 和 Schema 先于实现。
- 使用全局 Material + 项目 Usage，而非复制素材。
- 使用不可覆盖 ContentVersion。
- 使用 Mock Provider 驱动完整业务状态，不在业务代码散落模拟数据。
