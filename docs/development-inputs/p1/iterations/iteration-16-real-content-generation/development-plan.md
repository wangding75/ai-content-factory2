# Iteration 16 开发计划

**状态：`review_ready_13_tasks`。** 固定顺序为契约 → 后端 → 前端 → 人工 UI 验收 → 真实联调。不得在契约冻结前实现生产代码。

## 1. 契约（3 个）

| 任务 | 名称 | 输出 |
|---|---|---|
| CF-16-01A | 业务规则、状态机和相邻迭代边界冻结 | `business-rules.md`、`closed-loop.md`、`acceptance.md` |
| CF-16-01B | 数据模型、索引、Migration 终态和事务冻结 | `data-model.md`、`transaction-and-migration-design.md` |
| CF-16-01C | OpenAPI、错误码、UI/API 追踪和原型冻结 | 主 OpenAPI、`api-scope.yaml`、UI 文档和 11 个 Frame |

## 2. 后端（4 个）

| 任务 | 名称 | 前置 |
|---|---|---|
| CF-16-02A | Runtime subject、ContentVersion 来源字段与 Migration | 01B、01C |
| CF-16-02B | 预检、Run 创建、Summary 和活跃锁 | 02A |
| CF-16-02C | 输出 Schema、原子候选消费、Event 和消费重试 | 02A、02B |
| CF-16-02D | 版本查询兼容、设为当前版本和后端收口 | 02C |

## 3. 前端（3 个）

| 任务 | 名称 | 前置 |
|---|---|---|
| CF-16-03A | 正文编辑器三栏、上下文和生成抽屉 | 01C、02B |
| CF-16-03B | queued/running/succeeded/failed/not-configured 状态 | 03A、02C |
| CF-16-03C | 版本选择、比较、设为当前、中文文案与前端回归 | 03B、02D |

03C 后统一进行人工 UI 验收；人工 UI PASS 前不得开始 04A。

## 4. 联调收口（2 个）

| 任务 | 名称 | 前置 |
|---|---|---|
| CF-16-04A | 真实 n8n API 联调和候选版本闭环 | 02D、03C、人工 UI PASS、既有执行适配层 PASS |
| CF-16-04B | 最终 E2E、安全、P0 回归、文档和 Git 收口 | 04A |

## 5. 执行约束

- 每个任务只修改授权范围；
- 失败先局部定位和验证，不反复完整重跑；
- 后端严格按冻结 OpenAPI；
- 前端严格按 11 个具体 PNG 和正式路由；
- 不使用静态 HTML、截图背景或生产 Demo 参数；
- 不提前进入 Iteration 17。
