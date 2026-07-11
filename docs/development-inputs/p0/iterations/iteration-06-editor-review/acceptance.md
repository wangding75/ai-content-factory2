# 验收方案

| 用例 ID | 场景 | 通过标准 |
|---|---|---|
| I06-AC01 | 创建正文门槛 | 只有 confirmed ChapterPlan 才能创建 ContentItem。 |
| I06-AC02 | 草稿保存 | 编辑正文并保存后刷新，正文、字数和更新时间一致。 |
| I06-AC03 | 模拟生成 v1 | 首次模拟生成创建 v1，并记录 source=mock_generated。 |
| I06-AC04 | 审核结果完整 | 模拟审核产生可展示的问题与建议集合，D2 可正确渲染。 |
| I06-AC05 | 审核不覆盖正文 | 创建 ReviewReport 前后 ContentVersion 正文哈希不变。 |
| I06-AC06 | 返回编辑器 | 从 D2 返回 D1 后加载同一 contentItemId 和同一版本。 |

## 门禁

- 核心用例必须可重复执行。
- 不接受仅页面打开或 HTTP 200 的冒烟结果。
- 失败分支必须验证数据库无脏数据。
