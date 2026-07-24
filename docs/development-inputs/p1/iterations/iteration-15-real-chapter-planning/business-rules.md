# CF-15-01A — 业务规则与状态机冻结

**状态：`frozen_cf_15_01a`。** 本文件是 Iteration 15 章节规划、候选批次、采用和确认语义的唯一业务规则来源。CF-15-01B 冻结字段、索引、事务实现；CF-15-01C 冻结完整 API Schema 与错误码。两者不得改变本文件的业务结果。

## 1. 权威范围与用户闭环

Iteration 15 的权威名称为“真实章节规划与候选批次闭环”。闭环固定为：章节规划工作区 → 设置生成模式 → 无副作用预检 → 用户确认 → 创建既有 `WorkflowRun(stage=chapter_planning)` → 生成候选批次 → 查看、编辑、比较、采用或放弃候选 → 当前章节进入 `pending_confirmation` → 用户确认 → 已确认章节具备 Iteration 16 正文生产资格。

生成结果始终先进入候选批次，绝不直接覆盖当前 `ChapterPlan`；采用、确认和正文生产是三个独立动作。故事线页面只展示章节关联、已确认、规划中和候选推荐的只读聚合统计，不拥有或编辑章节范围。

## 2. 生成策略与故事线

所有模式都提交目标、故事线选择、上下文选项和附加要求；服务端根据预检输入摘要确定其语义。

| 模式 | 目标 | 已有章节处理 |
|---|---|---|
| `full` | 生成从第 1 章至目标总章节数的完整规划。 | 同章号的当前章节只作为替换候选的基线；不存在即为新增候选。 |
| `append` | 从项目当前最大章节号后的下一章起，连续生成指定数量。没有当前章节时从第 1 章起。 | 不改变已存在章节；目标范围内出现已有章节表示预检输入已漂移，创建 Run 时必须阻断并重新预检。 |
| `range` | 对含首尾的指定章节号范围生成规划。 | 已存在章节形成替换候选，缺失章节形成新增候选。 |

目标章节号为正整数；`full` 与 `append` 的数量、`range` 的区间均须由后续契约施加同一安全上限。范围首尾不得倒置。选择一个故事线父节点时，输入快照包含该节点及其全部有效后代；选择子节点不隐含父节点或同级节点。故事线选择仅影响生成上下文，不授予故事线对章节的所有权。失效、删除或跨项目引用均为阻断项。

## 3. 预检、Token 与 Run

预检只读校验项目上下文、目标、故事线树和引用、工作流绑定与安全配置摘要、当前活跃 Run，以及输入版本；不得创建 Run、CandidateBatch、Candidate 或 ChapterPlan，不得调用 n8n、LLM、Worker、Callback 或任何外部执行器。

预检返回短期 `preflightToken`、输入摘要、检查、警告和阻断项。Token 仅对同项目、同用户、同一输入摘要有效，自签发起 **10 分钟** 后失效；它不含密钥或原始配置。创建 Run 必须携带该 Token，服务端必须复核其有效性、输入摘要、关键配置和活跃 Run。Token 过期、输入不同或复核失败时不得创建 Run，用户必须重新预检。

同一项目同一 `chapter_planning` stage 最多存在一个状态为 `queued` 或 `running` 的 Run；预检和创建时均执行该检查。Run 成为 `succeeded`、`failed` 或 `cancelled` 后锁释放；Retry 创建新的 Run 时同样受此锁约束。创建入口复用 Iteration 14 Runtime，服务端固定 `stage=chapter_planning` 与 `triggerSource=manual`，并沿用持久化幂等。

## 4. Run 成功与结果消费

`WorkflowRun.succeeded` 仅表示 Runtime 已成功结束，**不**表示候选批次已被成功消费。结果消费独立执行：先完整校验规范化输出、章节号、数量、引用、目标和业务约束，再在单一事务创建一个 Batch 及其全部 Candidate。

任意输出非法或事务失败时，写入零 Batch、零 Candidate；Run 与安全错误记录保留，且不伪造无效 Batch。`sourceWorkflowRunId` 至多对应一个 Batch；同一 Run 的重复消费必须返回既有成功结果或既有安全失败结果，不能重复写入。Run 的 Retry 仍由 Runtime 创建新的 Run 并重新走消费流程。

## 5. Candidate、Batch 与比较

Candidate 的唯一持久状态为：

| 状态 | 含义与允许迁移 |
|---|---|
| `pending` | 可编辑、比较、重新比较、采用或放弃。基线不一致时转为 `stale`。 |
| `stale` | 当前章节相对候选基线已变化；可编辑、比较、重新比较或放弃，禁止采用。编辑后只有服务端以最新基线重算一致时才回到 `pending`。 |
| `adopted` | 已采用的终态，不可编辑、再次采用或放弃。 |
| `discarded` | 已放弃的终态，不可编辑或采用。 |

`edited` 不是状态或终态；编辑只递增 Candidate 的乐观锁版本，并重新计算差异与 `pending`/`stale`。Compare 与 Recompare 都只呈现当前章节、候选和差异；Recompare 同时以最新版本重算 stale 判定。不存在“强制采用”或任何绕过 stale 的路径。

Batch 的唯一状态为 `ready`、`partially_adopted`、`adopted`、`abandoned`：新建为 `ready`；存在已采用且仍有待处理或 stale Candidate 时为 `partially_adopted`；所有 Candidate 均已采用或放弃且至少一项被采用时为 `adopted`；用户放弃后为 `abandoned`。`abandoned` 后不能再采用或编辑其未采用 Candidate。状态由 Candidate 事实派生并持久化，历史 Batch 和 Candidate 均不得删除。

## 6. 采用、Revision、放弃与确认

单个 Adopt 是一个原子事务：锁定并校验 Candidate/Batch；校验 Candidate 为 `pending`、乐观锁与当前 ChapterPlan 版本；创建新增章节或不可变 `ChapterPlanRevision` 并更新当前章节；写入来源追溯；将 Candidate 设为 `adopted`；更新 Batch 统计与状态。无变化候选返回明确的 no-op 结果，不创建 Revision，也不标记 adopted。任何失败均回滚该 Candidate 的所有写入。

批量 Adopt 先逐项判定；每个 Candidate 独立事务，允许部分成功。响应必须逐项返回 `adopted`、`no_change`、`stale/conflict` 或 `failed` 的可追溯结果。一个 Candidate 的失败不得回滚其他已提交 Candidate，也不得被静默跳过；stale 或其他冲突项绝不写入当前章节。

Discard 仅将可处理 Candidate 变为 `discarded`，保留历史与来源。Batch abandon 把尚未采用的 Candidate 变为 `discarded` 并将 Batch 变为 `abandoned`，不删除历史、不回滚已采用 Revision 或当前章节；重放为幂等结果。

Adopt 绝不等于 Confirm。每次采用创建或替换的当前章节均为 `pending_confirmation`，包括此前已 `confirmed` 的章节再次被采用时；只有复用 P0 确认动作才可转为 `confirmed`。仅 `confirmed` ChapterPlan 能进入 Iteration 16 正文生产，未确认章节必须继续被拒绝。

## 7. 相邻迭代边界

Iteration 15 只复用 Iteration 14 的 WorkflowRun、Event、Cancel、Retry 与安全快照，不建立第二套 Run。`CF-14-N8N-Integration` 负责真实 n8n Transport、Worker、队列消费、Callback、外部状态回写及连接/执行可用性；本迭代不实现这些能力，最终真实执行闭环验收依赖该任务 PASS。Iteration 15 不实现正文生成、ContentVersion 或 Iteration 16 业务。
