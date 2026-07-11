# 验收方案

| 用例 ID | 场景 | 通过标准 |
|---|---|---|
| I07-AC01 | 创建 v2 保留 v1 | 重写成功后 versions 数量增加 1，v1 内容和记录仍存在。 |
| I07-AC02 | 不自动切换当前版本 | 生成 v2 后 current_version_id 仍保持原值，除非用户后续明确选择。 |
| I07-AC03 | 不自动审核与发布 | v2 初始 review_status=not_submitted，publish_status=not_published。 |
| I07-AC04 | 取消无副作用 | D4 取消时不创建 WorkflowRun 或 ContentVersion。 |
| I07-AC05 | 项目作品聚合 | D3 可显示章节、当前版本、审核状态和可用版本数量。 |
| I07-AC06 | 运行可追踪 | WorkflowRun 输入、输出摘要、开始/结束时间和错误信息可查询。 |

## 门禁

- 核心用例必须可重复执行。
- 不接受仅页面打开或 HTTP 200 的冒烟结果。
- 失败分支必须验证数据库无脏数据。
