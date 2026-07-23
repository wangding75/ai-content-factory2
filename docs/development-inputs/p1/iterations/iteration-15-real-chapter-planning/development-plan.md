# Iteration 15 开发计划

## 1. 总体结论

- 启动状态：`READY_FOR_CONTRACT_FREEZE`
- UI 输入：已更新为 21 Frame
- P0 继承边界：已明确
- 真实生成最终验收依赖：`CF-14-N8N-Integration`
- 建议任务数：18 个开发任务 + 1 次人工 UI 验收
- 固定顺序：契约冻结 → 后端 → 前端/UI → 联调与收口

## 2. 第一阶段：契约冻结

| 任务 | 名称 | 主要输出 | 前置 |
|---|---|---|---|
| CF-15-01A | 业务语义与状态机冻结 | 生成模式、预检、活跃锁、Batch/Candidate/Revision 状态、采用与确认边界 | Iteration 14 completed |
| CF-15-01B | 数据模型与事务冻结 | 表、字段、索引、外键、修订历史、结果入库与采用事务 | 01A |
| CF-15-01C | OpenAPI 与错误契约冻结 | Preflight、Run、Batch、Candidate、Adopt、Abandon、Summary API | 01A–01B |
| CF-15-01D | UI 状态与契约追踪冻结 | 21 Frame、字段/API/状态映射和验收矩阵 | 01C |

第一阶段完成标准：

- 主 OpenAPI 成为唯一契约；
- 数据模型与 Migration 设计冻结；
- UI 不依赖未定义字段；
- CF-14-N8N-Integration 的输入/输出边界明确；
- 无需开始外部执行开发即可进入核心数据层开发。

## 3. 第二阶段：后端开发

| 任务 | 名称 | 主要内容 | 前置 |
|---|---|---|---|
| CF-15-02A | Candidate Batch 数据层 | Migration、Batch/Candidate/Revision Domain 与 Repository | 01D |
| CF-15-02B | 预检服务 | 输入构建、故事线树、配置摘要、阻断/警告、Token | 02A |
| CF-15-02C | Run 编排 | 章节专用创建入口、Runtime 复用、活跃锁和幂等 | 02B |
| CF-15-02D | 结果 Schema 与原子入库 | 规范化输出、完整校验、Run→Batch 唯一映射 | 02A–02C |
| CF-15-02E | Batch/Candidate 查询与编辑 | 列表、详情、筛选、候选编辑、重新对比 | 02D |
| CF-15-02F | 采用、批量采用与修订历史 | 新增/替换、冲突、逐项结果、Revision | 02E |
| CF-15-02G | 放弃、统计和后端总门禁 | Candidate/Batch 放弃、Summary、HTTP、全量测试 | 02F |

后端阶段要求：

- 每个任务开发、自检、Commit、Push；
- 先专项再分组再全量；
- 禁止 gofmt 和 `go fmt`；
- 只验收当前数据库终态，不要求历史回滚；
- 不在业务模块实现通用 n8n Worker。

## 4. 第三阶段：前端与 UI

| 任务 | 名称 | Frame/范围 | 前置 |
|---|---|---|---|
| CF-15-03A | 章节工作区升级 | `P15_C1_*`，当前章节、统计、运行状态、失败、未配置 | 02C、02G API 稳定 |
| CF-15-03B | 生成与预检 | `P15_C2_*`、`P15_C3_*` | 03A |
| CF-15-03C | 候选批次列表 | `P15_C4_CANDIDATE_BATCH_LIST` | 02E |
| CF-15-03D | 批次详情、编辑和对比 | `P15_C5`、`P15_C6`、`P15_C7` | 03C |
| CF-15-03E | 采用、放弃和冲突 | `P15_C8`、`P15_C9`、`P15_C10` | 03D |
| CF-15-03F | 故事线职责修正与回归 | `P15_S1_STORYLINE_RELATION_READONLY` | 03A |
| CF-15-03G | 前端统一状态与人工 UI 验收包 | Loading/Empty/Error/Success、截图、浏览器检查 | 03A–03F |

前端阶段要求：

- 复用当前 App Shell 和组件；
- 不开发多个 C1 路由；
- 先 Mock/fixture 覆盖全部状态，再接真实 API；
- B 类页面完成后统一人工 UI 验收；
- 人工验收通过前不进入最终联调收口。

## 5. 第四阶段：联调与收口

| 任务 | 名称 | 主要内容 | 前置 |
|---|---|---|---|
| CF-15-04A | 真实 API 联调 | Preflight、CreateRun、Batch/Candidate 全部真实 API | 02G、03G |
| CF-15-04B | 真实执行与候选闭环 | 依赖 CF-14-N8N-Integration，真实 Run→结果→Batch→采用→确认 | 04A、执行集成完成 |
| CF-15-04C | 最终 E2E、安全与代码审查 | 失败零写入、幂等、冲突、回归、浏览器、独立 Review | 04B |
| CF-15-04D | 状态收口 | 文档、验收、状态文件、Commit、Push、Iteration completed | 人工 UI + 04C |

## 6. 关键里程碑

1. M1：CF-15-01D PASS — 契约可开发。
2. M2：CF-15-02G PASS — 后端候选域完整。
3. M3：CF-15-03G PASS — UI 人工验收通过。
4. M4：CF-15-04B PASS — 真实生成闭环通过。
5. M5：CF-15-04D PASS — Iteration 15 完成。

## 7. 第一项执行任务

第一个任务应为：

`CF-15-01A：业务语义与状态机冻结`

该任务只冻结：

- 三种生成模式；
- 预检阻断/警告；
- 一个项目章节规划活跃 Run 规则；
- Batch/Candidate/Revision 状态；
- 采用、批量采用、放弃和冲突；
- 采用与确认的边界；
- 与 CF-14-N8N-Integration 的职责接口。

不得直接开发代码或 Migration。
