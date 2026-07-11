
# AI Content Factory 2.0｜架构与脚手架规范索引

本目录补充 P0 开发迭代计划，作为业务、产品、技术与工程实现的共同基线。

## 文档

| 文档 | 作用 |
|---|---|
| `01-business-architecture.md` | 业务目标、价值链、业务能力、领域边界和核心规则 |
| `02-product-architecture.md` | 产品信息架构、功能架构、用户旅程、页面与状态 |
| `03-technical-architecture.md` | 系统架构、模块边界、数据流、Provider 与部署结构 |
| `04-scaffold-directory-standard.md` | 与 1.0 一致的 Monorepo 脚手架和目录规范 |

## 约束优先级

```text
P0 业务规则
→ 已冻结 API / 数据模型 / 状态机
→ 已冻结 UI 与页面链路
→ 技术架构
→ 目录与编码规范
→ 各迭代实现计划
```

发生冲突时，不允许开发者自行选择。必须先更新契约、追踪矩阵和变更记录，再修改实现。

## P0 固定边界

- 只实现小说内容包 `novel`。
- 只执行内置模拟能力 `mock`。
- 真实 AI 暂未配置。
- n8n、Coze、ComfyUI 暂未开放。
- 发布平台暂未开放。
- 项目是核心业务容器。
- Material 是全局唯一素材本体，项目通过 ProjectMaterialUsage 建立用途关系。
- 审核不覆盖正文。
- 重写创建新版本，保留旧版本。
- 新版本不自动成为当前版本、不自动审核、不自动发布。
