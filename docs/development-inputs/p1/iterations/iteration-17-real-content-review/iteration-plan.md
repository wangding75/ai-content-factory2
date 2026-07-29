# Iteration 17 — 真实内容审核

**状态：`rebuild_candidate_2026_07_29`。** 本目录用于替换原 Iteration 17 开发输入。业务、数据、事务、API、UI 和原型已经重新对齐；正式实现前仍需按 `development-plan.md` 完成 OpenAPI 与 Migration 冻结。

## 1. 迭代目标

针对一个明确、已保存的 `ContentVersion` 发起真实异步审核，通过项目绑定的 `review` 工作流生成结构化 `ReviewReport` 与 `ReviewIssue`，并完整保留来源版本、WorkflowRun、证据、定位和建议。

审核不得修改正文内容，也不得自动创建重写版本。用户可以离开页面；返回或刷新后，页面必须从持久化的 Run、Event、Report 和 Issue 恢复状态。

## 2. 完整用户闭环

```text
正文编辑器确认已保存版本
→ 打开发起审核抽屉
→ 选择固定 ContentVersion
→ 只读预检并确认项目审核工作流
→ 创建 WorkflowRun(stage=review, subject=content_version)
→ queued / running
→ 校验 review.output.v1
→ 原子创建 ReviewReport + ReviewIssue
→ review_ready
→ 查看问题列表、证据、全文定位和历史报告
→ 可标记问题为忽略
→ Iteration 18 才可基于选中问题创建重写任务
```

## 3. 核心业务边界

- Runtime Stage 固定为 `review`，与 Iteration 13 的项目工作流绑定枚举一致；不得使用 `content_review` 新建第二套 Stage。
- 审核来源固定为 `ContentVersion.id`。提交后正文 ID、乐观锁版本和内容摘要必须保持可追溯。
- 同一版本允许多次历史审核，但同一版本最多一个 `queued/running` 审核 Run。
- 每个成功 Run 最多生成一个 ReviewReport；失败 Run 不得留下部分 Report 或 Issue。
- ReviewReport 不覆盖历史报告；历史页同时展示成功、失败和运行中的 Run。
- 问题“忽略”只改变用户处置状态，不修改审核输出、原始证据或正文。
- “创建重写任务”属于 Iteration 18；本迭代原型只保留禁用的交接入口，不调用重写 API。
- P0 Mock Review API 与既有 Mock 数据继续保留，不作为真实审核成功依据。

## 4. 复用关系

| 来源迭代 | 直接复用 |
|---|---|
| Iteration 06 | ReviewReport、ReviewFinding、ReviewRecommendation、审核历史和详情读取语义 |
| Iteration 12 | n8n Connection、凭据保护与 Verify |
| Iteration 13 | ProjectWorkflowBinding(stage=`review`) |
| Iteration 14 | WorkflowRun、WorkflowRunEvent、Runtime Retry、运行详情和安全错误 |
| Iteration 15 | 项目绑定与异步状态恢复模式 |
| Iteration 16 | Preflight Token、命令幂等、输出校验失败/消费失败分离、Summary 聚合模式 |
| Iteration 18 | 只提供 ReviewReport/Issue 来源；不提前执行重写 |

## 5. API 范围

- `POST /api/v1/content-versions/{contentVersionId}/review-runs/preflight`
- `POST /api/v1/content-versions/{contentVersionId}/review-runs`
- `GET /api/v1/content-items/{contentItemId}/review-summary`
- 复用 `GET /api/v1/content-items/{contentItemId}/reviews`
- 复用 `GET /api/v1/reviews/{reviewId}`
- `PATCH /api/v1/reviews/{reviewId}/issues/{issueId}`
- `POST /api/v1/workflow-runs/{workflowRunId}/review-result-consumption-retries`
- 复用 WorkflowRun 详情、Event、Cancel 与 Runtime Retry API

完整冻结范围见 `api-scope.yaml`。

## 6. UI 与原型

Iteration 17 采用 8 张按用户链路排序的桌面原型：

1. `I17_D1_EDITOR_REVIEW_ENTRY` — 正文编辑器与提交审核入口
2. `D2_SUBMIT_REVIEW_DRAWER` — 发起内容审核抽屉
3. `STATE_TASK_RUNNING_BAR` — 审核运行中
4. `D2_REVIEW_V2` — 审核结果总览
5. `I17_D2_REVIEW_ISSUE_DETAIL` — 问题详情与全文定位
6. `STATE_TASK_FAILED_NOTICE` — 审核失败与安全恢复
7. `STATE_NOT_CONFIGURED_EMPTY` — 审核工作流未配置
8. `I17_D2_REVIEW_HISTORY` — 审核历史

原型冻结 ACF 现有桌面 AppShell，只允许中心业务区变化。详细映射见 `ui-scope.md`、`ui-manifest.json`、`prototype-source-mapping.md` 和 `ui-contract-traceability.md`。

## 7. 实施顺序

严格按 `development-plan.md` 执行：契约 → 数据与后端 → 前端 → 真实 n8n → 异常与最终回归。不得在契约冻结前直接实现 Iteration 17。

## 8. 不在范围

- 不修改正文内容；
- 不自动生成新 ContentVersion；
- 不实现 Iteration 18 重写；
- 不允许业务页临时切换审核工作流；
- 不建设工作流可视化编辑器、字段映射器或多 n8n 路由；
- 不删除、替换或静默改变 P0 Mock Review 契约；
- 不在业务页面展示密钥、原始上游响应、SQL、堆栈、内部节点或基础设施地址。

## 9. 完成定义

- [ ] 业务规则、OpenAPI、数据模型、事务和 8 张原型一致；
- [ ] 固定版本、Run、Report、Issue 来源关系可追溯；
- [ ] 正常、未配置、运行、失败、输出非法、消费失败和刷新恢复闭环通过；
- [ ] 失败零部分领域数据；
- [ ] P0 Mock Review 与 Iteration 16 无回归；
- [ ] 真实 n8n 正常链路、异常回归、独立 Code Review 和 Git 门禁完成。
