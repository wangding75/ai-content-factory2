# AI Content Factory 2.0｜P0 开发迭代计划包

本包用于将已经冻结的 P0 业务链路、页面链路、UI、API 和领域模型拆成可独立开发、可验证、可追踪的纵向迭代。

## 技术骨架

完全沿用 AI Content Factory 1.0：

- Monorepo
- Go 模块化单体 API
- TypeScript + React + Next.js Web
- PostgreSQL
- Redis + Asynq
- OpenAPI + JSON Schema
- Go test + Testcontainers + Playwright
- Docker Compose
- `apps/api`、`apps/web`、`packages/contracts`、`tests/e2e`、`docs`、`scripts`
- `.ai-dev` 迭代状态与每轮一次 Git commit 的管理方式

## P0 实现边界

- 内容类型：只实现 Novel Pack。
- 生成能力：只实现内置 Mock Provider。
- 真实 AI：暂未配置，不执行。
- n8n / Coze / ComfyUI：暂未开放，不执行。
- 发布平台：暂未开放，不执行。
- UI：以 Stitch 冻结 Frame 为基线。
- 开发方式：每个迭代同时覆盖 UI、API、领域、数据库和验收，不按前后端横向拆分。

## 目录

```text
baselines/      全局技术、链路、API、模型、状态机和验收基线
iterations/     Iteration 00–09 的独立开发包
matrices/       页面/API/模型/验收追踪矩阵
tasks/          可直接放入仓库 tasks/ 的迭代任务文档
source/         原始 UI 包摘要与校验信息
```

## 推荐执行顺序

```text
Iteration 00 契约冻结
→ Iteration 01 技术骨架
→ Iteration 02 项目创建
→ Iteration 03 策划与素材
→ Iteration 04 故事线与伏笔
→ Iteration 05 章节规划
→ Iteration 06 正文与审核
→ Iteration 07 重写与项目作品
→ Iteration 08 全局 Lite 页面
→ Iteration 09 全链路验收
```

每轮先审核 `iteration-plan.md`，再开发；验收通过后再进入下一轮。


## 新增架构文档

```text
architecture/
├── 00-architecture-index.md
├── 01-business-architecture.md
├── 02-product-architecture.md
├── 03-technical-architecture.md
├── 04-scaffold-directory-standard.md
└── architecture-manifest.json
```

这些文档与 Iteration 00–09 开发计划共同构成 P0 开发输入。
