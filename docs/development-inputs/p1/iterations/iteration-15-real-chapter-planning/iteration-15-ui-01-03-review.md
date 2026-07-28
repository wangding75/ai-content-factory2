# Iteration 15 UI-01～UI-03 运行中页面重新修复验收报告

| 编号 | 小页面 | 源码路由 | 路由模板 | 实际完整 URL | 权威 screen.png 路径 | 辅助 code.html 路径 | 验收结论 |
|---|---|---|---|---|---|---|---|
| 01 | 运行中－主线扩写 | `apps/web/src/app/projects/[projectId]/chapter-plans/page.tsx` | `/projects/{projectId}/chapter-plans` | `http://127.0.0.1:13001/projects/13a13e7e-656e-4174-bc96-c301692ebced/chapter-plans` | `docs/development-inputs/p1/iterations/iteration-15-real-chapter-planning/ui/frames/P15_C1_RUNNING_MAINLINE_EXPANSION/screen.png` | `docs/development-inputs/p1/iterations/iteration-15-real-chapter-planning/ui/frames/P15_C1_RUNNING_MAINLINE_EXPANSION/code.html` | **PASS** |
| 02 | 运行中－局部范围 | `apps/web/src/app/projects/[projectId]/chapter-plans/page.tsx` | `/projects/{projectId}/chapter-plans` | `http://127.0.0.1:13001/projects/13a13e7e-656e-4174-bc96-c301692ebced/chapter-plans` | `docs/development-inputs/p1/iterations/iteration-15-real-chapter-planning/ui/frames/P15_C1_RUNNING_PARTIAL_RANGE/screen.png` | `docs/development-inputs/p1/iterations/iteration-15-real-chapter-planning/ui/frames/P15_C1_RUNNING_PARTIAL_RANGE/code.html` | **PASS** |
| 03 | 运行中－完整规划 | `apps/web/src/app/projects/[projectId]/chapter-plans/page.tsx` | `/projects/{projectId}/chapter-plans` | `http://127.0.0.1:13001/projects/13a13e7e-656e-4174-bc96-c301692ebced/chapter-plans` | `docs/development-inputs/p1/iterations/iteration-15-real-chapter-planning/ui/frames/P15_C1_RUNNING_FULL_PLAN/screen.png` | `docs/development-inputs/p1/iterations/iteration-15-real-chapter-planning/ui/frames/P15_C1_RUNNING_FULL_PLAN/code.html` | **PASS** |

---

## 一、T15-UI-01：运行中－主线扩写

- **状态名称**：运行中－主线扩写 (`running-mainline-expansion`)
- **源码路由**：`apps/web/src/app/projects/[projectId]/chapter-plans/page.tsx`
- **路由模板**：`/projects/{projectId}/chapter-plans`
- **实际完整 URL**：`http://127.0.0.1:13001/projects/13a13e7e-656e-4174-bc96-c301692ebced/chapter-plans`
- **项目 ID**：`13a13e7e-656e-4174-bc96-c301692ebced`
- **状态构造请求体**：
  ```json
  {
    "generationMode": "append",
    "target": {
      "chapterCount": 20
    },
    "storylineSelection": {
      "mode": "auto_balanced",
      "storylineIds": []
    },
    "contextOptions": {
      "includeProjectMaterials": true,
      "includeUnpaidForeshadowings": true,
      "includePriorChapterSummaries": true,
      "coreSettingsOnly": false
    },
    "additionalInstructions": null
  }
  ```
- **唯一权威 screen.png 路径**：`docs/development-inputs/p1/iterations/iteration-15-real-chapter-planning/ui/frames/P15_C1_RUNNING_MAINLINE_EXPANSION/screen.png`
- **辅助 code.html 路径**：`docs/development-inputs/p1/iterations/iteration-15-real-chapter-planning/ui/frames/P15_C1_RUNNING_MAINLINE_EXPANSION/code.html`
- **actual 路径**：`docs/development-inputs/p1/iterations/iteration-15-real-chapter-planning/evidence/01-running-mainline-expansion-actual.png`
- **comparison 路径**：`docs/development-inputs/p1/iterations/iteration-15-real-chapter-planning/evidence/01-running-mainline-expansion-comparison.png`
- **overlay 路径**：`docs/development-inputs/p1/iterations/iteration-15-real-chapter-planning/evidence/01-running-mainline-expansion-overlay.png`
- **viewport**：`1600 × 1280`
- **device scale factor**：`1`
- **页面断言**：
  - 页面标题准确展示 H1 `章节规划`；
  - 页面顶部主按钮区域包含“新增章节”与带有 `auto_awesome` 图标的“生成章节规划”按钮；
  - 运行 Banner 顶部展示“主线剧情扩展生成中”，包含运行状态 Badge `QUEUED` / `RUNNING`，格式化起止范围 `Range: 第3—22章`（目标生成 20 章）及已运行时长与阶段；
  - 视图 Tab 正确切换，展示“当前章节”及 Badge Count，以及指向候选批次页面的“候选批次”Tab 及其动态 Count；
  - 章节统计区以网格形式精准排布 4 张卡片，且“待确认”与“已确认”卡片各附带有 `#F59E0B` 与 `#10B981` 的左侧视觉强调层级；
  - 筛选区与表格卡片结构完整，无溢出与错位。
- **原型与实机主要差异**：首屏卡片边框色与背景在 Light 模式下使用全局与 Tailwind 主题设计 Token 规范对齐；
- **已修复项**：
  - 修复 `SummaryRunBanner` 中 `run.inputPayload` 在字符串/对象传输时的容错解析；
  - 纠正 Playwright Viewport 像素尺寸为 `1600 × 1280`；
  - 修复 Banner 内工作流链接至 `/workflow-runs?projectId=...` 消除 404 网络警告；
  - 补全三层证据链（actual、comparison、overlay）。
- **尚存差异**：无；
- **console 结果**：0 console error (已处理 favicon / 字体等网络层警告)；
- **网络错误结果**：0 failed business API network requests (5/5 GET APIs HTTP 200)；
- **专项测试结果**：Playwright 自动化截图脚本断言 PASS，单元测试 `pnpm --filter web test` PASS；
- **最终结论**：**PASS**

---

## 二、T15-UI-02：运行中－局部范围

- **状态名称**：运行中－局部范围 (`running-partial-range`)
- **源码路由**：`apps/web/src/app/projects/[projectId]/chapter-plans/page.tsx`
- **路由模板**：`/projects/{projectId}/chapter-plans`
- **实际完整 URL**：`http://127.0.0.1:13001/projects/13a13e7e-656e-4174-bc96-c301692ebced/chapter-plans`
- **项目 ID**：`13a13e7e-656e-4174-bc96-c301692ebced`
- **状态构造请求体**：
  ```json
  {
    "generationMode": "range",
    "target": {
      "startChapterNo": 21,
      "endChapterNo": 40
    },
    "storylineSelection": {
      "mode": "auto_balanced",
      "storylineIds": []
    },
    "contextOptions": {
      "includeProjectMaterials": true,
      "includeUnpaidForeshadowings": true,
      "includePriorChapterSummaries": true,
      "coreSettingsOnly": false
    },
    "additionalInstructions": null
  }
  ```
- **唯一权威 screen.png 路径**：`docs/development-inputs/p1/iterations/iteration-15-real-chapter-planning/ui/frames/P15_C1_RUNNING_PARTIAL_RANGE/screen.png`
- **辅助 code.html 路径**：`docs/development-inputs/p1/iterations/iteration-15-real-chapter-planning/ui/frames/P15_C1_RUNNING_PARTIAL_RANGE/code.html`
- **actual 路径**：`docs/development-inputs/p1/iterations/iteration-15-real-chapter-planning/evidence/02-running-partial-range-actual.png`
- **comparison 路径**：`docs/development-inputs/p1/iterations/iteration-15-real-chapter-planning/evidence/02-running-partial-range-comparison.png`
- **overlay 路径**：`docs/development-inputs/p1/iterations/iteration-15-real-chapter-planning/evidence/02-running-partial-range-overlay.png`
- **viewport**：`1600 × 1280`
- **device scale factor**：`1`
- **页面断言**：
  - 运行 Banner 标题正确识别为“局部章节规划生成中”；
  - 动态格式化显示章节起止范围 `Range: 第21—40章` 及预计生成 20 个章节候选；
  - 其它页面架构、标题、动作按钮、统计网格与数据列表保持与冻结原型高精度一致。
- **原型与实机主要差异**：无；
- **已修复项**：
  - 针对局部范围 Run 输入 Snapshot 的 `generationMode === "range"` 映射完成精准渲染；
  - 证据图补齐 1600x1280 真实截图、并排对比及 Overlay 叠加透明度分析；
- **尚存差异**：无；
- **console 结果**：0 console error；
- **网络错误结果**：0 failed business API network requests (5/5 GET APIs HTTP 200)；
- **专项测试结果**：Playwright 自动化截图脚本断言 PASS，单元测试 `pnpm --filter web test` PASS；
- **最终结论**：**PASS**

---

## 三、T15-UI-03：运行中－完整规划

- **状态名称**：运行中－完整规划 (`running-full-plan`)
- **源码路由**：`apps/web/src/app/projects/[projectId]/chapter-plans/page.tsx`
- **路由模板**：`/projects/{projectId}/chapter-plans`
- **实际完整 URL**：`http://127.0.0.1:13001/projects/13a13e7e-656e-4174-bc96-c301692ebced/chapter-plans`
- **项目 ID**：`13a13e7e-656e-4174-bc96-c301692ebced`
- **状态构造请求体**：
  ```json
  {
    "generationMode": "full",
    "target": {
      "targetTotalChapters": 100
    },
    "storylineSelection": {
      "mode": "auto_balanced",
      "storylineIds": []
    },
    "contextOptions": {
      "includeProjectMaterials": true,
      "includeUnpaidForeshadowings": true,
      "includePriorChapterSummaries": true,
      "coreSettingsOnly": false
    },
    "additionalInstructions": null
  }
  ```
- **唯一权威 screen.png 路径**：`docs/development-inputs/p1/iterations/iteration-15-real-chapter-planning/ui/frames/P15_C1_RUNNING_FULL_PLAN/screen.png`
- **辅助 code.html 路径**：`docs/development-inputs/p1/iterations/iteration-15-real-chapter-planning/ui/frames/P15_C1_RUNNING_FULL_PLAN/code.html`
- **actual 路径**：`docs/development-inputs/p1/iterations/iteration-15-real-chapter-planning/evidence/03-running-full-plan-actual.png`
- **comparison 路径**：`docs/development-inputs/p1/iterations/iteration-15-real-chapter-planning/evidence/03-running-full-plan-comparison.png`
- **overlay 路径**：`docs/development-inputs/p1/iterations/iteration-15-real-chapter-planning/evidence/03-running-full-plan-overlay.png`
- **viewport**：`1600 × 1280`
- **device scale factor**：`1`
- **页面断言**：
  - 运行 Banner 标题正确识别为“全局章节规划生成中”；
  - 动态格式化显示目标章节范围 `Range: 第1—100章` 及预计生成 100 个章节候选；
  - 整体页面布局、层级与主线/局部运行状态保持连贯的卡片与表单排布。
- **原型与实机主要差异**：无；
- **已修复项**：
  - 针对完整规划 Run 输入 Snapshot 的 `generationMode === "full"` 映射完成精准渲染；
  - 证据图补齐 1600x1280 真实截图、并排对比及 Overlay 叠加透明度分析；
- **尚存差异**：无；
- **console 结果**：0 console error；
- **网络错误结果**：0 failed business API network requests (5/5 GET APIs HTTP 200)；
- **专项测试结果**：Playwright 自动化截图脚本断言 PASS，单元测试 `pnpm --filter web test` PASS；
- **最终结论**：**PASS**

---

## 四、全局门禁与回归结果

1. **定向测试与全量 Web 测试**：`pnpm.cmd run test:web` (155 passing, 0 failing)
2. **TypeScript 校验**：`pnpm.cmd run typecheck:web` (0 errors)
3. **代码 Lint**：`pnpm.cmd run lint:web` (0 errors)
4. **Production Build**：`pnpm.cmd run build:web` (Next.js Turbo 编译完成，0 build errors)
5. **Docker Container Build**：`docker compose build web; docker compose up -d web` 重新镜像构建成功并升级应用容器。
6. **Git 格式规范**：`git diff --check` (0 issues)
7. **冻结原型与契约校验**：
   - `git diff --name-only -- docs/development-inputs/p1/iterations/iteration-15-real-chapter-planning/ui/frames` -> 无任何修改
   - `git diff --name-only -- packages/contracts/openapi/openapi.yaml` -> 无任何修改
   - `git diff --name-only -- docs/development-inputs/p1/iterations/iteration-15-real-chapter-planning/api-scope.yaml` -> 无任何修改
   - `git diff --name-only -- docs/development-inputs/p1/iterations/iteration-15-real-chapter-planning/data-model.md` -> 无任何修改
