# Iteration 16 — 真实正文生成与候选版本闭环

**文档状态：`review_ready_cf_16_00`。** UI 已按 2026-07-28 Stitch 定稿冻结；业务、数据和 OpenAPI 作为 Iteration 16 契约评审基线。评审通过后再进入后端和前端开发，不得在开发阶段重新解释核心语义。


## 1. 业务目标

把 P0 的同步 Mock 正文生成升级为真实异步正文生成：

1. 用户从已确认章节对应的正文编辑器发起生成；
2. 系统读取项目 `content_generation` 工作流绑定并执行无副作用预检；
3. 用户确认后创建 `WorkflowRun(stage=content_generation)`；
4. 外部输出通过 Schema 校验和领域事务后创建新的候选 `ContentVersion`；
5. 当前版本在用户明确选择前保持不变；
6. 用户查看候选、与当前版本比较并设为当前版本；
7. 失败、刷新、未配置和结果消费重试均可恢复。

## 2. 核心结论

- 正文聚合继续使用 P0 的 `ContentItem`，不新增持久化 `Work`。
- 正式编辑器路由冻结为 `/projects/{projectId}/works/{workId}`，其中 `workId` **等于** `ContentItem.id`。
- 一个 `ContentItem` 只有一个 `current_version_id`，但可保留多个非当前候选版本。
- 真实生成成功创建 `source=workflow_generated` 的新 `ContentVersion`，不得更新或覆盖当前版本正文。
- 候选版本记录生成时的源版本 ID、源版本乐观锁号和 `WorkflowRun`；用户设置当前版本时执行 compare-and-swap。
- 同一正文同一时间最多一个 queued/running 的 `content_generation` Run；不同正文允许并行。
- `WorkflowRun.succeeded` 与领域结果消费成功分离；结果消费失败不产生部分版本。
- 生成正文、提交审核和重写是三个独立动作。Iteration 16 不自动提交 Iteration 17 审核。

## 3. 关联迭代评审

| 关联范围 | 继承内容 | Iteration 16 的处理 |
|---|---|---|
| P0 Iteration 05 章节规划 | 只有 `confirmed` ChapterPlan 可进入正文生产 | 预检和创建 Run 都重新校验；未确认时阻断 |
| P0 Iteration 06 编辑器与审核 | `ContentItem`、`ContentVersion`、保存当前草稿、Mock 生成入口 | 复用实体和编辑器；保留 Mock API，不用它模拟真实链路 |
| P0 Iteration 07 重写与作品 | 不可变版本历史、`workId=ContentItem.id`、版本列表/详情 | 复用版本历史和作品读模型；真实生成候选不自动成为当前版本 |
| ADR-0004 | 重写不覆盖内容版本 | 扩展为真实生成同样不覆盖当前版本 |
| Iteration 12 | 全局 WorkflowConnection / WorkflowConfiguration | 复用连接、配置、Schema 和密钥脱敏 |
| Iteration 13 | 项目四阶段绑定，正文阶段为 `content_generation` | 业务页面不临时切换工作流；未绑定引导项目设置 |
| Iteration 14 | Runtime Run、Event、Cancel、Retry、幂等和配置快照 | 不建立第二套 Run；新增 subject 关联和结果消费事件 |
| Iteration 14.5 | 本地 n8n 可复现集成基线 | 最终真实联调复用既有执行适配层，不在本轮重建 Transport/Worker |
| Iteration 15 | 只有已确认章节具备正文生产资格 | 以 `ChapterPlan` 和 `ContentItem` 为输入边界；不修改章节规划候选 |
| Iteration 17 | 审核固定 ContentVersion | Iteration 16 只创建/切换版本，审核必须由用户针对明确当前版本发起 |
| Iteration 18 | 重写创建新 ContentVersion | 不复用“生成正文”入口做重写；来源和阶段分别追踪 |
| Iteration 19 | 第二闭环最终联调 | Iteration 16 输出是其正文生成段的权威输入 |

### 3.1 发现并修正的旧文档问题

- 旧 Iteration 16 只列 5 个通用 Frame，与本次 11 张定稿 UI 不一致。
- 旧 API 写成 `/api/v1/chapters/{chapterId}/content-generation-runs`，但当前领域主键和正式作品路由都以 `ContentItem` 为正文聚合，应改为 `content-items`。
- 旧数据模型使用 `sourceType`，当前主契约字段是 `ContentVersion.source`，不得增加同义字段。
- 旧文档没有“候选设为当前版本”正式写 API，也没有解决并发覆盖。
- 旧文档没有预检、结果消费幂等、消费失败恢复和同正文活跃 Run 锁。
- Iteration 17 草案仍写 `content_review`，而当前 Runtime 固定 stage 为 `review`；本轮只记录该关联问题，不越界修改 Iteration 17。

## 4. 用户闭环

正文编辑器
→ 查看章节目标、故事情报和素材上下文
→ 点击“生成正文”
→ 查看工作流、目标版本和前置检查
→ 可填写补充要求
→ 无副作用预检通过
→ 用户确认创建 Run
→ queued / running
→ 输出校验
→ 原子创建候选版本
→ 打开候选版本
→ 与当前版本比较
→ 明确设为当前版本
→ 继续编辑或提交 Iteration 17 审核。

异常闭环见 `closed-loop.md`，状态机见 `business-rules.md`。

## 5. UI 定稿

本轮定稿共 11 个 Frame，全部复用同一正式编辑器路由；抽屉和异步状态不是独立路由。清单见：

- `ui-scope.md`
- `ui-manifest.json`
- `prototype-source-mapping.md`
- `ui-contract-traceability.md`
- `ui/DESIGN.md`

## 6. API 与数据范围

新增：

- 正文生成预检；
- 正文生成专用 Run 创建；
- 正文生成工作区 Summary；
- 结果消费失败的专用重试；
- 设为当前版本命令。

扩展：

- `ContentVersionSource` 增加 `workflow_generated`；
- `ContentVersion` 增加源版本和源 Runtime Run 追溯；
- Runtime Run 增加可选 subject 关联；
- Runtime Event 增加领域结果消费成功/失败事件。

详细定义见 `api-scope.yaml`、`data-model.md` 和主 OpenAPI。

## 7. 开发任务顺序

1. CF-16-01A：业务规则与状态机冻结；
2. CF-16-01B：数据模型、索引和事务冻结；
3. CF-16-01C：OpenAPI、Schema、错误码与 UI 追踪冻结；
4. CF-16-02A：Runtime subject 与 ContentVersion 来源数据层；
5. CF-16-02B：预检、Run 创建与工作区 Summary；
6. CF-16-02C：输出校验、原子候选版本消费和消费重试；
7. CF-16-02D：设为当前版本与版本查询收口；
8. CF-16-03A：编辑器三栏上下文与生成抽屉；
9. CF-16-03B：queued/running/succeeded/failed/not-configured 状态；
10. CF-16-03C：候选版本查看、比较、设为当前和中文文案回归；
11. 人工 UI 验收；
12. CF-16-04A：真实 API 与 n8n 联调；
13. CF-16-04B：E2E、安全、回归和 Git 收口。

完整任务见 `development-plan.md`。

## 8. 明确不在范围

- 不修改 ChapterPlan 候选和确认流程；
- 不自动覆盖当前正文；
- 不自动提交审核；
- 不实现真实审核或真实重写；
- 不新增正文候选表；
- 不在编辑器中临时选择 WorkflowConfiguration；
- 不建设 n8n 编辑器、节点映射器、多实例路由或费用大盘；
- 不删除或替换 P0 Mock API；
- 不更新 Iteration 17～19 的业务文档，后续以本轮冻结结果为输入单独修订。

## 9. 完成定义

- [ ] 11 个定稿 Frame、路由、状态和操作一一追踪；
- [ ] ContentItem、ContentVersion 和 WorkflowRun 不出现平行模型；
- [ ] 预检无副作用；
- [ ] 成功只创建一个非当前候选版本；
- [ ] 非法输出和事务失败零版本写入；
- [ ] 当前版本并发变化不会被静默覆盖；
- [ ] 页面刷新可恢复 queued/running/succeeded/failed/not-configured；
- [ ] 版本历史、保存草稿、Mock 生成、审核和重写无回归；
- [ ] 中文业务文案、图标和状态映射通过；
- [ ] OpenAPI、数据模型、UI 和实现一致；
- [ ] 真实联调、E2E、安全和 Git 门禁通过。
