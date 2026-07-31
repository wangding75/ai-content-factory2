# Iteration 19 — 开发准备说明

文档与原型已达到 `READY_FOR_CF_19_01A`：

1. UI 无 P0 阻断，15 Frame 冻结；
2. 迭代级 API Scope、数据模型和状态机完整；
3. 主 OpenAPI 尚未同步，因此第一个开发任务必须是 CF-19-01A，不能直接进入 Adapter 或前端实现；
4. Migration 19 必须在 OpenAPI/模型字段冻结后单独执行；
5. 真实外部凭据、n8n Workflow 和 LLM Model 由最终联调阶段提供，不影响先完成契约和代码骨架。
