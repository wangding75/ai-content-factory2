# Iteration 16 — 真实正文生成 — 验收标准

## 1. 契约和边界

| ID | 场景 | 通过标准 |
|---|---|---|
| I16-AC01 | P0 聚合复用 | 使用 ContentItem/ContentVersion；不新增 Work 或候选正文表。 |
| I16-AC02 | Runtime 复用 | 只使用 Iteration 14 WorkflowRun/Event/Retry/Cancel，不建立第二套 Run。 |
| I16-AC03 | 路由 | 编辑器正式路由为 `/projects/{projectId}/works/{workId}`，workId 等于 ContentItem.id。 |
| I16-AC04 | 相邻边界 | 不修改章节规划、不自动审核、不实现重写。 |

## 2. 前置条件和预检

| ID | 场景 | 通过标准 |
|---|---|---|
| I16-AC05 | 已确认章节 | 只有 confirmed ChapterPlan 对应 ContentItem 可通过。 |
| I16-AC06 | 无副作用 | 预检不创建 Run、不外呼、不创建版本、不修改正文。 |
| I16-AC07 | 输入摘要 | 返回源版本、目标版本号、工作流、上下文计数、检查、警告和阻断。 |
| I16-AC08 | Token | 绑定 actor/item/source/input/config，10 分钟有效，创建时复核。 |
| I16-AC09 | 未配置 | 缺少绑定或执行连接不可用时阻断并提供配置入口。 |

## 3. Run 创建和恢复

| ID | 场景 | 通过标准 |
|---|---|---|
| I16-AC10 | 专用创建 | 服务端固定 stage=content_generation、manual、content_item subject。 |
| I16-AC11 | 幂等 | 同 Key 同载荷返回首次 Run；不同载荷冲突；不重复创建。 |
| I16-AC12 | 活跃锁 | 同 ContentItem 只有一个 queued/running Run；不同正文允许并行。 |
| I16-AC13 | 刷新恢复 | queued/running/succeeded/failed/not-configured 均从持久化数据恢复。 |
| I16-AC14 | 详情 | 详情和流程中心使用现有 Runtime 路由。 |

## 4. 候选版本和原子消费

| ID | 场景 | 通过标准 |
|---|---|---|
| I16-AC15 | 成功结果 | 一个 Run 只创建一个 `workflow_generated` 非当前候选版本。 |
| I16-AC16 | 当前不变 | 候选创建后 current_version_id、当前正文和旧版本不变。 |
| I16-AC17 | 来源追溯 | 候选可追溯源 ContentVersion、Runtime Run、输入和配置快照。 |
| I16-AC18 | 非法输出 | Schema 任一字段非法时零版本写入。 |
| I16-AC19 | 事务失败 | 零部分版本写入，并记录安全 result_consumption_failed Event。 |
| I16-AC20 | 消费幂等 | 同 Run 重复消费返回既有候选，不新增版本或序号。 |
| I16-AC21 | 消费重试 | 只重试领域事务，不重复调用外部工作流。 |

## 5. 设为当前版本

| ID | 场景 | 通过标准 |
|---|---|---|
| I16-AC22 | 候选查看 | 版本列表和详情准确标记当前/候选，正文可比较。 |
| I16-AC23 | CAS | expectedCurrentVersionId 和 expectedCurrentVersion 均正确时更新 current pointer。 |
| I16-AC24 | 基线过期 | 当前版本变化后禁止强制设为当前，返回 candidate_source_stale。 |
| I16-AC25 | 领域终态 | 成功后 ContentItem 为 draft、reviewed_at 清空，旧审核和版本保留。 |
| I16-AC26 | 命令幂等 | 同 Key 重放返回首次结果，不重复变更。 |

## 6. UI 定稿验收

- [ ] 11 个 `screen.png` 和 `code.html` 一一对应并可打开；
- [ ] 11 项均使用 `/projects/{projectId}/works/{workId}`，抽屉和状态不创建伪路由；
- [ ] 固定 ACF 桌面框架、顶部导航、三栏工作区和底部状态栏；
- [ ] 章节目标、故事情报和素材库三种上下文状态完整；
- [ ] 生成抽屉展示工作流、源版本、新版本、前置检查和补充要求；
- [ ] queued、running、succeeded、failed、not-configured 状态完整；
- [ ] 成功后明确显示“候选版本”，不误写为当前版本；
- [ ] 候选可以打开、比较和设为当前；
- [ ] 失败时当前正文保持可见且不被覆盖；
- [ ] 用户可见业务文案为中文；Material Symbols 名称、snake_case、原始枚举和错误码不得裸露；
- [ ] 外部图标字体失败时仍不显示图标英文名称；
- [ ] 不要求像素级一致，但主要结构、信息层级和操作位置必须一致；
- [ ] 长正文、长标题和长上下文无横向溢出或逐字换行；
- [ ] 页面、抽屉和弹层滚动到底后无遗漏操作。

## 7. 安全和工程门禁

- [ ] 主 OpenAPI 解析、引用和 Contract Test 通过；
- [ ] Migration 在空库和现有开发库终态验证通过；
- [ ] Domain、Repository、Service、HTTP、幂等、事务和并发测试通过；
- [ ] 前端组件、路由、状态、中文文案和 API 测试通过；
- [ ] 真实 n8n 生成、结果消费、打开候选、设为当前 E2E 通过；
- [ ] 敏感信息、错误封装和日志检查通过；
- [ ] P0 保存草稿、Mock 生成、审核、重写和作品查询无回归；
- [ ] 先局部、再分组、最后完整门禁；
- [ ] 人工 UI 验收、独立 Code Review、Commit、Push 和 clean Git 状态完成。
