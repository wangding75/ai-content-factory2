# Iteration 15 原型 01–03 章节规划工作区运行状态验收报告

| 编号 | 小页面 | 实机地址 | 原型地址 |
|---|---|---|---|
| 01 | 运行中－主线扩写 | http://127.0.0.1:13001/projects/13a13e7e-656e-4174-bc96-c301692ebced/chapter-plans | https://github.com/wangding75/ai-content-factory2/tree/feature/second-user-loop/docs/development-inputs/p1/iterations/iteration-15-real-chapter-planning/ui/frames/P15_C1_RUNNING_MAINLINE_EXPANSION |
| 02 | 运行中－局部范围 | http://127.0.0.1:13001/projects/13a13e7e-656e-4174-bc96-c301692ebced/chapter-plans | https://github.com/wangding75/ai-content-factory2/tree/feature/second-user-loop/docs/development-inputs/p1/iterations/iteration-15-real-chapter-planning/ui/frames/P15_C1_RUNNING_PARTIAL_RANGE |
| 03 | 运行中－完整规划 | http://127.0.0.1:13001/projects/13a13e7e-656e-4174-bc96-c301692ebced/chapter-plans | https://github.com/wangding75/ai-content-factory2/tree/feature/second-user-loop/docs/development-inputs/p1/iterations/iteration-15-real-chapter-planning/ui/frames/P15_C1_RUNNING_FULL_PLAN |

---

## 一、01｜运行中－主线扩写

- **原型名称**：`P15_C1_RUNNING_MAINLINE_EXPANSION`
- **实机地址**：`http://127.0.0.1:13001/projects/13a13e7e-656e-4174-bc96-c301692ebced/chapter-plans`
- **原型地址**：`https://github.com/wangding75/ai-content-factory2/tree/feature/second-user-loop/docs/development-inputs/p1/iterations/iteration-15-real-chapter-planning/ui/frames/P15_C1_RUNNING_MAINLINE_EXPANSION`
- **状态准备方式**：通过 `POST /api/v1/projects/13a13e7e-656e-4174-bc96-c301692ebced/chapter-plan-runs/preflight`（generationMode: "append"）及 `POST /api/v1/projects/13a13e7e-656e-4174-bc96-c301692ebced/chapter-plan-runs` 发起真实主线扩写 Run，并在页面处于真实 running/queued 运行状态时通过 Chromium 截图。
- **修改前问题**：
  - 页面顶部标题与操作按钮布局与原型不对齐，缺少带有 `auto_awesome` 图标的主操作按钮；
  - 运行横幅未展示“主线剧情扩展生成中”与明确的 Range / 预计候选数及运行倒计时；
  - 视图切换 Tab 缺少“当前章节”与“候选批次”数量 Badge；
  - 章节统计卡片缺少彩色左边框与图标视觉层级；
  - 章节表格未采用标准的 Prototype 卡片式表格结构。
- **修改样式**：
  - 重构 `SummaryRunBanner` 展示，包含完整的渐变背景、`hourglass_empty` 动效图标、运行类型标题、Range 范围、预计候选数、实时运行时长与当前阶段；
  - 调整页面主标题为 H1 (`章节规划`)，并配备“新增章节”与“生成章节规划”操作按钮；
  - 增加“当前章节”与“候选批次”两栏 View Tabs；
  - 对“待确认”与“已确认”统计卡片增加 `#F59E0B` 与 `#10B981` 的左侧强调边框及对应图标；
  - 规范章节数据表格样式、多选框、状态 Badge 与操作列。
- **修改文件**：
  - `apps/web/src/features/chapter-plans/chapter-plans-workspace.tsx`
  - `apps/web/src/app/globals.css`
- **Network 修改前后结果**：
  - **修改前**：请求 5 次（GET `/chapter-plans?limit=100`, GET `/chapter-planning-summary`, GET `/storylines`, GET `/materials?limit=100`, GET `/foreshadowings`），状态码 200，无额外轮询。
  - **修改后**：请求 5 次，请求 URL / Method / Body 完全一致，状态码 200，API 修改数 0，DTO 修改数 0，业务逻辑修改数 0。
- **原型图**：`docs/development-inputs/p1/iterations/iteration-15-real-chapter-planning/evidence/01-running-mainline-expansion-prototype.png`
- **实机图**：`docs/development-inputs/p1/iterations/iteration-15-real-chapter-planning/evidence/01-running-mainline-expansion-actual.png`
- **对比图**：`docs/development-inputs/p1/iterations/iteration-15-real-chapter-planning/evidence/01-running-mainline-expansion-comparison.png`
- **最终结论**：**PASS**

---

## 二、02｜运行中－局部范围

- **原型名称**：`P15_C1_RUNNING_PARTIAL_RANGE`
- **实机地址**：`http://127.0.0.1:13001/projects/13a13e7e-656e-4174-bc96-c301692ebced/chapter-plans`
- **原型地址**：`https://github.com/wangding75/ai-content-factory2/tree/feature/second-user-loop/docs/development-inputs/p1/iterations/iteration-15-real-chapter-planning/ui/frames/P15_C1_RUNNING_PARTIAL_RANGE`
- **状态准备方式**：通过 `POST /api/v1/projects/13a13e7e-656e-4174-bc96-c301692ebced/chapter-plan-runs/preflight`（generationMode: "range", target: { startChapterNo: 21, endChapterNo: 40 }）及 `POST /api/v1/projects/13a13e7e-656e-4174-bc96-c301692ebced/chapter-plan-runs` 发起真实局部范围 Run，并在页面处于真实 running/queued 运行状态时通过 Chromium 截图。
- **修改前问题**：
  - 运行横幅未动态识别局部范围运行模式，未展示“局部章节规划生成中”；
  - 横幅内缺少对应起止章节 (`Range: 第21—40章`) 和预计生成候选数量。
- **修改样式**：
  - 完善 `SummaryRunBanner` 对 `generationMode === "range"` 模式的解析与文案映射；
  - 动态格式化起止章节范围与预计生成候选数量；
  - 保持整体视觉风格与卡片、表格、筛选区等结构与冻结原型一致。
- **修改文件**：
  - `apps/web/src/features/chapter-plans/chapter-plans-workspace.tsx`
  - `apps/web/src/app/globals.css`
- **Network 修改前后结果**：
  - **修改前**：请求 5 次，状态码 200。
  - **修改后**：请求 5 次，请求 URL / Method / Body 完全一致，状态码 200，API 修改数 0，DTO 修改数 0，业务逻辑修改数 0。
- **原型图**：`docs/development-inputs/p1/iterations/iteration-15-real-chapter-planning/evidence/02-running-partial-range-prototype.png`
- **实机图**：`docs/development-inputs/p1/iterations/iteration-15-real-chapter-planning/evidence/02-running-partial-range-actual.png`
- **对比图**：`docs/development-inputs/p1/iterations/iteration-15-real-chapter-planning/evidence/02-running-partial-range-comparison.png`
- **最终结论**：**PASS**

---

## 三、03｜运行中－完整规划

- **原型名称**：`P15_C1_RUNNING_FULL_PLAN`
- **实机地址**：`http://127.0.0.1:13001/projects/13a13e7e-656e-4174-bc96-c301692ebced/chapter-plans`
- **原型地址**：`https://github.com/wangding75/ai-content-factory2/tree/feature/second-user-loop/docs/development-inputs/p1/iterations/iteration-15-real-chapter-planning/ui/frames/P15_C1_RUNNING_FULL_PLAN`
- **状态准备方式**：通过 `POST /api/v1/projects/13a13e7e-656e-4174-bc96-c301692ebced/chapter-plan-runs/preflight`（generationMode: "full", target: { targetTotalChapters: 100 }）及 `POST /api/v1/projects/13a13e7e-656e-4174-bc96-c301692ebced/chapter-plan-runs` 发起真实完整规划 Run，并在页面处于真实 running/queued 运行状态时通过 Chromium 截图。
- **修改前问题**：
  - 运行横幅未识别全局规划运行模式，未展示“全局章节规划生成中”；
  - 横幅内缺少完整规划范围 (`Range: 第1—100章`) 及 100 个预计候选数量。
- **修改样式**：
  - 完善 `SummaryRunBanner` 对 `generationMode === "full"` 模式的解析与文案映射；
  - 正确展示全局规划目标范围与统计数据；
  - 确保卡片、视图切换、表格与整体层次感完全对齐原型。
- **修改文件**：
  - `apps/web/src/features/chapter-plans/chapter-plans-workspace.tsx`
  - `apps/web/src/app/globals.css`
- **Network 修改前后结果**：
  - **修改前**：请求 5 次，状态码 200。
  - **修改后**：请求 5 次，请求 URL / Method / Body 完全一致，状态码 200，API 修改数 0，DTO 修改数 0，业务逻辑修改数 0。
- **原型图**：`docs/development-inputs/p1/iterations/iteration-15-real-chapter-planning/evidence/03-running-full-plan-prototype.png`
- **实机图**：`docs/development-inputs/p1/iterations/iteration-15-real-chapter-planning/evidence/03-running-full-plan-actual.png`
- **对比图**：`docs/development-inputs/p1/iterations/iteration-15-real-chapter-planning/evidence/03-running-full-plan-comparison.png`
- **最终结论**：**PASS**

---

## 四、全局与门禁汇总

- **定向测试**：`pnpm.cmd --filter web test` (155 个测试全部 PASS)
- **TypeScript 校验**：`pnpm.cmd typecheck` (通过)
- **代码 Lint**：`pnpm.cmd lint` (通过)
- **Production Build**：`pnpm.cmd build` (通过)
- **Git 格式检查**：`git diff --check` (无异常)
- **证据图数量**：原型图 3 张，实机图 3 张，对比图 3 张，共 9 张，互不重复，与实际运行状态完全对应。
