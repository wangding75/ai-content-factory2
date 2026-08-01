# Iteration 19 — 开发准备说明

文档手动更新并提交后，状态为 `READY_FOR_TASK_02_OPENAPI`：

1. UI 无 P0 阻断，15 Frame 已冻结；
2. 迭代级 API Scope、逻辑数据模型、状态机、安全边界和 8 个剩余开发任务已冻结；
3. Workflow Configuration 使用同一记录 `version + 1`，不创建配置历史表；
4. 主 OpenAPI 尚未同步，因此第一个开发任务必须是 Task 02，不能跳到 Migration、Adapter、Runtime 或前端；
5. Task 03 必须在 Task 02 主 OpenAPI 和字段枚举冻结后执行；
6. Task 04～08 必须严格按 `execution-plan.md` 的依赖顺序推进；
7. 真实外部凭据、确定性 n8n Workflow 和 LLM Model 由 Task 09 最终联调提供，不阻塞 Task 02～08 的本地实现与自动测试。

下一任务：`Task 02 — 主 OpenAPI 契约同步`。
