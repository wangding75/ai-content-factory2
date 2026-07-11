# 验收方案

| 用例 ID | 场景 | 通过标准 |
|---|---|---|
| I01-AC01 | 一键启动 | docker compose 启动 API:8080、Web:3000、PostgreSQL、Redis。 |
| I01-AC02 | 健康检查 | /healthz 返回 200；/readyz 在依赖就绪时返回 200。 |
| I01-AC03 | 数据库迁移 | 空库可正向迁移到最新版本；测试库可重复创建。 |
| I01-AC04 | 前端骨架 | S00_HOME 可访问，全局导航和统一 API Client 已接入。 |
| I01-AC05 | 质量门禁 | Go test、Web lint/typecheck、契约校验、Playwright 基础用例通过。 |

## 门禁

- 核心用例必须可重复执行。
- 不接受仅页面打开或 HTTP 200 的冒烟结果。
- 失败分支必须验证数据库无脏数据。
