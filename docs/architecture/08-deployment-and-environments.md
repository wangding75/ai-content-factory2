# 部署与环境

## 1. Local

```text
Windows / WSL2
Docker Desktop
web: 3000
api: 8080
postgres: 5432
redis: 6379
```

## 2. Test

- 独立 PostgreSQL 测试容器。
- Redis 测试实例。
- 固定 Mock Provider seed。
- Playwright 启动完整 Web + API。

## 3. Future Production

P0 不交付生产部署，但需保持以下可迁移性：

- Web、API、Worker 独立容器。
- 配置来自环境变量或 Secret。
- 数据库迁移单独执行。
- Worker 可水平扩展。
- 对象存储通过抽象接口接入。

## 4. Docker Compose 规则

- 服务必须有健康检查。
- API 在依赖 ready 后启动。
- 数据卷命名稳定。
- 不在 Compose 文件硬编码真实密钥。
