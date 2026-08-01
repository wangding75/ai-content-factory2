# Iteration 19 — Task 02～09 任务书索引

- 任务书版本：`v2`
- 执行计划来源 Commit：`39fa4f3c4838c5d67b98c7958ba2f7a42ece75b7`
- 适用分支：`feature/second-user-loop`
- 使用规则：严格按顺序执行；每次只下发当前任务书；前一任务 PASS 并推送后才能进入下一任务。

| 顺序 | 任务书 | 任务名称 | 推荐模型 | 固定 Commit |
|---:|---|---|---|---|
| 02 | [CF-19-02-main-openapi-contract.md](CF-19-02-main-openapi-contract.md) | 主 OpenAPI 契约同步 | GPT-5.6 Sol / high | `feat: freeze iteration 19 api contracts` |
| 03 | [CF-19-03-migration19-persistence.md](CF-19-03-migration19-persistence.md) | Migration 19 与持久化模型 | GPT-5.6 Sol / high | `feat: add iteration 19 persistence model` |
| 04 | [CF-19-04-integration-foundation.md](CF-19-04-integration-foundation.md) | 公共集成基础 | GPT-5.6 Sol / high | `feat: add integration validation foundation` |
| 05 | [CF-19-05-integration-config-binding.md](CF-19-05-integration-config-binding.md) | 集成配置与项目绑定闭环 | GPT-5.6 Terra / medium | `feat: complete integration configuration lifecycle` |
| 06 | [CF-19-06-workflow-runtime-recovery.md](CF-19-06-workflow-runtime-recovery.md) | Workflow Runtime 与失败恢复 | GPT-5.6 Sol / high | `feat: complete workflow runtime recovery` |
| 07 | [CF-19-07-frontend.md](CF-19-07-frontend.md) | Iteration 19 完整前端 | GPT-5.6 Terra / medium | `feat: implement iteration 19 integration ui` |
| 08 | [CF-19-08-four-stage-runtime.md](CF-19-08-four-stage-runtime.md) | 四 Stage 真实 Runtime 接入 | GPT-5.6 Sol / high | `feat: connect four stages to real workflow runtime` |
| 09 | [CF-19-09-e2e-acceptance.md](CF-19-09-e2e-acceptance.md) | 真实联调与最终验收 | GPT-5.6 Sol / high | `test: close iteration 19 real integration loop` |

## 验收规则

- 每份任务书均包含独立“验收标准”和“验证要求”。
- 验收标准定义必须达到的业务、契约、数据、安全和 UI 结果；验证要求定义证明方式。
- 任一验收标准未满足时不得回执 PASS。
- 不允许仅用测试退出码为 0 替代业务结果验收。
- 任务书不得修改冻结业务契约，不得扩大当前任务授权范围。

## 上位文档

- [Iteration 计划](../iteration-plan.md)
- [执行计划](../execution-plan.md)
- [开发准备说明](../development-readiness.md)
- [迭代验收标准](../acceptance.md)
- [API Scope](../api-scope.yaml)
