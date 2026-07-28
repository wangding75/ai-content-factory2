# 本地 n8n 基础环境

## 来源与版本

本环境参考 n8n 官方 [`n8n-hosting` 的 `docker-compose/withPostgres`](https://github.com/n8n-io/n8n-hosting/tree/main/docker-compose/withPostgres) 配置中的官方镜像、持久化目录和健康检查方式；本仓库的配置不是该示例的直接副本。

使用固定镜像版本：`docker.n8n.io/n8nio/n8n:1.104.2`。不使用 `latest`，以保证本地环境可复现。

## 与 ACF Compose 的关系

`compose.n8n.yml` 仅新增 `n8n` 服务。它与根目录的 `compose.yml` 合并启动，继承 Compose 项目 `ai-content-factory2` 的默认网络，因此同一 Docker 网络内的服务可通过 `http://n8n:5678` 访问 n8n。

本任务没有采用官方示例中的 PostgreSQL、`init-data.sh` 或 runner 服务：ACF 已有 PostgreSQL 服务和持久化卷，但本任务不为 n8n 新增第二套 PostgreSQL，也不创建独立 ACF 数据库。n8n 使用其默认 SQLite 数据库，数据与自动生成的实例加密密钥一同持久化在独立卷中。

## 使用方式

启动 n8n：

```powershell
docker compose -f compose.yml -f compose.n8n.yml up -d n8n
```

停止 n8n：

```powershell
docker compose -f compose.yml -f compose.n8n.yml stop n8n
```

n8n UI：<http://127.0.0.1:15678>

健康检查端点：<http://127.0.0.1:15678/healthz>

持久化 Volume 名称为 `ai-content-factory2_n8n_storage`（Compose 中的逻辑卷名为 `n8n_storage`）。

## 当前任务边界

本任务只部署 n8n 基础环境。

不包含工作流导入、Integration Verify、项目绑定、Preflight、Create Run 或 Iteration 15 联调；也不连接 ACF Integration。
