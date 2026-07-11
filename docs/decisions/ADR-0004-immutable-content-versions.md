# ADR-0004：重写不覆盖内容版本

- 状态：Accepted
- 决策：重写总是新增 ContentVersion，并记录 parent_version_id。
- 原因：保证审核、生成和修改历史可追踪，避免原稿丢失。
- 后果：需要版本列表、current version 和历史审核关联。
