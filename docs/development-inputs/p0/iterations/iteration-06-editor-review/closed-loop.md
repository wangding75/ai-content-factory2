# D1/D2 闭环

## D1_EDITOR

入口是 confirmed ChapterPlan。页面先创建或打开 ContentItem：首次 `POST /chapter-plans/{id}/content` 得到 201，重复进入得到同一对象的 200；非 confirmed 显示不可创建错误。加载中禁用编辑动作，空态引导创建，错误态提供 Retry；Retry 不得造成重复 ContentItem。

用户编辑 title、content 或 summary 后以 `PUT /content-items/{id}/draft` 与 `expected_version` 保存。未发送字段保持原值，`summary:null` 清空，`summary:""` 保存为空；成功显示新 version，409 version_conflict 刷新后提示合并/重试，409 content_version_locked 转只读。刷新后 GET 读回持久化草稿。

“模拟生成正文”可取消尚未发出的请求；已发出的请求使用 `POST /mock-generate`、`Idempotency-Key`、`expected_version` 和冻结参数。成功同步更新 v1、版本及 WorkflowRun，仍为 draft；同键重试显示同一结果；不同 payload 显示幂等冲突；失败提示可用新键重试且保持原草稿。切换项目/章节时丢弃旧响应。

“提交审核”以当前 version 进入 D2；提交后 D1 不再允许编辑。Iteration 06 不创建 v2。

## D2_REVIEW

入口是 D1 当前 ContentItem/v1。提交 `POST /reviews/mock` 时发送 `content_version_id`、`expected_version`、`Idempotency-Key`。请求中可显示提交中，但成功页面只显示“已审核”，不得显示“待审核”。成功返回 reviewed ContentItem、冻结 ContentVersion、ReviewReport、Findings、Recommendations 与 succeeded WorkflowRun；正文快照固定指向本次 v1。

审核失败显示错误和 Retry（以同键重试返回原失败结果，使用新键重新执行）；服务端恢复 draft、没有部分 Report/Finding/Recommendation，并记录 failed WorkflowRun。相同成功键返回同一 Report；已审核 v1 使用新键返回 `409 content_version_already_reviewed`。

审核列表以 `GET /content-items/{id}/reviews?limit&offset` 显示，固定 `created_at DESC, id DESC`；详情以 `GET /reviews/{id}` 显示 Report、v1 摘要、Findings、Recommendations 和 WorkflowRun 摘要。重写按钮在冻结位置禁用并标注“Iteration 07”；不得进入 D4。项目切换必须隔离旧列表、详情和提交响应。

PostgreSQL 验收：创建唯一约束 `chapter_plan_id`；保存/生成乐观锁递增；review 成功事务同时落库版本冻结、报告、问题、建议、成功 run；review 失败不落库部分审核数据且 ContentItem 为 draft，失败 run 可追溯。
