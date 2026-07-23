# Iteration 15 — UI Scope

## 1. UI 基线

- 来源：上传的 `stitch_acf_iteration_15_full_exact_shell_migration.zip`。
- 设计权威：Stitch Project `10082349329835651109`。
- 应用壳层：必须复用 ACF 当前桌面框架；禁止复制第二套侧边栏或顶部导航。
- 原型规模：21 个 Frame；旧版 5 个 Iteration 15 Frame 被本清单替代。
- 详细源目录映射见 `prototype-source-mapping.md`。

## 2. Frame 清单

| Frame ID | 区域 | 类型 | 业务用途 | Canonical |
|---|---|---|---|---|
| `P15_C1_RUNNING_MAINLINE_EXPANSION` | 章节规划工作区 | page_state | 主线剧情扩展运行中 | 状态/数据变体 |
| `P15_C1_RUNNING_PARTIAL_RANGE` | 章节规划工作区 | page_state | 局部范围生成运行中（数据样例 A） | 是 |
| `P15_C1_RUNNING_FULL_PLAN` | 章节规划工作区 | page_state | 完整章节规划运行中 | 状态/数据变体 |
| `P15_C1_RUNNING_PARTIAL_ETA` | 章节规划工作区 | page_state | 局部范围生成中（剩余时间） | 状态/数据变体 |
| `P15_C1_RUNNING_PARTIAL_VALIDATING` | 章节规划工作区 | page_state | 局部范围生成中（结果校验） | 状态/数据变体 |
| `P15_C1_RUNNING_DATA_VARIANT` | 章节规划工作区 | fixture_variant | 不同章节状态与统计数据样例 | 状态/数据变体 |
| `P15_C1_FAILED_ATOMIC` | 章节规划工作区 | page_state | 生成失败且候选零写入 | 状态/数据变体 |
| `P15_C1_NOT_CONFIGURED` | 章节规划工作区 | page_state | 未配置工作流状态 | 状态/数据变体 |
| `P15_C2_GENERATION_SETTINGS` | 生成章节规划 | drawer | 生成参数抽屉 | 是 |
| `P15_C2_PREFLIGHT_PROGRESS` | 生成章节规划 | dialog | 预检进度弹窗 | 状态/数据变体 |
| `P15_C3_PREFLIGHT_PASS` | 生成章节规划 | dialog | 预检通过报告 | 是 |
| `P15_C3_PREFLIGHT_BLOCKED` | 生成章节规划 | dialog | 预检阻断报告 | 是 |
| `P15_C3_RUN_CREATED` | 生成章节规划 | dialog | 任务创建成功 | 是 |
| `P15_C4_CANDIDATE_BATCH_LIST` | 候选批次 | page | 候选批次列表 | 是 |
| `P15_C5_CANDIDATE_BATCH_DETAIL` | 候选批次 | page | 候选批次详情与候选列表 | 是 |
| `P15_C6_CANDIDATE_EDIT_DRAWER` | 候选批次 | drawer | 编辑章节候选 | 是 |
| `P15_C7_CANDIDATE_COMPARE_DIALOG` | 候选批次 | dialog | 当前章节与新候选差异对比 | 是 |
| `P15_C8_BATCH_ADOPT_DIALOG` | 候选批次 | dialog | 批量采用候选确认 | 是 |
| `P15_C9_BATCH_ABANDON_DIALOG` | 候选批次 | dialog | 放弃候选批次确认 | 是 |
| `P15_C10_STALE_CONFLICT_DIALOG` | 候选批次 | dialog | 候选基线过期冲突处理 | 是 |
| `P15_S1_STORYLINE_RELATION_READONLY` | 故事线 | page | 章节关联统计只读修正版 | 是 |

## 3. 页面与组件归并

### 3.1 章节规划工作区

`P15_C1_*` 是同一业务页面的状态和数据变体，不得开发为多个路由。页面需要由真实数据驱动：

- 当前章节列表与统计；
- 活跃 Run 状态条；
- 未配置状态；
- 原子失败状态；
- 完整、局部和重点故事线生成摘要；
- 批量确认、故事线标记和删除操作。

### 3.2 生成与预检

- `P15_C2_GENERATION_SETTINGS`：右侧抽屉。
- `P15_C2_PREFLIGHT_PROGRESS`：短时预检进度弹窗。
- `P15_C3_PREFLIGHT_PASS`：通过/警告报告。
- `P15_C3_PREFLIGHT_BLOCKED`：阻断报告。
- `P15_C3_RUN_CREATED`：创建成功弹窗。

预检进度可以由真实步骤或安全的顺序状态驱动，但不得伪造 Run；只有用户最终确认后才创建 Run。

### 3.3 候选批次

- 批次列表与批次详情为页面。
- 编辑为抽屉。
- 差异、批量采用、放弃和过期冲突为弹窗。
- 候选处理完成后仍保留批次、Run 和来源追溯。

### 3.4 故事线

`P15_S1_STORYLINE_RELATION_READONLY` 只修正故事线与章节职责：

- 故事线保持递归树；
- 章节关联统计只读；
- 不增加可编辑章节范围；
- 具体章节落点由章节规划产生。

## 4. 固定视觉规则

- 复用当前 274px 左侧栏、顶栏、项目头和项目 Tab。
- 项目 Tab 中“章节”为主页面 active；故事线修正版中“故事线”为 active。
- 内容区浅灰背景、白色边框卡片、克制阴影、靛蓝主操作。
- 保留表格列、卡片数量、树层级、警告详情和按钮顺序。
- 抽屉与弹窗类型不得互换。
- 不得用简化摘要卡替换候选表、差异表或预检报告。

## 5. 文案与术语

默认中文环境：

- 首页、项目、素材、作品、流程、设置；
- 排队中、运行中、已成功、已失败、已取消；
- 待处理、已采用、已放弃、存在冲突、无变化；
- 采用候选、确认章节、进入正文生产是三个不同动作。

`Run ID`、Schema、Workflow、模型名和配置版本等技术标识可保留英文。源 HTML 中的 `Batch Candidates List`、`Ch`、`RUNNING` 等不构成最终中文文案冻结。

## 6. 必须实现的 UI 状态

- Loading：工作区、批次列表、批次详情、候选详情和预检。
- Empty：无当前章节、无候选批次、无筛选结果、未配置。
- Error：列表错误、详情错误、预检失败、运行失败、结果入库失败。
- Warning：预检可继续警告、部分故事线缺少伏笔。
- Conflict：活跃 Run 冲突、候选基线过期。
- Success：预检通过、Run 创建、候选采用、批次放弃。
- Disabled：不满足业务门槛的采用、确认、删除和正文入口。

## 7. 原型使用规则

1. `screen.png` 负责视觉、层级、密度、状态和动作位置。
2. `code.html` 负责组件结构和内容细节参考，不直接复制其应用壳层。
3. `P15_C1_RUNNING_PARTIAL_RANGE` 是章节工作区的 canonical 视觉基线；其他 `P15_C1_*` 用于状态和数据验收。
4. `P15_C1_RUNNING_DATA_VARIANT` 中的 Basic Plan 与源 `DESIGN.md` 的 Pro Plan 不一致，开发必须复用真实当前 App Shell，不以该样例改变全局用户卡。
5. 任何字段或按钮语义冲突，以 `iteration-plan.md`、`closed-loop.md`、冻结 OpenAPI 和本文件为准。
