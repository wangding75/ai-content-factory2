# Iteration 17 — 真实内容审核业务规则与状态机

**状态：`frozen_cf_17_01`。** 本文件是 Iteration 17 业务语义的权威来源；HTTP 字段由正式 OpenAPI 冻结，表结构与事务由 `data-model.md`、`transaction-and-migration-design.md` 固定。

## 1. 审核对象与固定版本

- 审核对象是明确的 `ContentVersion`，不是可变的 ContentItem 当前内容，也不是编辑器内存草稿。
- 发起审核前，客户端必须先保存正文；存在未保存修改时，“提交审核”不可执行。
- 预检和创建均校验 `contentVersionId`、乐观锁 `version`、内容安全摘要和所属 Project/ContentItem。
- 创建 Run 后，来源版本不得再被原地修改。若来源仍为 editable，创建事务只允许将其冻结，不改变 title/content/summary/wordCount。
- 提交审核不自动创建新版本。需要继续编辑时，用户必须通过既有版本机制进入新的 editable 版本；历史审核始终指向原固定版本。
- 审核历史允许同一版本多次运行；每次成功运行生成独立报告，旧报告不覆盖。

## 2. 入口、预检与创建

编辑器只展示已经持久化的可审核版本。抽屉允许选择当前或历史固定版本；工作流由项目 `review` Binding 决定，不允许临时切换。审核维度来自 Workflow Configuration，只读展示。

预检是只读操作，不创建 Run/Event/Report/Issue，不调用 n8n，不冻结版本，不写幂等记录。预检至少检查：

- Project、ContentItem、ContentVersion 归属；
- 来源版本存在、已保存、未发生乐观锁漂移；
- `review` Binding、Workflow Configuration 与 Connection 可用；
- 同一 ContentVersion 没有 active review Run；
- 审核说明长度不超过 2,000 个 Unicode 字符；
- 当前配置声明 `review.input.v1` / `review.output.v1`。

通过时签发 10 分钟有效的 Preflight Token，绑定 actor、projectId、contentItemId、sourceContentVersionId/version/hash、optionalInstructions、reviewDimensions、Binding/Configuration/Connection 版本、input digest、nonce 与 expiresAt。

创建命令必须使用 `Idempotency-Key`。服务端固定：

- `stage=review`；
- `triggerSource=manual`；
- `subjectType=content_version`；
- `subjectId=ContentVersion.id`。

创建时重新复核 Token、来源版本、配置快照和 active Run。任一漂移返回需重新预检的冲突；相同键同请求回放首次 Run，相同键不同请求返回幂等冲突。

## 3. Runtime 输入与输出

### 3.1 `review.input.v1`

安全输入按固定顺序包含：

- schemaVersion=`review.input.v1`；
- projectId、contentItemId、sourceContentVersionId、sourceContentVersionVersion、sourceContentHash；
- sourceTitle、sourceContent、optionalInstructions；
- reviewDimensions，键固定为 `compliance/factual_consistency/language_quality/structural_logic/character_consistency`；
- workflowRunId、correlationId。

Run/Event/日志不得记录 Preflight Token、Idempotency-Key 原文、凭据、Authorization、Cookie 或内部地址。

### 3.2 `review.output.v1`

输出必须是严格 JSON，未知字段和尾随 JSON 均拒绝。最小结构：

```json
{
  "schemaVersion": "review.output.v1",
  "conclusion": "passed",
  "summary": "审核通过",
  "passedRuleCount": 0,
  "issues": [
    {
      "issueKey": "character-consistency-1",
      "position": 1,
      "categoryKey": "character_consistency",
      "categoryLabel": "角色一致性",
      "severity": "warning",
      "title": "人物行为与设定不一致",
      "description": "问题说明",
      "evidence": {
        "quote": "必要短引文",
        "sourceRefs": ["sourceContentVersion"]
      },
      "location": {
        "paragraphStart": 1,
        "paragraphEnd": 1,
        "sentenceStart": 1,
        "sentenceEnd": 1
      },
      "suggestion": "修改建议"
    }
  ],
  "recommendations": [
    {
      "position": 1,
      "priority": "medium",
      "title": "报告级建议",
      "description": "建议说明"
    }
  ]
}
```

规则：

- `summary` 必填；`issues` 可为空，空数组只允许 `conclusion=passed`；
- 每个 Report 最多 200 个 Issue；`issueKey` 与从 1 开始连续的 `position` 在 Report 内分别唯一；
- `recommendations` 最多 100 条，使用从 1 开始的稳定 position，只保存报告级建议；
- 所有字符串使用 Unicode 长度限制；正文证据只能保存必要短片段；
- severity 仅允许 `critical/warning/suggestion`；location 可空，非空时必须满足段落和句子范围；evidence 必须是固定对象；
- 顶层和嵌套对象拒绝未知字段，解析器拒绝尾随 JSON、非法枚举、重复 issueKey/position 与任何凭据或内部信息字段；
- Provider 不得返回 issue disposition，系统创建时统一为 `open`。

## 4. Report、Issue 与处置

- 一个成功 review Run 最多一个 ReviewReport；Report 永久关联来源 ContentVersion 和 WorkflowRun。
- ReviewIssue 是 API/领域名称，持久化复用并扩展 P0 `review_findings`，禁止新建平行 `review_issues` 表。
- Issue 保存分类、严重级别、说明、证据、位置、建议和稳定顺序。
- 用户可将 Issue 标记为 `ignored` 或恢复为 `open`；该动作不修改 Provider 输出、不删除 Issue、不改变 Report 原始结论和统计。
- Report 统计显示原始严重/警告/建议数量；处置统计单独计算。
- Iteration 18 可以读取 `open` Issue 创建重写任务；Iteration 17 仅提供来源数据和禁用入口。

## 5. Summary 状态机

| State | 持久化依据 | UI 行为 |
|---|---|---|
| `idle` | 无需展示的 Run/Report，配置可用 | 可发起审核 |
| `not_configured` | 无可恢复执行事实，且 review Binding/配置/连接不可用 | 引导项目设置 |
| `queued` | 最新 active Run 为 queued | 显示任务排队，可离开页面 |
| `running` | 最新 active Run 为 running | 显示安全进度与运行详情 |
| `review_ready` | 最新 Run 已成功消费并存在对应 ReviewReport | 展示报告和问题 |
| `runtime_failed` | 最新 Run failed/cancelled | Runtime Retry 创建新 Run |
| `output_validation_failed` | succeeded Run 输出不符合 `review.output.v1` | Runtime Retry 创建新 Run，零报告 |
| `result_consumption_failed` | 输出合法，但 Report/Issue 事务失败 | 仅结果消费 Retry，不外呼 |

`WorkflowRun.succeeded` 不等于 `review_ready`。Summary 必须把 Report 绑定到同一 LatestRun，不得展示其他 Run 的历史报告。配置后来失效时，已经存在的 running、failure 或 report 事实仍优先展示。

## 6. 重试规则

- Runtime Retry：适用于 failed、cancelled、output_validation_failed；创建新 Run并保留 `retryOfRunId`，不复用原 Preflight Token。
- 结果消费 Retry：只适用于 succeeded、存在 `result_consumption_failed`、未生成 Report 的 Run；只重新消费已持久化输出，绝不调用 n8n。
- 同一 Run 重复消费幂等；已有 Report 时返回原 Report，不创建第二份。
- 普通 succeeded、已消费、已有 Report 或不属于 `review` 的 Run 不可调用审核消费 Retry。

## 7. ContentItem 状态兼容

成功创建 Report 后：

- 若来源版本仍是当前版本且结论为 `passed`：ContentItem 可置为 `reviewed` 并写 `reviewed_at`；
- 若来源版本仍是当前版本且结论为 `needs_changes`：ContentItem 保持/恢复 `draft`，`reviewed_at=NULL`；
- 若来源版本已不是当前版本：不得修改 ContentItem 当前状态或 `reviewed_at`。

任何失败都不得改变正文、current_version_id 或产生部分 Report/Issue。

## 8. 相邻迭代边界

- Iteration 16 只负责产生并选择正文版本；不会自动审核。
- Iteration 17 只审核固定版本并管理问题处置；不会修改正文或创建重写版本。
- Iteration 18 以明确 ReviewReport 和 Issue 为输入，生成新的非当前候选版本。
- P0 Mock Review 继续走既有同步接口和 DTO；真实审核不得改变其幂等和历史读取行为。
