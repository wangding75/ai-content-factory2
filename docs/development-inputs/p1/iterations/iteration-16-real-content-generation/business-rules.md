# CF-16-01A — 真实正文生成业务规则与状态机

**状态：`frozen_cf_16_01a`。** 本文件是业务语义的权威来源；字段、索引、Migration 终态与事务由 `data-model.md`、`transaction-and-migration-design.md` 固定；HTTP 契约由 CF-16-01B 的既有 OpenAPI 固定。

## 1. 聚合、输入与范围

- 正文聚合为 `ContentItem`，路由 `workId` 等于 `ContentItem.id`；`ContentVersion` 是不可变版本，`current_version_id` 是唯一当前版本指针。
- 只有 `confirmed` ChapterPlan 对应的 ContentItem 可预检或创建 Run。真实生成只读取并冻结输入快照，绝不修改 ChapterPlan、故事线、素材或伏笔。
- P0 Mock 生成入口保留但不是本闭环的真实生成依据；本迭代不自动审核、重写或将候选设为当前，也不新建 Work/候选聚合。
- 预检和 Run 保存确定性、脱敏的上下文摘要/快照。补充要求最大 2,000 字，空字符串规范化为 null；不得保存密钥、内部 URL、原始上游响应或未授权素材全文。

## 2. 预检与 Run 创建

预检是只读操作：不创建 Run/Event/Version/幂等记录，不调用 n8n 或其他外部执行器，不改变当前版本。它校验 ContentItem/项目/ChapterPlan 关系、ChapterPlan confirmed、当前版本 ID 与乐观锁版本、`content_generation` 绑定、执行可用性、活跃 Run、上下文及请求长度。`blocked` 是正常业务结果；通过时签发 10 分钟有效的 Token，绑定 actor、项目、ContentItem、源版本、上下文选项、补充要求摘要和关键配置版本。

创建仅接受有效 Token，服务端固定 `stage=content_generation`、`triggerSource=manual`、`subjectType=content_item` 和 `subjectId=ContentItem.id`；客户端不能走通用 Runtime 创建入口。创建时重新校验 Token、源版本、绑定/配置和活跃 Run。任一漂移要求重新预检。

创建命令须使用持久化 `Idempotency-Key`：相同作用域、相同键且相同规范化请求返回首次 Run；相同键而请求不同返回 `idempotency_key_reused_with_different_payload`。同一 `projectId + stage + subjectType + subjectId` 最多一个 queued/running Run；不同 ContentItem 可以并行。

## 3. 候选与 Summary 状态机

候选 ContentVersion 必须为 `source=workflow_generated`、`status=editable_draft`、新 `version_no`，并保存来源 ContentVersion ID/乐观锁版本和来源 WorkflowRun。它默认不是 current；每 Run 最多一个；历史版本不覆盖、不删除。候选在切换为当前前只读，保存草稿继续仅更新当前 editable 版本。

| Summary state | 判定 | 数据终态与恢复 |
|---|---|---|
| `idle` | 可生成且没有需展示的 Run 结果 | 当前正文可读，允许预检。 |
| `not_configured` | 绑定缺失或执行不可用 | 无 Run/候选；引导至最近配置入口。 |
| `queued` | 活跃 Run 排队 | Summary 从 Run/Event 恢复。 |
| `running` | 活跃 Run 运行 | Summary 从 Run/Event 恢复。 |
| `candidate_ready` | 输出已校验且候选已原子写入 | 显示候选；当前版本仍不变。 |
| `runtime_failed` | Runtime failed/cancelled | 零候选；可走 Runtime Retry（新 Run）。 |
| `output_validation_failed` | succeeded Run 输出不符合协议 | 零候选；修复后走 Runtime Retry（新 Run）。 |
| `result_consumption_failed` | 输出返回且已通过协议，但候选未安全写入 | 零候选；仅可结果消费 Retry。 |

`WorkflowRun.succeeded` 仅表示外部执行成功，绝不等价于 `candidate_ready`。页面刷新必须由 Summary、Run、Event 和版本历史恢复，不依赖内存状态。失败始终保持当前正文可读，不能产生部分候选。

## 4. Runtime、消费与重试

Runtime 生命周期仍为 `queued → running → succeeded|failed|cancelled`。对 succeeded Run，先校验标题、正文、摘要、字数、安全边界和完整输出 Schema；合法输出在单一事务创建候选并追加 `result_consumed` Event。事务失败后以独立安全事务追加 `result_consumption_failed` Event，且零候选写入。

同一 Run 的重复消费幂等：已有候选时返回该候选，绝不产生第二个版本或序号。Runtime Retry 由 Iteration 14 创建新 Run；结果消费 Retry 仅针对已有 succeeded Run 的既有输出重新运行领域消费，绝不再次调用外部工作流。

## 5. 设为当前、CAS 与 stale

设为当前命令必须含 `candidateVersionId`、`expectedCurrentVersionId`、`expectedCurrentVersion` 和 `Idempotency-Key`。候选必须同属该 ContentItem、为非当前 `workflow_generated` editable 候选。服务端同时校验：请求 expected ID/version 等于实际当前版本，且实际当前 ID/version 等于候选保存的 `source_content_version_id/source_content_version_version`。

任一条件不成立返回 `candidate_source_stale`（或当前版本冲突），不提供强制覆盖。stale 候选仍可查看和比较，用户必须基于最新当前版本重新生成。成功仅切换 `current_version_id`，将 ContentItem 置 `draft` 并清空 `reviewed_at`；旧正文、版本、审核关系均保留。相同幂等键和请求返回首次成功结果，不能重复变更。

## 6. 相邻边界

用户明确选择当前版本后才可继续编辑或按 Iteration 17 针对明确 `ContentVersion.id` 提交审核；审核 Runtime stage 是 `review`。Iteration 16 不自动创建 ReviewReport、不自动提交审核，也不触发 Iteration 18 重写。
