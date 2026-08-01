# Iteration 19 — UI Scope（最终冻结）

**UI 状态：`FROZEN_WITH_DEVELOPMENT_CORRECTIONS`。** 正式开发输入为 15 个 Frame；不新增路由，不改变 AppShell 和一级导航，不整体重做 Iteration 15～18。

## 1. Frame 清单

| 顺序 | Frame | 区域/形态 | 路由 | 冻结用途 |
|---:|---|---|---|---|
| 01 | `I19_01_LLM_PROVIDER_LIST` | 主页面 | `/settings?tab=llm` | Provider 列表、模型、验证、启用与执行资格 |
| 02 | `I19_02_LLM_PROVIDER_DRAWER` | 抽屉 | 同上 | 保存凭据、模型发现/校验、默认模型、保存并验证 |
| 03 | `I19_03_N8N_CONNECTION_LIST` | 主页面 | `/settings?tab=workflows&subtab=connections` | Connection 验证、启用、执行资格与依赖数量 |
| 04 | `I19_04_N8N_CONNECTION_DRAWER` | 抽屉 | 同上 | 实例根 URL、凭据、验证和依赖影响 |
| 05 | `I19_05_WORKFLOW_CONFIGURATION_LIST` | 主页面 | `/settings?tab=workflows&subtab=workflows` | Stage、LLM 策略、验证、启用、版本和执行资格 |
| 06 | `I19_06_WORKFLOW_CONFIGURATION_DRAWER` | 抽屉 | 同上 | 引用/契约/默认参数、三种 LLM 策略和分层验证 |
| 07 | `I19_07_PROJECT_BINDINGS_EXECUTABLE` | 主页面状态 | `/projects/{projectId}/settings?tab=workflow-bindings` | 四环节全部可执行 |
| 08 | `I19_08_PROJECT_BINDINGS_DEPENDENCY_INVALID` | 主页面状态 | 同上 | 绑定保留但依赖失效或配置已变更 |
| 09 | `I19_09_SELECT_WORKFLOW_DRAWER` | 共享抽屉 | 同上 | 四 Stage 候选、不可执行原因和换绑摘要 |
| 10 | `I19_10_WORKFLOW_RUN_LIST` | 主页面 | `/workflow-runs` | 真实运行列表、状态、筛选、取消/重试入口 |
| 11 | `I19_11_WORKFLOW_RUN_DETAIL_PAGE` | 独立页面 | `/workflow-runs/{runId}` | 时间线、错误诊断、安全快照、领域影响和恢复 |
| 12 | `I19_12_RETRY_CONFIRM_DIALOG` | 弹窗 | 详情页内 | 当前配置与条件原配置重试 |
| 13 | `I19_13_LEGACY_WORKFLOWS_GUIDANCE` | 主页面微调 | `/workflows` | 内置 Mock 只读说明和真实流程中心引导 |
| 14 | `I19_14_SHARED_PREFLIGHT_BLOCKED` | 共享状态 | 四阶段发起抽屉 | 绑定/Connection/Workflow/LLM/契约检查 |
| 15 | `I19_15_SHARED_RUNTIME_RECOVERY` | 共享状态 | 四阶段业务页 | 取消中、超时、输出校验失败、结果消费失败恢复 |

每个 Frame 路径：`ui/frames/<FRAME_ID>/screen.png` 与 `code.html`。

## 2. 页面范围

- 6 个主要页面/路由组：01、03、05、07/08（同路由状态）、10、11；
- 5 个抽屉/弹窗：02、04、06、09、12；
- 1 个旧页面定位微调：13；
- 2 个共享状态：14、15。

统计仅用于设计交付，不代表新增路由数量。新增路由为 0。

## 3. 统一状态词表

配置：未验证、验证中、验证成功、验证失败、配置已变更、已启用、已停用、可执行、不可执行。

运行：排队中、运行中、取消中、已取消、执行成功、执行失败、已超时、输出校验失败、结果消费失败。

恢复：可重试、不可重试、重试提交结果、使用当前配置重试、使用原配置重试（条件可用）。

## 4. 交互规则

1. 保存、验证、启用分离。
2. 配置关键字段变化保留 enabled 和 Binding，但立即不可执行。
3. 项目只绑定可执行 Workflow Configuration，不覆盖 LLM 配置。
4. 不可执行候选保留可见性和修复入口，但不可勾选。
5. 业务发起必须先显示/执行共享 Preflight；失败不创建 Run。
6. WorkflowRun 详情为独立页面，不使用旧详情抽屉形态。
7. 原配置重试按 retry-options 返回结果启用或禁用。
8. Result consumption failure 使用“重试提交结果”，不再次调用 Runtime。
9. `/workflows` 保留只读，不用于外部执行。

## 5. 强制开发修正

详见 `ui-review.md` 第 4 节。关键项：现有 AppShell、中文 locale、列表高级筛选、独立详情页、敏感信息脱敏、抽屉滚动、13 页头修正、消费 Retry 文案。

## 6. 不变范围

- AppShell、顶部栏、一级导航；
- 章节规划、正文生成、内容审核、正文重写的主体布局和核心业务操作；
- Iteration 15～18 已冻结领域规则；
- `/workflows` 路由和导航入口；
- 项目四 Stage 的唯一绑定语义。
