# Iteration 19 — LLM 与工作流真实接入及第二用户闭环关闭

**状态：`ui_and_iteration_contract_frozen`。** 新 UI 已评审并冻结；迭代级 API、数据模型、状态机和安全边界已定义。主 OpenAPI、Migration 和代码尚未实施，开发必须先同步单一契约源再进入后端实现。

## 1. 基线

- 分支基线：`feature/second-user-loop`。
- 文档更新前 HEAD：`235917f9d74c1263093e06ec0629d71dfed9ee02`。
- Iteration 12～18 已完成配置记录、项目绑定、WorkflowRun、章节规划、正文生成、审核和重写能力。
- Iteration 14.5 已建立本地 n8n 连接、Workflow Verify、绑定和 Preflight 的真实基线；Iteration 19 不否定或重建该基线。
- 当前开发目标：真实 LLM Provider、真实 n8n Runtime、明确 LLM 策略和四 Stage 完整闭环。

## 2. 产品决策冻结

1. WorkflowRun 详情保持 `/workflow-runs/{runId}` 独立页面。
2. `/workflows` 本轮保留为内置 Mock 流程只读页，并引导到 `/workflow-runs`；不合并、不删除导航。
3. 重试默认使用当前有效配置；原配置只有在完整、安全、可重放时才启用。
4. Provider、Connection、Workflow Configuration 修改关键字段后：`enabled` 和项目绑定保留，验证状态转为 `stale`，执行资格立即失效；重新验证成功后自动恢复。
5. LLM 策略固定在 Workflow Configuration 层，项目不得覆盖 Provider、模型或策略。
6. 四个 Stage 共用绑定抽屉、运行前置检查和异常恢复组件，仅业务输入、契约摘要和结果名称不同。
7. 默认模型从模型目录消失时不自动切换，配置进入不可执行状态。

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

`executable` 为服务端派生事实，不单独持久化。绑定存在不等于当前可执行。

## 6. 开发阶段

| 阶段 | 任务 | 输出 |
|---|---|---|
| CF-19-01A | 主 OpenAPI 与生成类型同步 | Operation、Schema、错误码、状态枚举唯一 |
| CF-19-01B | 数据模型与 Migration 19 | 最小向前 Migration、约束、索引、兼容迁移 |
| CF-19-02A | LLM Provider Adapter | 模型发现、验证、启停、安全调用 |
| CF-19-02B | n8n Connection/Workflow Adapter | 连接验证、工作流验证、真实调用、取消/状态同步 |
| CF-19-02C | WorkflowRun 扩展 | cancelling/timed_out、失败阶段、快照、重试资格 |
| CF-19-03A | 全局配置前端 | 01～06 Frame |
| CF-19-03B | 项目绑定与 Preflight | 07～09、14 Frame |
| CF-19-03C | 流程中心与恢复 | 10～13、15 Frame |
| CF-19-04A | 四 Stage 真实接入 | 章节规划、正文生成、审核、重写移除最终 Mock 路径 |
| CF-19-04B | 真实全链路联调 | 唯一数据库、真实 n8n、真实 LLM、真实 API |
| CF-19-05 | 验收与 Review | E2E、安全、P0 回归、人工验收、独立 Code Review |

任务必须按依赖拆小执行；不得在一个任务中同时修改 OpenAPI、Migration、四个 Stage 和全部前端。

## 7. 不在范围

- Coze、ComfyUI 或其他工作流平台；
- n8n 可视化编辑器；
- 多 n8n 实例自动路由；
- 自动模型路由、成本大盘和计费优化；
- 项目级 Provider/模型覆盖；
- 自动审核—重写循环、自动 Set Current 或发布；
- 删除或合并 `/workflows`；
- 用 Mock Adapter 作为最终闭环验收；
- 密钥版本历史库；原配置不能安全重放时必须禁用该选项。

## 8. 开发准备完成定义

- [x] 新 UI 已评审并归档；
- [x] 产品决策和强制开发修正已冻结；
- [x] 迭代级 API Scope 已完整定义；
- [x] 逻辑数据模型、状态机、快照和安全边界已定义；
- [x] UI/API/数据模型追踪已建立；
- [ ] 主 OpenAPI 已更新并通过生成类型验证；
- [ ] Migration 19 已设计、实现并通过数据库门禁；
- [ ] Iteration 19 后端与前端开发完成；
- [ ] 真实 n8n、真实 LLM、四 Stage E2E 通过。
