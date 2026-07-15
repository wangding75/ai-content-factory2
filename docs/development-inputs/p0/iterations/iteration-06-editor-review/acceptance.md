# Iteration 06 completion: completed (final verification passed)

# Iteration 06.1 验收

| ID | 入口/操作/请求 | 页面与数据验收 |
|---|---|---|
| I06-AC01 | confirmed ChapterPlan 进入 D1，创建或 Retry | 首次 POST 返回 201 并原子创建唯一 ContentItem+空 v1；重复为 200 同一 ID；非 confirmed 为 409 chapter_plan_not_confirmed。|
| I06-AC02 | D1 Loading/Empty/Error 后编辑并 PUT draft | required expected_version；omitted/null/空串语义正确；成功 version+1，刷新 GET 一致；409 conflict/locked 有反馈。|
| I06-AC03 | D1 Mock Generate、取消与同键重试 | 200 同步、确定性，仅更新 editable v1 且仍 draft；首次有效执行一个 run；同键不重复 version/run；失败原子回滚，项目切换隔离旧响应。|
| I06-AC04 | D1 提交 D2 Mock Review | 请求带 version ID、expected_version、key；成功页显示“已审核”；事务冻结 v1、创建 completed report/findings/recommendations 和 succeeded run、写 reviewed_at。|
| I06-AC05 | D2 审核失败、Retry、重复请求 | 回到 draft，无部分审核记录，有 failed run；同键返回原结果；已审核版本新键为 409 content_version_already_reviewed。|
| I06-AC06 | D2 列表及详情 | 列表稳定为 created_at DESC,id DESC；详情含 Report、版本摘要、Findings、Recommendations、WorkflowRun 摘要，均指向固定 v1。|
| I06-AC07 | D2 重写入口 | 按钮禁用并标注 Iteration 07；没有重写 API、v2 或 D4 跳转。|

每项 PostgreSQL 验收都检查事务边界、唯一性、版本号、冻结时间、关联外键和失败后无脏审核记录。错误响应只含稳定 code、用户安全 message、可选安全 details 和 request_id，绝不含 SQL/堆栈。
