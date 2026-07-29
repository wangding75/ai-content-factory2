# Iteration 18 开发计划

**状态：`rebuild_candidate_2026_07_29`。** Iteration 18 采用四个顶层任务；CF-18-01 冻结后不得随意拆分或扩大验收范围。

| 顺序 | 任务 | 名称 | 固定产出 / 前置 |
|---:|---|---|---|
| 1 | CF-18-01 | 契约与数据基础冻结 | 业务、rewrite.input/output、OpenAPI、最小向前 Migration、事务、Summary、P0 兼容和 9 Frame 追踪。 |
| 2 | CF-18-02 | 后端开发 | Preflight、Run 创建、Summary、输出消费、IssueLink、Retry 与 set-current 复用；依赖 CF-18-01。 |
| 3 | CF-18-03 | 前端开发 | 审核入口、创建页、配置抽屉、运行/失败、候选结果、确认弹窗；依赖 CF-18-02。 |
| 4 | CF-18-04 | 真实 n8n 联调与最终收口 | 正常/异常链路、P0 回归、浏览器原型验收和最终 Review；依赖 CF-18-03。 |

Iteration 17 冻结修复完成前不得开始 CF-18-01 的代码或契约修改；本次临时任务仅替换文档与原型。
