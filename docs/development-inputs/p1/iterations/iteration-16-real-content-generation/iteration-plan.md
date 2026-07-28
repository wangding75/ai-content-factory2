# Iteration 16 — 真实正文生成与候选版本闭环

**状态：`frozen_cf_16_01a`。** 业务、数据和事务契约已冻结；CF-16-01B 单独冻结 OpenAPI、UI 与追踪契约。开发阶段不得重新解释本文件的核心语义。

## 1. 业务目标与范围

将 P0 同步 Mock 正文生成扩展为真实异步正文生成：用户以当前 `ContentVersion` 为基线预检、二次确认创建 `WorkflowRun(stage=content_generation)`、消费外部工作流结果为候选版本、比较并明确切换当前版本。刷新或重新进入必须从持久化 Summary 恢复。

Iteration 16 包含：无副作用预检、真实 Run、输出校验、候选创建、候选查看/比较、显式设为当前、Runtime Retry、结果消费 Retry、失败与未配置恢复。它不包含自动提交审核、自动触发重写、自动将候选设为当前、修改 Iteration 17 审核或 Iteration 18 重写、新正文聚合、删除 P0 Mock，或以 P0 Mock 作为真实链路验收。

## 2. 不可变业务结论

- `ContentItem` 是正文逻辑聚合，`ContentVersion` 是不可变正文版本，`ContentItem.current_version_id` 是唯一当前指针；`workId = ContentItem.id`。
- 仅已确认的 `ChapterPlan` 对应的 ContentItem 可生成。未存在 ContentItem 时复用既有幂等创建入口及空白 v1。
- source ContentVersion 是生成输入基线；candidate ContentVersion 是工作流输出；source WorkflowRun 是候选来源 Run。
- 候选固定 `source=workflow_generated`，默认非当前，保存 `source_content_version_id`、`source_content_version_version`、`source_workflow_run_id`；一个 Run 至多一个候选，不覆盖或删除历史版本。
- 相同 `projectId + stage + subjectType + subjectId` 只允许一个 active Run；本迭代 subjectType 固定 `content_item`、subjectId 固定 ContentItem.id。不同 ContentItem 可以并行。
- Runtime succeeded 只表示外部执行成功；只有输出校验和候选写入成功才是候选成功。候选绝不自动替换当前版本。

## 3. 用户闭环

正文编辑器 → 选择当前版本作为基线 → 填写补充要求与上下文选项 → 无副作用预检 → 查看报告 → 二次确认 → queued/running Run → 输出校验 → 原子创建候选 → 查看/比较候选 → 用户明确设为当前版本 → 继续编辑或按 Iteration 17 对明确当前版本提交审核。

失败、刷新、取消、Runtime Retry 与结果消费 Retry 的闭环见 `closed-loop.md`；状态、stale 和领域边界见 `business-rules.md`。

## 4. 相邻迭代边界

复用 Iteration 13 的 `content_generation` 项目绑定，禁止业务页临时换工作流；复用 Iteration 14 的 Runtime Run/Event/Cancel/Retry 和脱敏快照，不建立第二套 Run。Iteration 15 的 `confirmed` ChapterPlan 是正文生产门槛。Iteration 17 审核固定绑定明确 `ContentVersion.id`，且 Runtime stage 为 `review`；本迭代不修改其文档或业务。Iteration 18 重写仍是独立版本创建动作。

## 5. 固定开发任务（9 个）

1. CF-16-01A｜业务、数据模型和事务契约冻结
2. CF-16-01B｜OpenAPI、UI 和追踪契约冻结
3. CF-16-02A｜Migration、Domain 和 Repository
4. CF-16-02B｜预检、Run 创建和 Summary
5. CF-16-02C｜结果消费、候选和版本切换
6. CF-16-03A｜编辑器、生成抽屉和预检
7. CF-16-03B｜运行状态、候选闭环和 UI 收口
8. CF-16-04A｜真实 n8n 正常闭环
9. CF-16-04B｜异常、安全、回归和收口

人工验收、审核包和完整代码审核不计入上述 9 个 Agent 开发任务。详细依赖见 `development-plan.md`。

## 6. 完成定义

- 预检无副作用；同正文仅一个活跃 Run，不同正文可并行；刷新恢复真实状态。
- 成功 Run 最多产生一个非当前候选；非法输出或消费事务失败零候选写入。
- 当前切换使用 CAS；源基线过期时保留候选供查看/比较，返回 `candidate_source_stale`，没有强制覆盖。
- 结果消费 Retry 仅重试领域消费，绝不重新调用外部工作流。
- 数据库仅在唯一开发数据库上通过新增 Iteration 16 Migration 向前演进；历史 Migration 不变，不要求空库、历史回滚或 downgrade。
