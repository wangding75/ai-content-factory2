# P0 需求追踪规则

## 1. 追踪链

```text
FR / BR / NFR
→ Iteration
→ API endpoint
→ Domain entity/use case
→ Migration
→ UI Frame
→ Automated acceptance case
→ Git commit
```

## 2. 已有矩阵

位于：

```text
docs/development-inputs/p0/matrices/
├── acceptance-traceability.csv
├── api-to-iteration.csv
├── iteration-scope.csv
├── model-to-iteration.csv
└── page-to-iteration.csv
```

## 3. 变更要求

新增、删除或改变核心需求时，必须同时更新：

1. 产品或业务文档。
2. OpenAPI / 数据模型 / 状态机。
3. 对应迭代计划与验收。
4. 追踪矩阵。
5. 实现与测试。

任何一个环节缺失都视为需求链路未闭合。
