# Iteration 14 — n8n 适配与 WorkflowRun 异步运行

## 当前状态

`ready_for_contract_freeze`。本文件是 Iteration 14 的开发输入；尚未开始 CF-14-01，Iteration 14 不得标记为已完成。

## 目标与前置依赖

在 Iteration 12 的连接与工作流配置 CRUD、以及 Iteration 13 的项目工作流绑定前置依赖基础上，建设 n8n Adapter 与 WorkflowRun 异步运行闭环：

1. 验证、启用和停用 WorkflowConnection；
2. 验证、启用和停用 WorkflowConfiguration；
3. 创建 WorkflowRun、入队、Worker 执行、状态机、事件、失败、取消与重试；
4. 以不含密钥的配置和绑定快照执行 n8n Webhook，并校验输出；
5. 从项目业务页触发运行，并在流程中心查看全局运行记录、详情和项目筛选结果。

当前只实现 n8n Adapter。Worker 通过 HTTP Webhook 请求并等待响应；不建设 callback server。

## 产品与 UI 基线

- 流程中心没有“工作流配置”标签页，也没有其他标签页，直接展示全局运行记录；唯一列表路由为 `/workflow-runs`。
- 运行详情路由为 `/workflow-runs/{runId}`，面包屑为“流程中心 / 运行记录 / 运行编号”。项目名称仅是详情字段和可点击链接，不是父级路由；不新增项目级重复运行列表路由。
- 项目概览只提供总运行次数、运行中、最近失败、最近运行、最近运行列表、查看全部运行记录与查看环节运行记录。
- 项目侧入口分别跳转至 `/workflow-runs?projectId={projectId}` 与 `/workflow-runs?projectId={projectId}&stage={stage}`。
- P14_01～P14_10 是人工验收 PASS 的 UI 基线；其中已绑定且可运行时打开 P14_08 确认弹窗，未绑定时打开 P14_09 提示弹窗，P14_09 不是独立页面。创建成功后直接进入运行详情。

## 实施顺序

1. **契约冻结**：CF-14-01 冻结 API、状态机、错误结构、快照和并发/幂等规则。
2. **后端开发**：实现 Adapter、验证和启停、持久化、队列、Worker、事件、状态机、取消与重试。
3. **前端和 UI**：在既有产品壳层接入 P14_01～P14_10 所定义的列表、详情、弹窗和项目摘要入口。
4. **真实 API 联调**：以真实 n8n、PostgreSQL、Worker 与浏览器完成联调和验收。

## 不在范围

- 分发平台绑定、验证、OAuth、分发执行和发布；
- LLM Provider 验证、模型发现和直接调用；
- 其他 Adapter（包括 Coze、ComfyUI）；
- n8n 可视化编辑器、多实例智能路由和 callback server；
- Iteration 15 内容。
