# Iteration 17 开发计划

**状态：`frozen_cf_17_01`。** Iteration 17 固定为以下四个顶层任务，不再拆分 A/B/C 中间任务。

| 顺序 | 任务 | 名称 | 固定产出 / 前置 |
|---|---|---|---|
| 1 | CF-17-01 | 契约与数据基础冻结 | 业务、Runtime 输入输出、OpenAPI、Migration 17、事务、Summary、P0 兼容与 8 Frame 追踪；本任务完成后形成唯一冻结基线。 |
| 2 | CF-17-02 | 后端开发 | Domain、Repository、Preflight、Run 创建、Summary、输出消费、Issue 处置与 Retry；仅依赖 CF-17-01 最终 Commit。 |
| 3 | CF-17-03 | 前端开发 | 编辑器入口、审核抽屉、运行/失败状态、报告、全文定位和历史；依赖 CF-17-02。 |
| 4 | CF-17-04 | 真实 n8n 联调与最终收口 | 正常链路、异常/安全/P0 回归、浏览器验收与最终 Review；依赖 CF-17-03。 |

执行顺序固定为契约与数据基础 → 后端 → 前端 → 真实 n8n 联调与最终收口。Iteration 18 在 CF-17-04 PASS 前不得开始。
