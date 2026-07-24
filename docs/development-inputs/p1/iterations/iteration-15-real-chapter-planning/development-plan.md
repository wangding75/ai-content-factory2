# Iteration 15 开发计划

**状态：`confirmed_14_tasks`。** 本计划只有 14 个执行任务，固定顺序为契约 → 后端 → 前端 → 人工 UI 验收 → 联调收口。不得保留或恢复旧 22 任务计划。

## 1. 契约（3 个）

| 任务 | 名称 | 前置 |
|---|---|---|
| CF-15-01A | 业务规则与状态机冻结 | Iteration 14 completed |
| CF-15-01B | 数据模型、索引与事务冻结 | 01A；已冻结实体/索引/Migration 终态与事务矩阵，不创建 Migration |
| CF-15-01C | OpenAPI、Schema 与错误契约冻结 | 01A、01B |

## 2. 后端（5 个）

| 任务 | 名称 | 前置 |
|---|---|---|
| CF-15-02A | Candidate Batch、Candidate、Revision 数据层 | 01B、01C |
| CF-15-02B | 预检与章节规划 Run 编排 | 02A |
| CF-15-02C | 规范化结果校验与原子 Batch 消费 | 02A、02B |
| CF-15-02D | Candidate 查询、编辑、比较与 Recompare | 02A、02C |
| CF-15-02E | Adopt、Discard、Abandon、统计与后端收口 | 02C、02D |

## 3. 前端（4 个）

| 任务 | 名称 | 前置 |
|---|---|---|
| CF-15-03A | 章节规划工作区、生成设置与预检 | 02B、02E |
| CF-15-03B | 候选批次列表与详情 | 02D、02E |
| CF-15-03C | 候选编辑、比较、Recompare、Adopt 与 Abandon | 03B、02E |
| CF-15-03D | 故事线只读统计、统一状态与前端回归 | 03A、03C |

`03D` 完成后统一进行人工 UI 验收；人工 UI PASS 前不得开始 04A。

## 4. 联调收口（2 个）

| 任务 | 名称 | 前置 |
|---|---|---|
| CF-15-04A | 真实 API 联调与候选闭环 | 02E、03D、人工 UI PASS、CF-14-N8N-Integration PASS |
| CF-15-04B | 最终 E2E、安全、状态与 Git 收口 | 04A |

Iteration 15 不实现 n8n Transport、Worker 或 Callback；真实执行通路只在既有 `CF-14-N8N-Integration` PASS 后作为 04A 的依赖进行联调。
