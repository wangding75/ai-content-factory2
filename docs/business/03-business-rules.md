# AI Content Factory 2.0｜业务规则目录

## BR-PROJECT

- BR-PROJECT-001：项目名称不能为空，长度 1–120。
- BR-PROJECT-002：P0 只允许 `type=novel`。
- BR-PROJECT-003：新项目默认状态为 `planning`。
- BR-PROJECT-004：P0 不允许删除、复制和归档项目入口。

## BR-MATERIAL

- BR-MATERIAL-001：Material 是全局唯一素材本体。
- BR-MATERIAL-002：项目用途必须存储在 ProjectMaterialUsage。
- BR-MATERIAL-003：同一项目与同一素材最多一个 Usage。
- BR-MATERIAL-004：项目内创建素材必须原子创建 Material 与 Usage。
- BR-MATERIAL-005：解除绑定只删除 Usage，不删除 Material。
- BR-MATERIAL-006：编辑 Material 会影响所有引用项目；UI 必须提示全局影响。

## BR-NARRATIVE

- BR-NARRATIVE-001：主线和子线统一使用 PlotLine。
- BR-NARRATIVE-002：根故事线 parent_id 为空，子线必须引用同项目父线。
- BR-NARRATIVE-003：不能形成父子循环。
- BR-NARRATIVE-004：伏笔必须属于项目，可选关联种下和回收故事线。

## BR-CONTENT

- BR-CONTENT-001：只有 confirmed ChapterPlan 可创建 ContentItem，且每个 ChapterPlan 最多一个。
- BR-CONTENT-002：创建时原子创建空白 ContentVersion v1（manual_created/editable_draft）；本轮不创建 v2。
- BR-CONTENT-003：`version_no` 是历史编号，`version` 是乐观锁，`expected_version` 是写入条件；不得混用。
- BR-CONTENT-004：可编辑草稿支持重复保存；省略字段不变、null 清空可空字段、空字符串保留为空值；冻结/已审核版本不可编辑。
- BR-CONTENT-005：Mock Generate 和 Mock Review 必须确定性、同步且使用 Idempotency-Key；同键同 payload 重放同一结果，不得重复创建版本、审核或 WorkflowRun。
- BR-CONTENT-006：Mock Review 成功原子冻结版本并创建审核数据；失败恢复 draft 且不保留部分审核数据。已审核版本以新键重审返回 content_version_already_reviewed。

## BR-CHAPTER

- BR-CHAPTER-001：Mock 生成只产生 pending_confirmation 候选。
- BR-CHAPTER-002：只有 pending 候选可编辑、删除和确认。
- BR-CHAPTER-003：确认后不可通过普通编辑接口修改。
- BR-CHAPTER-004：确认和进入正文生产是两个独立动作。
- BR-CHAPTER-005：只有 confirmed 规划可创建 ContentItem。

## BR-CONTENT

- BR-CONTENT-001：ContentItem 是作品单元，ContentVersion 是正文快照。
- BR-CONTENT-002：审核不能修改 ContentVersion.body。
- BR-CONTENT-003：重写必须新增版本，不允许覆盖源版本。
- BR-CONTENT-004：重写版本必须记录 parent_version_id。
- BR-CONTENT-005：重写版本默认不自动成为 current version。
- BR-CONTENT-006：重写版本不自动审核、不自动发布。

## BR-WORKFLOW

- BR-WORKFLOW-001：每次生成、审核和重写都必须创建 WorkflowRun。
- BR-WORKFLOW-002：P0 只允许 provider_key=mock。
- BR-WORKFLOW-003：失败运行必须保留错误摘要和结束时间。
- BR-WORKFLOW-004：运行失败不得留下未完成领域结果。

## BR-INTEGRATION

- BR-INTEGRATION-001：未配置能力必须明确显示未配置或未开放。
- BR-INTEGRATION-002：禁止伪造外部连接、发布成功或真实 AI 结果。
