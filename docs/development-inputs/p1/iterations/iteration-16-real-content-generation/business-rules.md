# CF-16-01A — 真实正文生成业务规则与状态机

**状态：`review_ready_cf_16_01a`。** 本文件是 Iteration 16 业务结果的权威来源。字段、索引和事务实现由 `data-model.md`、`transaction-and-migration-design.md` 约束；完整 HTTP Schema 和错误码由主 OpenAPI 约束。

## 1. 正文聚合与入口

- 正文聚合是 `ContentItem`，正式编辑器 `workId` 等于 `ContentItem.id`。
- `ContentItem` 必须关联一个已确认 `ChapterPlan`；未确认或失效章节不得创建正文生成 Run。
- 如果已确认章节还没有 `ContentItem`，继续复用 `POST /api/v1/chapter-plans/{chapterPlanId}/content` 幂等创建正文和空白 v1，再进入编辑器。
- 真实生成不修改 `ChapterPlan`、故事线、素材或伏笔，只读取并冻结生成输入快照。

## 2. 当前版本与候选版本

- `ContentItem.current_version_id` 是唯一当前版本来源。
- 真实生成成功创建新的 `ContentVersion`：
  - `source=workflow_generated`；
  - `status=editable_draft`；
  - `version_no=max(existing version_no)+1`；
  - `source_content_version_id` 指向发起生成时的当前版本；
  - `source_content_version_version` 保存该源版本发起时的乐观锁版本；
  - `source_workflow_run_id` 指向本次 Runtime Run。
- 创建候选后 `current_version_id` 保持不变，当前正文、标题、审核关系和旧版本均不改变。
- 候选版本在设为当前前只读；保存草稿 API 始终只更新当前 editable 版本。
- 每个成功 Run 最多创建一个候选版本；同一 Run 重复消费返回既有结果。

## 3. 上下文和补充要求

默认上下文包括：

- 已确认 ChapterPlan 的本章目标、关键情节、角色变化、建议字数、推进阶段和情绪基调；
- 与本章关联的故事线、角色、世界设定和伏笔；
- 用户允许注入的项目素材；
- 用户允许注入的前序章节摘要；
- 本次补充要求。

规则：

- 预检和 Run 输入必须保存脱敏、确定性的 context snapshot 与摘要；
- 不保存密钥、内部 URL、原始上游响应或未授权素材正文；
- 补充要求最长 2,000 字；空字符串规范化为 null；
- 客户端“生成完成后返回当前编辑器”属于 UI 偏好，不进入领域请求。

## 4. 预检

预检是只读操作，不创建 Run、不调用外部执行器、不创建版本、不修改当前版本。

预检校验：

1. ContentItem、项目和 ChapterPlan 关系有效；
2. ChapterPlan 为 `confirmed`；
3. 当前版本 ID 和乐观锁版本未漂移；
4. 项目存在 `content_generation` 绑定；
5. 绑定的配置和连接存在，且执行集成可用；
6. 当前 ContentItem 没有 queued/running 的同阶段 Run；
7. 上下文引用和请求长度合法。

预检返回 `passed` 或 `blocked`。`blocked` 是 HTTP 200 正常业务结果。通过时签发 10 分钟有效的 `preflightToken`，绑定 actor、项目、ContentItem、源版本、上下文选项、补充要求摘要和关键配置版本。

## 5. Run 创建和并发

- 创建 Run 只接受有效 `preflightToken`，服务端固定 `stage=content_generation`、`triggerSource=manual`、`subjectType=content_item` 和 `subjectId=ContentItem.id`。
- 客户端不得直接调用通用 `createWorkflowRun` 发起正文生成。
- 同一 ContentItem 最多一个 `queued` 或 `running` 的正文生成 Run；不同 ContentItem 允许并行。
- 创建时重新校验 Token、源版本、绑定配置和活跃 Run；任一漂移都拒绝创建并要求重新预检。
- 创建命令使用持久化 `Idempotency-Key`：同 Key 同载荷返回首次 Run，同 Key 不同载荷冲突。

## 6. Runtime 和领域消费

Runtime 状态仍为 `queued → running → succeeded|failed|cancelled`。领域消费独立追踪：

1. 外部结果到达；
2. 完整校验输出 Schema、标题、正文、摘要、字数和内容安全边界；
3. 输出合法后，在单一事务创建一个候选 ContentVersion；
4. 成功追加 `result_consumed` Event；
5. 事务失败追加 `result_consumption_failed` 安全 Event，不创建部分版本。

`WorkflowRun.succeeded` 不单独证明候选已创建；编辑器使用正文生成 Summary 的 `state` 和 `latestCandidateVersion` 判断结果。

## 7. 失败和重试

| 失败类型 | 数据结果 | 恢复方式 |
|---|---|---|
| Runtime failed/cancelled | 零候选版本 | 查看详情；按 Iteration 14 Retry 创建新 Run |
| `output_validation_failed` | 零候选版本 | 修复工作流输出或 Retry 新 Run；不得重放非法输出 |
| `result_consumption_failed` | 零候选版本 | 使用专用结果消费重试；不重复调用外部工作流 |
| 页面刷新 | 无数据变化 | 读取 Summary、Run、Event 和版本历史恢复 |
| 未配置/执行不可用 | 不创建 Run | 前往项目工作流设置或全局连接设置 |

所有错误只返回安全 code、message 和 details。不得返回密钥、请求头、内部 URL、堆栈、SQL、原始上游响应或原始幂等键。

## 8. 设为当前版本

用户查看候选并点击“设为当前版本”时：

- 请求必须携带 `candidateVersionId`、`expectedCurrentVersionId` 和 `expectedCurrentVersion`；
- 候选必须属于同一 ContentItem、`source=workflow_generated`、非当前、`status=editable_draft`；
- 当前版本 ID 和乐观锁版本必须仍等于请求中的 expected 值；
- 当前版本 ID/版本还必须等于候选的 `source_content_version_id/source_content_version_version`，否则候选基线已过期，禁止强制覆盖；
- 成功只更新 `ContentItem.current_version_id`，把 ContentItem 状态恢复为 `draft` 并清空 `reviewed_at`；
- 旧当前版本、审核报告和历史关系保持不变；
- 命令使用 `Idempotency-Key`，同键重放返回首次结果。

不存在“强制设为当前版本”。基线过期时用户需基于最新当前版本重新生成。

## 9. 与审核和重写的边界

- “提交审核”必须明确绑定当前 `ContentVersion.id`，由 Iteration 17 处理。
- 设为当前版本不会自动冻结或审核。
- Iteration 18 重写也创建新版本，但来源 stage、输入和审核关系与正文生成不同。
- 版本来源必须可区分 `manual_created`、`mock_generated`、`mock_rewrite` 和 `workflow_generated`。
