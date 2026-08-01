# Iteration 19 — 执行计划

**状态：`frozen`。** Task 01 已在 Commit `39fa4f3c4838c5d67b98c7958ba2f7a42ece75b7` 完成文档歧义修正和计划落盘。Task 02～09 必须按本文件的依赖顺序执行。

## 1. 全局执行规则

1. 单一事实源优先级：主 OpenAPI → Migration/Schema → 后端领域实现 → 前端类型与 UI。
2. 每个任务一个主 Commit；不得把后续任务提前混入当前 Commit。
3. 每个任务先读取 `AGENTS.md`、适用目录规则和本迭代冻结文档。
4. 先局部验证，再分组验证，最后执行当前任务完整门禁；禁止未定位根因前反复跑总门禁。
5. 最终状态只允许 PASS 或真正不可恢复的 BLOCKED。
6. 普通编译、测试、格式、SQL、路径、暂存和网络问题必须自行修复到 PASS。
7. 不修改历史 Migration，不创建平行 Provider/Connection/Workflow/Binding/Run 架构。
8. 四 Stage 最终正式路径不得继续调用 Mock Adapter。

## 2. 剩余任务总览

| 顺序 | 任务 | 主要模块 | 固定 Commit message | 完成后里程碑 |
|---:|---|---|---|---|
| 02 | 主 OpenAPI 契约同步 | `packages/contracts/openapi`、契约验证/生成 | `feat: freeze iteration 19 api contracts` | HTTP 契约冻结 |
| 03 | Migration 19 与持久化模型 | migrations、Repository、Fixture、dbcheck | `feat: add iteration 19 persistence model` | 数据基础完成 |
| 04 | 公共集成基础 | globalconfig 公共状态、安全 HTTP、Secret、执行资格 | `feat: add integration validation foundation` | 公共基础完成 |
| 05 | 集成配置与项目绑定闭环 | LLM、n8n、Workflow Configuration、workflowbinding | `feat: complete integration configuration lifecycle` | 配置可执行 |
| 06 | Workflow Runtime 与失败恢复 | workflowrun、n8n executor、快照、取消、超时、重试 | `feat: complete workflow runtime recovery` | Runtime 可运行 |
| 07 | Iteration 19 完整前端 | global-config、bindings、workflow-runs、共享状态 | `feat: implement iteration 19 integration ui` | 15 Frame 实现 |
| 08 | 四 Stage 真实 Runtime 接入 | chapterplan、contentitem、review、rewrite | `feat: connect four stages to real workflow runtime` | 第二闭环代码完成 |
| 09 | 真实联调与最终验收 | compose/n8n、E2E、安全、数据库、UI、Review | `test: close iteration 19 real integration loop` | Iteration 19 关闭 |

## 3. 依赖图

```text
Task 02 OpenAPI
→ Task 03 Migration/Repository
→ Task 04 公共状态与安全基础
→ Task 05 配置与绑定闭环
→ Task 06 Runtime 与恢复
→ Task 07 完整前端
→ Task 08 四 Stage 真实接入
→ Task 09 真实联调与关闭
```

不得并行跨过未完成的上游契约或 Schema。本文件只维护迭代级阶段顺序、范围边界和里程碑；具体执行拆分属于可变执行资料，不构成冻结事实源。

## 4. Task 02 门禁

- 主 OpenAPI 覆盖冻结 `api-scope.yaml`；
- 既有 Path、operationId、字段和成功响应兼容；
- 生成类型与 OpenAPI 无漂移；
- 敏感字段 writeOnly/不回显；
- 不修改数据库、后端业务和前端页面。

## 5. Task 03 门禁

- 新增且只新增 Migration 19；
- 历史 Migration 校验和不变；
- 兼容迁移 `not_connected → unverified`；
- Repository、Fixture、Schema、Data consistency PASS；
- Workflow Configuration 不新增历史表。

## 6. Task 04 门禁

- 统一验证状态转换和版本保护；
- `executable` 由服务端派生并返回精准原因；
- URL、DNS、重定向、TLS、超时、响应大小和日志脱敏具备测试；
- PATCH Secret/Credential 保留语义明确；
- 不实现具体 Provider/n8n 业务闭环。

## 7. Task 05 门禁

- LLM Provider 模型发现、验证、启停完成；
- n8n Connection 验证、启停和依赖影响完成；
- Workflow Configuration 三种策略和分层验证完成；
- Binding 区分 bound/executable，失效不解绑；
- Provider→Connection→Workflow→Binding 联合测试 PASS。

## 8. Task 06 门禁

- Run 创建时锁定 Binding 并冻结安全快照；
- n8n Runtime 真实提交、查询、取消和超时完成；
- 状态、事件、失败阶段和领域影响稳定；
- 当前配置/条件原配置重试完整；
- Result consumption retry 仍走 Stage 专用接口。

## 9. Task 07 门禁

- 15 Frame 对应 UI 全部实现；
- 不改变 AppShell、导航和现有路由；
- WorkflowRun 详情保持独立页面；
- 抽屉滚动、固定底栏、高级筛选和脱敏符合 `ui-review.md`；
- typecheck、lint、unit、build PASS。

## 10. Task 08 门禁

- 章节规划、正文生成、内容审核、正文重写依次接入统一 Runtime；
- 每个 Stage 单独通过 Preflight、快照、输出校验、原子消费、重试和幂等测试；
- 正式入口不调用 Mock Adapter；
- Iteration 15～18 领域规则和专用消费重试保持不变。

## 11. Task 09 门禁

- 使用真实 OpenAI-compatible Provider 和真实 n8n；
- 四 Stage 成功链路与失败矩阵通过；
- SSRF、DNS、重定向、TLS、超时、凭据与日志安全通过；
- Migration History、Schema、Data consistency 通过；
- 15 Frame 浏览器人工验收通过；
- 独立全量 Code Review 和缺陷修复完成；
- `.ai-dev/state.json`、报告和最终状态更新，工作区 clean。
