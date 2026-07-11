# AI Content Factory 2.0｜产品文档索引

## 1. 文档目标

本目录给产品、设计、研发、测试和执行 Agent 提供统一的产品上下文。任何迭代不得只读取局部任务文档而忽略本目录。

## 2. 文档清单

| 文档 | 作用 |
|---|---|
| `01-product-overview.md` | 产品定位、目标用户、核心问题和价值主张 |
| `02-p0-prd.md` | P0 完整 PRD、功能范围、流程与验收目标 |
| `03-user-personas-and-scenarios.md` | 用户角色、核心场景和任务模型 |
| `04-information-architecture.md` | 信息架构、全局导航、项目工作区导航 |
| `05-page-and-interaction-spec.md` | 页面注册表、交互规则、页面状态和路由语义 |
| `06-roadmap-and-scope.md` | P0/P1/P2 边界、明确不做事项和演进路线 |

## 3. 需求优先级

```text
业务规则与状态机
→ P0 PRD
→ API / 数据模型契约
→ 冻结 UI Frame 与页面链路
→ 迭代计划与验收用例
→ 实现代码
```

发生冲突时必须先提交变更说明并同步修改追踪矩阵，不允许在代码中默默改变产品语义。
