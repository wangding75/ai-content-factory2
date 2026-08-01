# Iteration 19 — 开发准备说明

当前状态为 `READY_FOR_TASK_02_OPENAPI`：

1. UI 无 P0 阻断，15 Frame 已冻结；
2. 迭代级 API Scope、逻辑数据模型、状态机和安全边界已冻结；
3. Task 01 文档修正已在 Commit `39fa4f3c4838c5d67b98c7958ba2f7a42ece75b7` 完成；
4. Task 02～09 最新任务书已归档到 [`taskbooks/README.md`](taskbooks/README.md)，每份任务书包含逐文件修改逻辑、详细验收标准、验证门禁和回执要求；
5. Workflow Configuration 使用同一记录 `version + 1`，不创建配置历史表；
6. 主 OpenAPI 尚未同步，因此第一个开发任务必须是 Task 02，不能跳到 Migration、Adapter、Runtime 或前端；
7. Task 03 必须在 Task 02 主 OpenAPI 和字段枚举冻结后执行；
8. Task 04～09 必须严格按 `execution-plan.md` 和任务书依赖顺序推进；
9. 真实外部凭据、确定性 n8n Workflow 和 LLM Model 由 Task 09 最终联调提供，不阻塞 Task 02～08 的本地实现与自动测试。

下一任务：[Task 02 — 主 OpenAPI 契约同步](taskbooks/CF-19-02-main-openapi-contract.md)。
