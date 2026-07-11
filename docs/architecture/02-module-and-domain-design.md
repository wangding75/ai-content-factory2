# 模块与领域设计

## 1. 后端模块

```text
project
material
narrative
chapterplan
content
review
workflow
works
capability
audit
```

## 2. 分层

```text
interfaces → application → domain
infrastructure → domain ports
plugins → extension contracts
```

禁止：

- Handler 直接写 SQL。
- Domain import HTTP、PostgreSQL、Redis 或 Provider 包。
- Application 依赖具体 Postgres Repository。
- 跨模块直接复用数据库 Row。

## 3. 模块职责

| 模块 | 负责 | 不负责 |
|---|---|---|
| project | 项目身份、状态、策划、工作区 | 素材本体、正文版本 |
| material | Material 与项目 Usage | 项目生命周期 |
| narrative | PlotLine、Foreshadowing | 章节正文 |
| chapterplan | 章节候选、编辑、确认 | 正文版本 |
| content | ContentItem、ContentVersion | 审核评分逻辑 |
| review | ReviewReport、Finding、Recommendation | 修改正文 |
| workflow | WorkflowRun 和 Provider 调度 | 领域真值替代 |
| works | 项目与全局聚合读模型 | 重复业务真值 |
| capability | 能力和集成状态 | 伪造连接 |
| audit | 审计记录 | 业务流程编排 |

## 4. Web 分层建议

```text
app/routes
widgets
features
entities
shared/api
shared/ui
```

页面负责组合 Feature，不直接拼接未定义 DTO；API Client 由契约生成或严格映射。
