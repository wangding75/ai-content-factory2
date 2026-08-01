# Iteration 19 — LLM 与工作流真实接入及第二用户闭环关闭

**状态：`execution_plan_frozen_ready_for_task_02`。** 新 UI、迭代级 API Scope、逻辑数据模型、状态机、安全边界和执行计划已冻结。Task 01 已完成；后续从 Task 02 主 OpenAPI 契约开始开发。

## 1. 基线

- 分支：`feature/second-user-loop`。
- UI 与迭代契约冻结 Commit：`6e8656467f3c204feb83d19185d10f78e092cd56`。
- 执行计划修正 Commit：`39fa4f3c4838c5d67b98c7958ba2f7a42ece75b7`。
- Iteration 12～18 已完成配置记录、项目绑定、WorkflowRun、章节规划、正文生成、审核和重写能力。
- Iteration 14.5 已建立本地 n8n Connection Verify、Workflow Verify、绑定和 Preflight 基线；Iteration 19 在现有架构上扩展，不重建平行模块。
- 当前目标：真实 LLM Provider、真实 n8n Runtime、Workflow Configuration LLM 策略、实时执行资格和四 Stage 完整闭环。

## 2. 产品决策冻结

1. WorkflowRun 详情保持 `/workflow-runs/{runId}` 独立页面。
2. `/workflows` 本轮保留为内置 Mock 流程只读页，并引导到 `/workflow-runs`；不合并、不删除路由。
3. 重试默认使用当前有效配置；原配置只有在快照、指纹和外部引用完整且可重放时才启用。
4. Provider、Connection、Workflow Configuration 修改关键字段后：`enabled` 和项目绑定保留，验证状态转为 `stale`，执行资格立即失效；重新验证成功后恢复。
5. LLM 策略固定在 Workflow Configuration 层，项目不得覆盖 Provider、模型或策略。
6. 四个 Stage 共用绑定抽屉、运行前置检查和异常恢复组件，仅业务输入、契约摘要和结果名称不同。
7. 默认模型从模型目录消失时不自动切换，配置进入不可执行状态。
8. Workflow Configuration 关键字段变更更新同一记录并执行 `version + 1`；不建设配置历史表。历史运行事实由 WorkflowRun 不可变快照承担。
9. Result consumption failure 只使用 Stage 专用消费重试，不重新调用 n8n/LLM，不创建新的 Runtime Run。

## 3. UI 冻结范围

正式开发输入为 `ui-manifest.json` 中 15 个 Frame：

- 01～06：LLM Provider、n8n Connection、Workflow Configuration；
- 07～09：项目四环节绑定；
- 10～12：流程中心、独立运行详情、重试确认；
- 13：旧 `/workflows` 的定位引导；
- 14～15：四阶段共享 Preflight 阻断与运行恢复状态。

AppShell、一级导航和 Iteration 15～18 主体布局保持不变。详见 `ui-review.md`、`ui-scope.md` 和 `prototype-source-mapping.md`。

## 4. 最终用户闭环

```text
配置 LLM Provider
→ 获取/校验模型并验证
→ 启用 Provider
→ 配置并验证 n8n Connection
→ 配置 Workflow Configuration
→ 固定 LLM 策略、输入/输出契约和 Stage
→ 验证并启用 Workflow Configuration
→ 项目绑定四个可执行 Stage
→ 真实章节规划
→ 真实正文生成
→ 固定版本真实审核
→ 选择问题真实重写
→ 比较并采用新版本
→ 流程中心追踪、取消、诊断和重试
→ 第二用户闭环关闭
```

## 5. 执行资格

```text
enabled = true
+ current version verified
+ connection executable
+ workflow reference and contracts valid
+ LLM strategy complete
+ ACF-managed Provider/model executable when applicable
= executable = true
```

`executable` 为服务端派生事实，不接受客户端写入，也不单独持久化。绑定存在不等于当前可执行。

## 6. 任务计划

Task 01 文档修正已完成。当前只剩以下 8 个开发任务：

| 任务 | 名称 | 核心输出 | 前置任务 |
|---|---|---|---|
| Task 02 | 主 OpenAPI 契约同步 | 完整 Operation、Schema、错误码、枚举和生成类型 | Task 01 |
| Task 03 | Migration 19 与持久化模型 | 向前 Migration、兼容迁移、Repository、数据库门禁 | Task 02 |
| Task 04 | 公共集成基础 | 统一验证状态机、执行资格、安全外呼、凭据保护 | Task 03 |
| Task 05 | 集成配置与项目绑定闭环 | LLM、n8n、Workflow Configuration、四 Stage 绑定 | Task 04 |
| Task 06 | Workflow Runtime 与失败恢复 | 快照、真实 n8n 执行、生命周期、取消、超时、重试 | Task 05 |
| Task 07 | Iteration 19 完整前端 | 15 Frame、真实 API、共享状态、独立 Run 详情页 | Task 06 |
| Task 08 | 四 Stage 真实 Runtime 接入 | 章节规划、正文、审核、重写依次移除最终 Mock 路径 | Task 07 |
| Task 09 | 真实联调与最终验收 | 真实 LLM/n8n E2E、安全、数据库、UI、Review、关闭迭代 | Task 08 |

完整边界、依赖、阶段顺序和里程碑见 `execution-plan.md`。具体执行拆分属于可变执行资料，不构成本目录的冻结事实源；执行资料不得反向修改或覆盖本目录中的业务契约、数据模型、API Scope、UI Scope 和验收标准。

## 7. 任务拆分原则

1. 一个任务只形成一个可验收业务闭环。
2. OpenAPI、Migration、公共基础、Runtime、前端和最终联调保持层级隔离。
3. LLM Provider、n8n Connection、Workflow Configuration 和 Binding 同属配置闭环，在 Task 05 内按依赖顺序实现。
4. WorkflowRun 创建、快照、真实执行、取消、超时和重试属于同一状态机，在 Task 06 内完成。
5. 15 个 Frame 共用类型、状态和组件，在 Task 07 内统一实现，避免前端重复拆改。
6. 四个 Stage 在 Task 08 内依次完成，但每个 Stage 必须单独验证后才进入下一 Stage。
7. 最终真实凭据、确定性 n8n Workflow 和完整 E2E 只在 Task 09 使用。

## 8. 不在范围

- Coze、ComfyUI 或其他工作流平台；
- n8n 可视化编辑器；
- 多 n8n 实例自动路由；
- 自动模型路由、成本大盘和计费优化；
- 项目级 Provider/模型覆盖；
- 自动审核—重写循环、自动 Set Current 或发布；
- 删除或合并 `/workflows`；
- 用 Mock Adapter 作为最终闭环验收；
- 密钥版本历史库或 Workflow Configuration 历史表。

## 9. 开发准备完成定义

- [x] 新 UI 已评审并归档；
- [x] 产品决策和强制开发修正已冻结；
- [x] 迭代级 API Scope 已完整定义；
- [x] 逻辑数据模型、状态机、快照和安全边界已定义；
- [x] UI/API/数据模型追踪已建立；
- [x] 8 个剩余开发阶段、依赖和迭代级门禁已冻结；
- [ ] 主 OpenAPI 已更新并通过生成类型验证；
- [ ] Migration 19 已实现并通过数据库门禁；
- [ ] 后端、前端和四 Stage 真实接入完成；
- [ ] 真实 n8n、真实 LLM 和四 Stage E2E 通过。
