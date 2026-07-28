# Iteration 16 — 真实正文生成验收标准

## 1. 契约、范围与任务

| ID | 场景 | 通过标准 |
|---|---|---|
| I16-AC01 | 聚合与路由 | 使用 ContentItem/ContentVersion，`workId=ContentItem.id`；不新增 Work、候选表或第二套 Run。 |
| I16-AC02 | 相邻边界 | 复用 Iteration 13 binding 与 Iteration 14 Runtime；不自动审核、重写或覆盖当前版本。 |
| I16-AC03 | 任务拆分 | iteration-plan、development-plan 和本文件一致列出 9 个 Agent 任务；人工验收/审核包/代码审核不计入任务。 |
| I16-AC04 | P0 Mock | P0 Mock 保留但不作为真实生成链路验收依据。 |

固定 9 个 Agent 任务为：CF-16-01A 业务、数据模型和事务契约冻结；CF-16-01B OpenAPI、UI 和追踪契约冻结；CF-16-02A Migration、Domain 和 Repository；CF-16-02B 预检、Run 创建和 Summary；CF-16-02C 结果消费、候选和版本切换；CF-16-03A 编辑器、生成抽屉和预检；CF-16-03B 运行状态、候选闭环和 UI 收口；CF-16-04A 真实 n8n 正常闭环；CF-16-04B 异常、安全、回归和收口。

## 2. 预检、Run 与恢复

| ID | 场景 | 通过标准 |
|---|---|---|
| I16-AC05 | 正文资格 | 只有 confirmed ChapterPlan 的 ContentItem 可通过。 |
| I16-AC06 | 无副作用预检 | 不创建 Run/Event/Version/幂等记录，不外呼，不修改当前正文。 |
| I16-AC07 | Token | Token 绑定 actor/item/source/input/config，10 分钟有效，创建时完整复核。 |
| I16-AC08 | 专用创建 | 服务端固定 content_generation/manual/content_item/ContentItem.id，客户端不能用通用创建入口。 |
| I16-AC09 | active Run | 同正文只有一个 queued/running Run；不同正文可并行。 |
| I16-AC10 | 创建幂等 | 同键同请求返回首次 Run；同键不同请求冲突；不重复创建。 |
| I16-AC11 | Summary | `idle`、`not_configured`、`queued`、`running`、`candidate_ready`、`runtime_failed`、`output_validation_failed`、`result_consumption_failed` 均从持久化数据恢复。 |

## 3. 候选、消费与切换

| ID | 场景 | 通过标准 |
|---|---|---|
| I16-AC12 | 成功语义 | Runtime succeeded 不等于候选成功；仅校验和原子消费完成后为 candidate_ready。 |
| I16-AC13 | 候选 | 一个 Run 至多创建一个 `workflow_generated` 非当前候选，保留 source ContentVersion ID/version 与 source Run。 |
| I16-AC14 | 安全失败 | 非法输出或消费事务失败均零候选写入，当前正文不变；写安全失败 Event。 |
| I16-AC15 | 消费 Retry | 仅重试既有输出的领域消费，绝不重新调用 n8n/外部工作流，也不产生第二候选。 |
| I16-AC16 | CAS | expected 当前 ID/version 和候选来源基线均匹配时才切换 current pointer；成功后 status=draft、reviewed_at 清空。 |
| I16-AC17 | stale | 当前版本变化后候选仍可查看/比较，设为当前返回 candidate_source_stale；无强制覆盖。 |
| I16-AC18 | 切换幂等 | 相同幂等键重放首次结果，不重复改变版本状态。 |

## 4. 数据库与工程规则

- 当前项目仅使用一个开发数据库；Iteration 16 仅以新增向前 Migration 演进并验收最终状态。
- 历史 Migration 文件不变；不要求空库验证、历史数据库回滚、downgrade、任意版本切换或重建数据库。
- 不删除 Volume，不执行 `docker compose down -v`，不手工篡改业务数据规避问题。
- 最终结构含 Runtime subject、active subject Run 部分唯一索引、三项 ContentVersion 来源字段/FK、`workflow_generated`、每 Run 一个候选、消费 Event 和必要查询索引。

## 5. 最终流程门禁

- 9 个任务按 `development-plan.md` 的固定顺序完成；人工 UI 验收、审核包和完整代码审核作为开发完成后的项目流程进行。
- 真实 n8n 正常闭环、异常/并发/幂等/安全/P0 回归、文档与 Git 门禁由 CF-16-04A/04B 完成。
- 本任务及后续实现不得修改 Iteration 17/18 业务范围；审核必须由用户对明确 ContentVersion 显式发起。
