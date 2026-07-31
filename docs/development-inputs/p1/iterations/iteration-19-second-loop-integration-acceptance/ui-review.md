# Iteration 19 — 新 UI 评审与冻结结论

**状态：`FROZEN_WITH_DEVELOPMENT_CORRECTIONS`。** 评审对象为 `stitch_acf_iteration_19_ui_final_freeze_rebuild`。本轮确认 15 个正式 Frame；`04_n8n_2` 为重复候选，不进入开发清单。

## 1. 总体结论

新 UI 已满足 Iteration 19 的核心产品闭环：

```text
配置可验证
→ 工作流可执行
→ 项目绑定可判断
→ 业务入口可阻断
→ 运行可追踪
→ 失败可诊断
→ 重试可选择有效配置
```

未发现需要重新调用 Stitch 的 P0 阻断问题。AppShell、一级导航和 Iteration 15～18 主体布局不变。原型可以冻结并进入契约/开发阶段，但生产实现必须完成第 4 节的开发修正。

## 2. 此前 6 个产品决策核对

| 决策 | UI 证据 | 结论 |
|---|---|---|
| WorkflowRun 保持 `/workflow-runs/{runId}` 独立详情页 | `I19_11_WORKFLOW_RUN_DETAIL_PAGE` | PASS；不改为抽屉 |
| `/workflows` 保留只读并引导真实流程中心 | `I19_13_LEGACY_WORKFLOWS_GUIDANCE` | PASS；本轮不合并、不删除路由 |
| 重试默认使用当前有效配置；原配置仅在可重放时可用 | `I19_11`、`I19_12` | PASS；原配置不可重放时禁用并解释原因 |
| 修改关键配置后保留 enabled 与绑定，但执行资格失效 | `I19_01`～`I19_08` | PASS；重新验证成功后自动恢复 |
| LLM 策略固定在 Workflow Configuration 层，项目不可覆盖 | `I19_06`、`I19_07`、`I19_09` | PASS |
| 四个 Stage 共用一套绑定抽屉与状态组件 | `I19_07`、`I19_09`、`I19_14`、`I19_15` | PASS |

补充规则“默认模型从模型目录消失后不得自动切换”未单独制作异常页，但 `I19_02` 已提供模型目录、默认模型和校验结构；文档冻结为 `model_unavailable` 验证失败原因，结构足以承载，不要求重画。

## 3. 页面级评审

| Frame | 结论 | 说明 |
|---|---|---|
| `I19_01_LLM_PROVIDER_LIST` | PASS | 明确区分验证、启用和执行资格，覆盖模型数量、默认模型、最近验证 |
| `I19_02_LLM_PROVIDER_DRAWER` | PASS | 覆盖密钥安全状态、模型发现、手动模型校验、保存并验证；抽屉必须可滚动 |
| `I19_03_N8N_CONNECTION_LIST` | PASS | 覆盖连接健康、启用状态、依赖数量与最近验证 |
| `I19_04_N8N_CONNECTION_DRAWER` | PASS | 选择 `04_n8n_1`；Base URL 保持实例根地址，Workflow 引用归 Workflow Configuration |
| `I19_05_WORKFLOW_CONFIGURATION_LIST` | PASS | LLM 策略、验证、启用、执行资格和版本完整 |
| `I19_06_WORKFLOW_CONFIGURATION_DRAWER` | PASS | 三种 LLM 策略和分层验证清单完整；长抽屉使用滚动区和固定底栏 |
| `I19_07_PROJECT_BINDINGS_EXECUTABLE` | PASS | 明确“已绑定且可执行”，不允许项目覆盖 Provider/模型 |
| `I19_08_PROJECT_BINDINGS_DEPENDENCY_INVALID` | PASS | 绑定、启用和历史运行保留，失效链与精准修复入口完整 |
| `I19_09_SELECT_WORKFLOW_DRAWER` | PASS | 可执行候选可选，不可执行候选可查看原因但禁止选择；四 Stage 共享结构 |
| `I19_10_WORKFLOW_RUN_LIST` | PASS_WITH_CORRECTION | 能力完整，但筛选和表格密度较高；生产实现采用“主筛选 + 高级筛选”，避免常驻九个筛选器和横向溢出 |
| `I19_11_WORKFLOW_RUN_DETAIL_PAGE` | PASS_WITH_CORRECTION | 独立详情页形态正确；生产只保留一个主要重试入口，长页可使用 sticky action，避免静态重复按钮 |
| `I19_12_RETRY_CONFIRM_DIALOG` | PASS | 双模式重试有可用性判断、retryOf 和幂等说明 |
| `I19_13_LEGACY_WORKFLOWS_GUIDANCE` | PASS_WITH_CORRECTION | 保留只读与引导；生产页头不得带 Run breadcrumb，必须使用 `/workflows` 自身上下文 |
| `I19_14_SHARED_PREFLIGHT_BLOCKED` | PASS | 四阶段共享依赖检查、阻断原因和修复入口完整 |
| `I19_15_SHARED_RUNTIME_RECOVERY` | PASS_WITH_CORRECTION | 恢复结构完整；“专用重试”生产文案统一为“重试提交结果”或对应领域结果名称 |

## 4. 强制开发修正

1. 生产必须复用仓库现有 AppShell、路由和设计 Token，不复制 Stitch 壳层。
2. 默认中文环境下，Home/Projects/Assets/Works/Workflows/Settings、Global Settings、READ ONLY 等用户可见英文全部进入 locale 并显示中文；技术标识可保留英文。
3. `I19_10` 的 Connection、Provider、模型、配置版本、可重试性放入“高级筛选”；常驻筛选保留项目、Stage、状态、时间和搜索。
4. `I19_10` 表格必须保证操作入口和 `retryOf` 完整可见；禁止靠截断隐藏关键关系。
5. `I19_11` 维持独立详情页；旧抽屉 Frame 不再作为页面形态约束。
6. `I19_11` 不展示密钥、Credential、Authorization Header、完整 URL 查询参数、原始上游响应或内部堆栈。
7. `I19_13` 页头和面包屑使用“内置流程”，不得出现具体 Run ID。
8. `I19_02`、`I19_04`、`I19_06` 抽屉使用独立滚动区和 sticky footer；当前截图下方内容不视为缺失。
9. 默认模型消失时进入 `model_unavailable`，不自动选择其他模型；用户重新选择并验证后恢复执行资格。
10. “结果消费失败”的恢复按钮统一为“重试提交结果”，不得让用户误以为会再次调用 n8n/LLM。
11. 配置列表中的 `enabled=true` 与 `executable=false` 必须可同时表达。
12. 04 的正式原型只使用 `04_n8n_1`；`04_n8n_2` 不进入 Manifest。

## 5. 范围结论

- 核心配置、绑定和流程追踪：12 个设计对象已覆盖。
- 补充设计：旧 `/workflows` 引导、共享 Preflight、共享运行恢复，共 3 个 Frame。
- 正式 Frame：15 个。
- 新增路由：0。
- 导航变化：0。
- AppShell 变化：0。
- Iteration 15～18 整页重做：0。
- 四阶段只复用 `I19_14`、`I19_15` 的增量状态，不重画业务主体。
