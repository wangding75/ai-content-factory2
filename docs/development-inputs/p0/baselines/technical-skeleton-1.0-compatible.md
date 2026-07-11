# 技术骨架基线｜与 1.0 保持一致

## 1. 技术选型

| 层 | 技术 |
|---|---|
| 后端 | Go / Golang |
| 前端 | TypeScript + React + Next.js |
| 数据库 | PostgreSQL |
| 缓存 / 异步 | Redis + Asynq |
| 对象存储 | S3 / MinIO 抽象；P0 可使用本地或测试实现 |
| 契约 | OpenAPI + JSON Schema |
| 测试 | Go test、Testcontainers、Web unit test、Playwright |
| 部署 | Docker Compose 起步 |

## 2. Monorepo

```text
ai-content-factory/
├── apps/
│   ├── api/
│   └── web/
├── packages/
│   ├── contracts/
│   ├── shared-types/
│   └── eslint-config/
├── docs/
│   ├── architecture/
│   ├── product/
│   ├── api/
│   └── decisions/
├── deployments/
│   ├── docker/
│   ├── k8s/
│   └── compose/
├── scripts/
├── tasks/
├── tests/
│   ├── e2e/
│   └── fixtures/
├── .ai-dev/
├── Makefile
├── docker-compose.yml
└── README.md
```

## 3. 后端分层

```text
interfaces
  ├── http
  ├── webhook
  └── worker
application
  ├── commands
  ├── queries
  └── use cases
domain
  ├── entities
  ├── value objects
  ├── services
  └── repository interfaces
infrastructure
  ├── postgres
  ├── redis
  ├── objectstore
  └── telemetry
plugins
  ├── contentpacks/novel
  └── workflowproviders/mock
```

依赖方向：

```text
interfaces → application → domain
infrastructure → domain interfaces
plugins → extension contracts
domain 不依赖 infrastructure
```

## 4. P0 模块

```text
project
material
storyline
foreshadowing
chapterplan
content
review
workflow
works
capability
audit
```

## 5. 强制边界

1. Handler 不写业务逻辑，不直接访问 Repository。
2. API DTO 不直接作为 Domain Entity。
3. Novel 差异由 Novel Pack 承载。
4. 模拟生成、模拟审核、模拟重写统一通过 Mock Provider。
5. 所有核心接口有 OpenAPI；所有响应包含 request_id。
6. 所有状态变化写 AuditLog。
7. 所有写接口明确事务和幂等策略。
8. 前端页面不得散落 `if contentType == novel`；通过 pack/feature 配置收敛。
9. P0 不实现真实 AI、外部工作流和发布适配。
10. 每个迭代结束必须执行核心需求 E2E，而非只做冒烟。
