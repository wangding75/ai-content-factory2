# Iteration 17 开发计划

**状态：`rebuild_candidate_2026_07_29`。** 固定为 9 个开发任务，避免在实现后补规则导致返工。

| 顺序 | 任务 | 名称 | 固定产出 / 前置 |
|---|---|---|---|
| 1 | CF-17-01A | 业务、数据、事务与输出契约冻结 | 本目录业务规则、模型、事务、状态机；不写实现。 |
| 2 | CF-17-01B | OpenAPI、UI 与追踪冻结 | 正式 OpenAPI、api-scope、8 Frame 追踪；依赖 01A。 |
| 3 | CF-17-02A | Migration、Domain 与 Repository | 只新增向前 Migration；扩展 Run/Report/Finding；依赖 01A/01B。 |
| 4 | CF-17-02B | Preflight、Run 创建与 Summary | 固定版本、Token、幂等、active Run 和八状态；依赖 02A。 |
| 5 | CF-17-02C | 输出消费、Report/Issue 与处置 | 原子消费、消费 Retry、历史与忽略；依赖 02A/02B。 |
| 6 | CF-17-03A | 编辑器入口与发起审核抽屉 | 01/02 Frame，预检和创建；依赖 01B/02B。 |
| 7 | CF-17-03B | 运行、结果、全文定位、失败和历史 | 03–08 Frame，刷新恢复与前端回归；依赖 02C/03A。 |
| 8 | CF-17-04A | 真实 n8n 正常闭环 | review.input/output.v1、Report/Issue、历史；依赖 02C/03B。 |
| 9 | CF-17-04B | 异常、安全、P0 回归和最终 Review | 输出非法、消费失败、重试、并发、安全、浏览器和 Git；依赖 04A。 |

执行顺序固定为契约、后端、前端、真实联调、最终收口。Iteration 18 在 CF-17-04B PASS 前不得开始。
