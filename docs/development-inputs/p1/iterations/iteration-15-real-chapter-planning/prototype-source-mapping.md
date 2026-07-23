# Iteration 15 原型源映射

## 1. 来源

- 上传包：`stitch_acf_iteration_15_full_exact_shell_migration.zip`
- Stitch Project：`10082349329835651109`
- 原始设计说明：`ui/source/DESIGN.md`
- 原始截图：21 张
- 原始 HTML：21 个

## 2. 映射

| 源目录 | Frame ID | 类型 | 业务含义 |
|---|---|---|---|
| `c1_1` | `P15_C1_RUNNING_MAINLINE_EXPANSION` | page_state | 主线剧情扩展运行中 |
| `c1_2` | `P15_C1_RUNNING_PARTIAL_RANGE` | page_state | 局部范围生成运行中（数据样例 A） |
| `c1_3` | `P15_C1_RUNNING_FULL_PLAN` | page_state | 完整章节规划运行中 |
| `c1_4` | `P15_C1_RUNNING_PARTIAL_ETA` | page_state | 局部范围生成中（剩余时间） |
| `c1_5` | `P15_C1_RUNNING_PARTIAL_VALIDATING` | page_state | 局部范围生成中（结果校验） |
| `c1_6` | `P15_C1_RUNNING_DATA_VARIANT` | fixture_variant | 不同章节状态与统计数据样例 |
| `c1_f` | `P15_C1_FAILED_ATOMIC` | page_state | 生成失败且候选零写入 |
| `c1_n` | `P15_C1_NOT_CONFIGURED` | page_state | 未配置工作流状态 |
| `c2` | `P15_C2_GENERATION_SETTINGS` | drawer | 生成参数抽屉 |
| `c2_p` | `P15_C2_PREFLIGHT_PROGRESS` | dialog | 预检进度弹窗 |
| `c3_a` | `P15_C3_PREFLIGHT_PASS` | dialog | 预检通过报告 |
| `c3_b` | `P15_C3_PREFLIGHT_BLOCKED` | dialog | 预检阻断报告 |
| `c3_c` | `P15_C3_RUN_CREATED` | dialog | 任务创建成功 |
| `c4` | `P15_C4_CANDIDATE_BATCH_LIST` | page | 候选批次列表 |
| `c5` | `P15_C5_CANDIDATE_BATCH_DETAIL` | page | 候选批次详情与候选列表 |
| `c6` | `P15_C6_CANDIDATE_EDIT_DRAWER` | drawer | 编辑章节候选 |
| `c7` | `P15_C7_CANDIDATE_COMPARE_DIALOG` | dialog | 当前章节与新候选差异对比 |
| `c8` | `P15_C8_BATCH_ADOPT_DIALOG` | dialog | 批量采用候选确认 |
| `c9` | `P15_C9_BATCH_ABANDON_DIALOG` | dialog | 放弃候选批次确认 |
| `c10` | `P15_C10_STALE_CONFLICT_DIALOG` | dialog | 候选基线过期冲突处理 |
| `s1` | `P15_S1_STORYLINE_RELATION_READONLY` | page | 章节关联统计只读修正版 |

## 3. 旧版替换关系

| 旧 Frame | 处理 | 新基线 |
|---|---|---|
| `C1_CHAPTER_PLANNING_V2` | 退出 Iteration 15 开发基线 | `P15_C1_*` 工作区状态组 |
| `C2_GENERATE_CHAPTER_PLAN_DRAWER_V2` | 退出 Iteration 15 开发基线 | `P15_C2_*` + `P15_C3_*` |
| `STATE_TASK_RUNNING_BAR` | 不再作为独立通用 Spec 直接实现 | `P15_C1_RUNNING_*` 页面内状态条 |
| `STATE_TASK_FAILED_NOTICE` | 不再作为独立通用 Spec 直接实现 | `P15_C1_FAILED_ATOMIC` |
| `STATE_NOT_CONFIGURED_EMPTY` | 不再作为独立通用 Spec 直接实现 | `P15_C1_NOT_CONFIGURED` |

旧文件不应残留在 Iteration 15 `ui/frames` 目录，避免开发 Agent 误选。

## 4. Canonical 与变体

- 工作区 canonical：`P15_C1_RUNNING_PARTIAL_RANGE`。
- 其他 C1 Frame 用于模式、进度、失败、未配置和数据状态验收。
- 抽屉、弹窗和候选页面均为独立业务状态，不得合并删除。
- `P15_S1_STORYLINE_RELATION_READONLY` 是对 P0 故事线职责的修正，不是新故事线管理重构。
