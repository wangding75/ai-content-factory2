# Iteration 18 开发计划

**状态：`frozen_cf_18_01b`。** CF-18-01A 与 CF-18-01B 已按顺序完成冻结；不得自动开始 CF-18-02A 或扩大各任务验收范围。

| 顺序 | 任务 | 名称 | 固定产出 / 前置 |
|---:|---|---|---|
| 1 | CF-18-01A | 业务与 API 契约冻结 | 模型：GPT-5.6 Sol；推理：high；状态：已完成；业务闭环、版本/Issue、rewrite.input/output、OpenAPI、Summary、Retry、Set Current、错误语义和 9 Frame 追踪。 |
| 2 | CF-18-01B | 数据模型、事务与 Migration 契约冻结 | 模型：GPT-5.6 Sol；推理：high；状态：已完成；数据模型、事务、锁、索引与最小向前 Migration 18；仅依赖 01A。 |
| 3 | CF-18-02A | 后端 Preflight、Token、创建 Rewrite Run 与输入快照 | 模型：GPT-5.6 Sol；推理：high；依赖 01B。 |
| 4 | CF-18-02B | 后端输出校验、Candidate 原子消费与消费失败 | 模型：GPT-5.6 Sol；推理：high；依赖 02A。 |
| 5 | CF-18-02C | 后端 Summary、Runtime Retry、消费 Retry、Set Current 与历史 | 模型：GPT-5.6 Sol；推理：high；依赖 02B。 |
| 6 | CF-18-03A | 前端审核入口、Availability、Preflight 与创建重写 | 模型：GPT-5.6 Terra；推理：medium；依赖 02C。 |
| 7 | CF-18-03B | 前端 queued/running、失败恢复、结果和 Candidate 展示 | 模型：GPT-5.6 Terra；推理：medium；依赖 03A。 |
| 8 | CF-18-03C | 前端 Set Current、CAS 冲突、刷新恢复与历史 | 模型：GPT-5.6 Terra；推理：medium；依赖 03B。 |
| 9 | CF-18-04A | Iteration 18 全量代码 Review | 模型：GPT-5.6 Sol；推理：high；只做 Review，不修改代码；问题确认后另行下发修复任务。 |
| 10 | CF-18-04B | Iteration 18 前后端功能联调与工程冻结 | 模型：GPT-5.6 Sol；推理：high；不包含浏览器截图比对和视觉验收。 |

执行顺序固定为上述十项任务。后续任务不得重新解释 CF-18-01A 的 Stage、输入输出、API、状态与错误语义。
