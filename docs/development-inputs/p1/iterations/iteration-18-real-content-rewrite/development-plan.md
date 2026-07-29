# Iteration 18 开发计划

**状态：`frozen_cf_18_01a`。** 当前任务只完成 CF-18-01A；不得自动开始后续任务或扩大各任务验收范围。

| 顺序 | 任务 | 名称 | 固定产出 / 前置 |
|---:|---|---|---|
| 1 | CF-18-01A | 业务与 API 契约冻结 | 业务闭环、版本/Issue、rewrite.input/output、OpenAPI、Summary、Retry、Set Current、错误语义和 9 Frame 追踪。 |
| 2 | CF-18-01B | 数据与 Migration 契约冻结 | 数据模型、事务、锁、索引与最小向前 Migration 设计；仅依赖 01A，不在 01A 执行。 |
| 3 | CF-18-02 | 后端开发 | Domain、Repository、Preflight、Run、Summary、输出消费、Retry 与 Set Current 复用；依赖 01B。 |
| 4 | CF-18-03 | 前端开发 | 审核入口、创建、配置抽屉、运行/失败、结果和确认弹窗；依赖 02。 |
| 5 | CF-18-04 | 真实 n8n 联调与最终收口 | 正常/异常链路、兼容回归、浏览器原型验收和最终 Review；依赖 03。 |

执行顺序固定为业务/API 契约 → 数据/Migration 契约 → 后端 → 前端 → 真实联调收口。后续任务不得重新解释 CF-18-01A 的 Stage、输入输出、API、状态与错误语义。
