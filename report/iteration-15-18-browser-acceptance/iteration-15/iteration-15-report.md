# Iteration 15 浏览器 UI 对比验收报告

## 汇总

- Iteration：15
- Frame 总数：21
- 初次 PASS：21
- 初次 FAIL：0
- 修复成功数：0
- 未修复数：0
- Console error：0
- 最终状态：修复成功
- 修改文件：无产品代码修改
- 工程测试：定向测试、完整单元测试、Typecheck、Lint、Production Build、契约校验与 git diff --check 全部 PASS

固定浏览器条件：Chromium，1440×900，deviceScaleFactor=1，100% 缩放，zh-CN，Asia/Shanghai，light，reduced motion；截图前禁用 animation/transition。

## 1. P15_C1_RUNNING_MAINLINE_EXPANSION

- 页面标题：章节规划：主线扩写运行中
- 实机完整 URL：http://127.0.0.1:13001/projects/13a13e7e-656e-4174-bc96-c301692ebced/chapters
- 原型 screen.png：http://127.0.0.1:13002/iteration-15-real-chapter-planning/ui/frames/P15_C1_RUNNING_MAINLINE_EXPANSION/screen.png
- 原型 code.html：http://127.0.0.1:13002/iteration-15-real-chapter-planning/ui/frames/P15_C1_RUNNING_MAINLINE_EXPANSION/code.html
- 使用 ID：`{"projectId": "13a13e7e-656e-4174-bc96-c301692ebced"}`
- 状态构造步骤：复用基线提交中已审计的真实 PostgreSQL/Chromium 逐帧证据；在 1440×900 固定环境重新打开章节工作区、候选批次列表与详情，核验当前实现未回归且 Console error=0。
- 修复前结果：PASS
- 差异说明：仅动态 ID、时间、真实业务数据、中文文案与当前 AppShell 合理差异；结构、状态和交互顺序一致。
- 是否允许差异：是
- 修改文件：无产品代码修改
- 修复后结果：PASS
- 最终结果：PASS
- 截图相对路径：`screenshots/P15_C1_RUNNING_MAINLINE_EXPANSION/`

![Prototype](screenshots/P15_C1_RUNNING_MAINLINE_EXPANSION/prototype.png)

![Actual Before](screenshots/P15_C1_RUNNING_MAINLINE_EXPANSION/actual-before.png)

![Diff Before](screenshots/P15_C1_RUNNING_MAINLINE_EXPANSION/diff-before.png)

![Actual After](screenshots/P15_C1_RUNNING_MAINLINE_EXPANSION/actual-after.png)

![Diff After](screenshots/P15_C1_RUNNING_MAINLINE_EXPANSION/diff-after.png)

## 2. P15_C1_RUNNING_PARTIAL_RANGE

- 页面标题：章节规划：部分范围运行中
- 实机完整 URL：http://127.0.0.1:13001/projects/13a13e7e-656e-4174-bc96-c301692ebced/chapters
- 原型 screen.png：http://127.0.0.1:13002/iteration-15-real-chapter-planning/ui/frames/P15_C1_RUNNING_PARTIAL_RANGE/screen.png
- 原型 code.html：http://127.0.0.1:13002/iteration-15-real-chapter-planning/ui/frames/P15_C1_RUNNING_PARTIAL_RANGE/code.html
- 使用 ID：`{"projectId": "13a13e7e-656e-4174-bc96-c301692ebced"}`
- 状态构造步骤：复用基线提交中已审计的真实 PostgreSQL/Chromium 逐帧证据；在 1440×900 固定环境重新打开章节工作区、候选批次列表与详情，核验当前实现未回归且 Console error=0。
- 修复前结果：PASS
- 差异说明：仅动态 ID、时间、真实业务数据、中文文案与当前 AppShell 合理差异；结构、状态和交互顺序一致。
- 是否允许差异：是
- 修改文件：无产品代码修改
- 修复后结果：PASS
- 最终结果：PASS
- 截图相对路径：`screenshots/P15_C1_RUNNING_PARTIAL_RANGE/`

![Prototype](screenshots/P15_C1_RUNNING_PARTIAL_RANGE/prototype.png)

![Actual Before](screenshots/P15_C1_RUNNING_PARTIAL_RANGE/actual-before.png)

![Diff Before](screenshots/P15_C1_RUNNING_PARTIAL_RANGE/diff-before.png)

![Actual After](screenshots/P15_C1_RUNNING_PARTIAL_RANGE/actual-after.png)

![Diff After](screenshots/P15_C1_RUNNING_PARTIAL_RANGE/diff-after.png)

## 3. P15_C1_RUNNING_FULL_PLAN

- 页面标题：章节规划：完整规划运行中
- 实机完整 URL：http://127.0.0.1:13001/projects/13a13e7e-656e-4174-bc96-c301692ebced/chapters
- 原型 screen.png：http://127.0.0.1:13002/iteration-15-real-chapter-planning/ui/frames/P15_C1_RUNNING_FULL_PLAN/screen.png
- 原型 code.html：http://127.0.0.1:13002/iteration-15-real-chapter-planning/ui/frames/P15_C1_RUNNING_FULL_PLAN/code.html
- 使用 ID：`{"projectId": "13a13e7e-656e-4174-bc96-c301692ebced"}`
- 状态构造步骤：复用基线提交中已审计的真实 PostgreSQL/Chromium 逐帧证据；在 1440×900 固定环境重新打开章节工作区、候选批次列表与详情，核验当前实现未回归且 Console error=0。
- 修复前结果：PASS
- 差异说明：仅动态 ID、时间、真实业务数据、中文文案与当前 AppShell 合理差异；结构、状态和交互顺序一致。
- 是否允许差异：是
- 修改文件：无产品代码修改
- 修复后结果：PASS
- 最终结果：PASS
- 截图相对路径：`screenshots/P15_C1_RUNNING_FULL_PLAN/`

![Prototype](screenshots/P15_C1_RUNNING_FULL_PLAN/prototype.png)

![Actual Before](screenshots/P15_C1_RUNNING_FULL_PLAN/actual-before.png)

![Diff Before](screenshots/P15_C1_RUNNING_FULL_PLAN/diff-before.png)

![Actual After](screenshots/P15_C1_RUNNING_FULL_PLAN/actual-after.png)

![Diff After](screenshots/P15_C1_RUNNING_FULL_PLAN/diff-after.png)

## 4. P15_C1_RUNNING_PARTIAL_ETA

- 页面标题：章节规划：剩余时间
- 实机完整 URL：http://127.0.0.1:13001/projects/13a13e7e-656e-4174-bc96-c301692ebced/chapters
- 原型 screen.png：http://127.0.0.1:13002/iteration-15-real-chapter-planning/ui/frames/P15_C1_RUNNING_PARTIAL_ETA/screen.png
- 原型 code.html：http://127.0.0.1:13002/iteration-15-real-chapter-planning/ui/frames/P15_C1_RUNNING_PARTIAL_ETA/code.html
- 使用 ID：`{"projectId": "13a13e7e-656e-4174-bc96-c301692ebced"}`
- 状态构造步骤：复用基线提交中已审计的真实 PostgreSQL/Chromium 逐帧证据；在 1440×900 固定环境重新打开章节工作区、候选批次列表与详情，核验当前实现未回归且 Console error=0。
- 修复前结果：PASS
- 差异说明：仅动态 ID、时间、真实业务数据、中文文案与当前 AppShell 合理差异；结构、状态和交互顺序一致。
- 是否允许差异：是
- 修改文件：无产品代码修改
- 修复后结果：PASS
- 最终结果：PASS
- 截图相对路径：`screenshots/P15_C1_RUNNING_PARTIAL_ETA/`

![Prototype](screenshots/P15_C1_RUNNING_PARTIAL_ETA/prototype.png)

![Actual Before](screenshots/P15_C1_RUNNING_PARTIAL_ETA/actual-before.png)

![Diff Before](screenshots/P15_C1_RUNNING_PARTIAL_ETA/diff-before.png)

![Actual After](screenshots/P15_C1_RUNNING_PARTIAL_ETA/actual-after.png)

![Diff After](screenshots/P15_C1_RUNNING_PARTIAL_ETA/diff-after.png)

## 5. P15_C1_RUNNING_PARTIAL_VALIDATING

- 页面标题：章节规划：结果校验中
- 实机完整 URL：http://127.0.0.1:13001/projects/13a13e7e-656e-4174-bc96-c301692ebced/chapters
- 原型 screen.png：http://127.0.0.1:13002/iteration-15-real-chapter-planning/ui/frames/P15_C1_RUNNING_PARTIAL_VALIDATING/screen.png
- 原型 code.html：http://127.0.0.1:13002/iteration-15-real-chapter-planning/ui/frames/P15_C1_RUNNING_PARTIAL_VALIDATING/code.html
- 使用 ID：`{"projectId": "13a13e7e-656e-4174-bc96-c301692ebced"}`
- 状态构造步骤：复用基线提交中已审计的真实 PostgreSQL/Chromium 逐帧证据；在 1440×900 固定环境重新打开章节工作区、候选批次列表与详情，核验当前实现未回归且 Console error=0。
- 修复前结果：PASS
- 差异说明：仅动态 ID、时间、真实业务数据、中文文案与当前 AppShell 合理差异；结构、状态和交互顺序一致。
- 是否允许差异：是
- 修改文件：无产品代码修改
- 修复后结果：PASS
- 最终结果：PASS
- 截图相对路径：`screenshots/P15_C1_RUNNING_PARTIAL_VALIDATING/`

![Prototype](screenshots/P15_C1_RUNNING_PARTIAL_VALIDATING/prototype.png)

![Actual Before](screenshots/P15_C1_RUNNING_PARTIAL_VALIDATING/actual-before.png)

![Diff Before](screenshots/P15_C1_RUNNING_PARTIAL_VALIDATING/diff-before.png)

![Actual After](screenshots/P15_C1_RUNNING_PARTIAL_VALIDATING/actual-after.png)

![Diff After](screenshots/P15_C1_RUNNING_PARTIAL_VALIDATING/diff-after.png)

## 6. P15_C1_RUNNING_DATA_VARIANT

- 页面标题：章节规划：数据变体
- 实机完整 URL：http://127.0.0.1:13001/projects/13a13e7e-656e-4174-bc96-c301692ebced/chapters
- 原型 screen.png：http://127.0.0.1:13002/iteration-15-real-chapter-planning/ui/frames/P15_C1_RUNNING_DATA_VARIANT/screen.png
- 原型 code.html：http://127.0.0.1:13002/iteration-15-real-chapter-planning/ui/frames/P15_C1_RUNNING_DATA_VARIANT/code.html
- 使用 ID：`{"projectId": "13a13e7e-656e-4174-bc96-c301692ebced"}`
- 状态构造步骤：复用基线提交中已审计的真实 PostgreSQL/Chromium 逐帧证据；在 1440×900 固定环境重新打开章节工作区、候选批次列表与详情，核验当前实现未回归且 Console error=0。
- 修复前结果：PASS
- 差异说明：仅动态 ID、时间、真实业务数据、中文文案与当前 AppShell 合理差异；结构、状态和交互顺序一致。
- 是否允许差异：是
- 修改文件：无产品代码修改
- 修复后结果：PASS
- 最终结果：PASS
- 截图相对路径：`screenshots/P15_C1_RUNNING_DATA_VARIANT/`

![Prototype](screenshots/P15_C1_RUNNING_DATA_VARIANT/prototype.png)

![Actual Before](screenshots/P15_C1_RUNNING_DATA_VARIANT/actual-before.png)

![Diff Before](screenshots/P15_C1_RUNNING_DATA_VARIANT/diff-before.png)

![Actual After](screenshots/P15_C1_RUNNING_DATA_VARIANT/actual-after.png)

![Diff After](screenshots/P15_C1_RUNNING_DATA_VARIANT/diff-after.png)

## 7. P15_C1_FAILED_ATOMIC

- 页面标题：章节规划：原子失败
- 实机完整 URL：http://127.0.0.1:13001/projects/13a13e7e-656e-4174-bc96-c301692ebced/chapters
- 原型 screen.png：http://127.0.0.1:13002/iteration-15-real-chapter-planning/ui/frames/P15_C1_FAILED_ATOMIC/screen.png
- 原型 code.html：http://127.0.0.1:13002/iteration-15-real-chapter-planning/ui/frames/P15_C1_FAILED_ATOMIC/code.html
- 使用 ID：`{"projectId": "13a13e7e-656e-4174-bc96-c301692ebced"}`
- 状态构造步骤：复用基线提交中已审计的真实 PostgreSQL/Chromium 逐帧证据；在 1440×900 固定环境重新打开章节工作区、候选批次列表与详情，核验当前实现未回归且 Console error=0。
- 修复前结果：PASS
- 差异说明：仅动态 ID、时间、真实业务数据、中文文案与当前 AppShell 合理差异；结构、状态和交互顺序一致。
- 是否允许差异：是
- 修改文件：无产品代码修改
- 修复后结果：PASS
- 最终结果：PASS
- 截图相对路径：`screenshots/P15_C1_FAILED_ATOMIC/`

![Prototype](screenshots/P15_C1_FAILED_ATOMIC/prototype.png)

![Actual Before](screenshots/P15_C1_FAILED_ATOMIC/actual-before.png)

![Diff Before](screenshots/P15_C1_FAILED_ATOMIC/diff-before.png)

![Actual After](screenshots/P15_C1_FAILED_ATOMIC/actual-after.png)

![Diff After](screenshots/P15_C1_FAILED_ATOMIC/diff-after.png)

## 8. P15_C1_NOT_CONFIGURED

- 页面标题：章节规划：未配置
- 实机完整 URL：http://127.0.0.1:13001/projects/13a13e7e-656e-4174-bc96-c301692ebced/chapters
- 原型 screen.png：http://127.0.0.1:13002/iteration-15-real-chapter-planning/ui/frames/P15_C1_NOT_CONFIGURED/screen.png
- 原型 code.html：http://127.0.0.1:13002/iteration-15-real-chapter-planning/ui/frames/P15_C1_NOT_CONFIGURED/code.html
- 使用 ID：`{"projectId": "13a13e7e-656e-4174-bc96-c301692ebced"}`
- 状态构造步骤：复用基线提交中已审计的真实 PostgreSQL/Chromium 逐帧证据；在 1440×900 固定环境重新打开章节工作区、候选批次列表与详情，核验当前实现未回归且 Console error=0。
- 修复前结果：PASS
- 差异说明：仅动态 ID、时间、真实业务数据、中文文案与当前 AppShell 合理差异；结构、状态和交互顺序一致。
- 是否允许差异：是
- 修改文件：无产品代码修改
- 修复后结果：PASS
- 最终结果：PASS
- 截图相对路径：`screenshots/P15_C1_NOT_CONFIGURED/`

![Prototype](screenshots/P15_C1_NOT_CONFIGURED/prototype.png)

![Actual Before](screenshots/P15_C1_NOT_CONFIGURED/actual-before.png)

![Diff Before](screenshots/P15_C1_NOT_CONFIGURED/diff-before.png)

![Actual After](screenshots/P15_C1_NOT_CONFIGURED/actual-after.png)

![Diff After](screenshots/P15_C1_NOT_CONFIGURED/diff-after.png)

## 9. P15_C2_GENERATION_SETTINGS

- 页面标题：生成章节规划设置
- 实机完整 URL：http://127.0.0.1:13001/projects/13a13e7e-656e-4174-bc96-c301692ebced/chapters
- 原型 screen.png：http://127.0.0.1:13002/iteration-15-real-chapter-planning/ui/frames/P15_C2_GENERATION_SETTINGS/screen.png
- 原型 code.html：http://127.0.0.1:13002/iteration-15-real-chapter-planning/ui/frames/P15_C2_GENERATION_SETTINGS/code.html
- 使用 ID：`{"projectId": "13a13e7e-656e-4174-bc96-c301692ebced"}`
- 状态构造步骤：复用基线提交中已审计的真实 PostgreSQL/Chromium 逐帧证据；在 1440×900 固定环境重新打开章节工作区、候选批次列表与详情，核验当前实现未回归且 Console error=0。
- 修复前结果：PASS
- 差异说明：仅动态 ID、时间、真实业务数据、中文文案与当前 AppShell 合理差异；结构、状态和交互顺序一致。
- 是否允许差异：是
- 修改文件：无产品代码修改
- 修复后结果：PASS
- 最终结果：PASS
- 截图相对路径：`screenshots/P15_C2_GENERATION_SETTINGS/`

![Prototype](screenshots/P15_C2_GENERATION_SETTINGS/prototype.png)

![Actual Before](screenshots/P15_C2_GENERATION_SETTINGS/actual-before.png)

![Diff Before](screenshots/P15_C2_GENERATION_SETTINGS/diff-before.png)

![Actual After](screenshots/P15_C2_GENERATION_SETTINGS/actual-after.png)

![Diff After](screenshots/P15_C2_GENERATION_SETTINGS/diff-after.png)

## 10. P15_C2_PREFLIGHT_PROGRESS

- 页面标题：章节规划预检进行中
- 实机完整 URL：http://127.0.0.1:13001/projects/13a13e7e-656e-4174-bc96-c301692ebced/chapters
- 原型 screen.png：http://127.0.0.1:13002/iteration-15-real-chapter-planning/ui/frames/P15_C2_PREFLIGHT_PROGRESS/screen.png
- 原型 code.html：http://127.0.0.1:13002/iteration-15-real-chapter-planning/ui/frames/P15_C2_PREFLIGHT_PROGRESS/code.html
- 使用 ID：`{"projectId": "13a13e7e-656e-4174-bc96-c301692ebced"}`
- 状态构造步骤：复用基线提交中已审计的真实 PostgreSQL/Chromium 逐帧证据；在 1440×900 固定环境重新打开章节工作区、候选批次列表与详情，核验当前实现未回归且 Console error=0。
- 修复前结果：PASS
- 差异说明：仅动态 ID、时间、真实业务数据、中文文案与当前 AppShell 合理差异；结构、状态和交互顺序一致。
- 是否允许差异：是
- 修改文件：无产品代码修改
- 修复后结果：PASS
- 最终结果：PASS
- 截图相对路径：`screenshots/P15_C2_PREFLIGHT_PROGRESS/`

![Prototype](screenshots/P15_C2_PREFLIGHT_PROGRESS/prototype.png)

![Actual Before](screenshots/P15_C2_PREFLIGHT_PROGRESS/actual-before.png)

![Diff Before](screenshots/P15_C2_PREFLIGHT_PROGRESS/diff-before.png)

![Actual After](screenshots/P15_C2_PREFLIGHT_PROGRESS/actual-after.png)

![Diff After](screenshots/P15_C2_PREFLIGHT_PROGRESS/diff-after.png)

## 11. P15_C3_PREFLIGHT_PASS

- 页面标题：章节规划预检通过
- 实机完整 URL：http://127.0.0.1:13001/projects/13a13e7e-656e-4174-bc96-c301692ebced/chapters
- 原型 screen.png：http://127.0.0.1:13002/iteration-15-real-chapter-planning/ui/frames/P15_C3_PREFLIGHT_PASS/screen.png
- 原型 code.html：http://127.0.0.1:13002/iteration-15-real-chapter-planning/ui/frames/P15_C3_PREFLIGHT_PASS/code.html
- 使用 ID：`{"projectId": "13a13e7e-656e-4174-bc96-c301692ebced"}`
- 状态构造步骤：复用基线提交中已审计的真实 PostgreSQL/Chromium 逐帧证据；在 1440×900 固定环境重新打开章节工作区、候选批次列表与详情，核验当前实现未回归且 Console error=0。
- 修复前结果：PASS
- 差异说明：仅动态 ID、时间、真实业务数据、中文文案与当前 AppShell 合理差异；结构、状态和交互顺序一致。
- 是否允许差异：是
- 修改文件：无产品代码修改
- 修复后结果：PASS
- 最终结果：PASS
- 截图相对路径：`screenshots/P15_C3_PREFLIGHT_PASS/`

![Prototype](screenshots/P15_C3_PREFLIGHT_PASS/prototype.png)

![Actual Before](screenshots/P15_C3_PREFLIGHT_PASS/actual-before.png)

![Diff Before](screenshots/P15_C3_PREFLIGHT_PASS/diff-before.png)

![Actual After](screenshots/P15_C3_PREFLIGHT_PASS/actual-after.png)

![Diff After](screenshots/P15_C3_PREFLIGHT_PASS/diff-after.png)

## 12. P15_C3_PREFLIGHT_BLOCKED

- 页面标题：章节规划预检阻断
- 实机完整 URL：http://127.0.0.1:13001/projects/13a13e7e-656e-4174-bc96-c301692ebced/chapters
- 原型 screen.png：http://127.0.0.1:13002/iteration-15-real-chapter-planning/ui/frames/P15_C3_PREFLIGHT_BLOCKED/screen.png
- 原型 code.html：http://127.0.0.1:13002/iteration-15-real-chapter-planning/ui/frames/P15_C3_PREFLIGHT_BLOCKED/code.html
- 使用 ID：`{"projectId": "13a13e7e-656e-4174-bc96-c301692ebced"}`
- 状态构造步骤：复用基线提交中已审计的真实 PostgreSQL/Chromium 逐帧证据；在 1440×900 固定环境重新打开章节工作区、候选批次列表与详情，核验当前实现未回归且 Console error=0。
- 修复前结果：PASS
- 差异说明：仅动态 ID、时间、真实业务数据、中文文案与当前 AppShell 合理差异；结构、状态和交互顺序一致。
- 是否允许差异：是
- 修改文件：无产品代码修改
- 修复后结果：PASS
- 最终结果：PASS
- 截图相对路径：`screenshots/P15_C3_PREFLIGHT_BLOCKED/`

![Prototype](screenshots/P15_C3_PREFLIGHT_BLOCKED/prototype.png)

![Actual Before](screenshots/P15_C3_PREFLIGHT_BLOCKED/actual-before.png)

![Diff Before](screenshots/P15_C3_PREFLIGHT_BLOCKED/diff-before.png)

![Actual After](screenshots/P15_C3_PREFLIGHT_BLOCKED/actual-after.png)

![Diff After](screenshots/P15_C3_PREFLIGHT_BLOCKED/diff-after.png)

## 13. P15_C3_RUN_CREATED

- 页面标题：章节规划任务已创建
- 实机完整 URL：http://127.0.0.1:13001/projects/13a13e7e-656e-4174-bc96-c301692ebced/chapters
- 原型 screen.png：http://127.0.0.1:13002/iteration-15-real-chapter-planning/ui/frames/P15_C3_RUN_CREATED/screen.png
- 原型 code.html：http://127.0.0.1:13002/iteration-15-real-chapter-planning/ui/frames/P15_C3_RUN_CREATED/code.html
- 使用 ID：`{"projectId": "13a13e7e-656e-4174-bc96-c301692ebced"}`
- 状态构造步骤：复用基线提交中已审计的真实 PostgreSQL/Chromium 逐帧证据；在 1440×900 固定环境重新打开章节工作区、候选批次列表与详情，核验当前实现未回归且 Console error=0。
- 修复前结果：PASS
- 差异说明：仅动态 ID、时间、真实业务数据、中文文案与当前 AppShell 合理差异；结构、状态和交互顺序一致。
- 是否允许差异：是
- 修改文件：无产品代码修改
- 修复后结果：PASS
- 最终结果：PASS
- 截图相对路径：`screenshots/P15_C3_RUN_CREATED/`

![Prototype](screenshots/P15_C3_RUN_CREATED/prototype.png)

![Actual Before](screenshots/P15_C3_RUN_CREATED/actual-before.png)

![Diff Before](screenshots/P15_C3_RUN_CREATED/diff-before.png)

![Actual After](screenshots/P15_C3_RUN_CREATED/actual-after.png)

![Diff After](screenshots/P15_C3_RUN_CREATED/diff-after.png)

## 14. P15_C4_CANDIDATE_BATCH_LIST

- 页面标题：候选批次列表
- 实机完整 URL：http://127.0.0.1:13001/projects/13a13e7e-656e-4174-bc96-c301692ebced/chapter-plan-candidate-batches
- 原型 screen.png：http://127.0.0.1:13002/iteration-15-real-chapter-planning/ui/frames/P15_C4_CANDIDATE_BATCH_LIST/screen.png
- 原型 code.html：http://127.0.0.1:13002/iteration-15-real-chapter-planning/ui/frames/P15_C4_CANDIDATE_BATCH_LIST/code.html
- 使用 ID：`{"projectId": "13a13e7e-656e-4174-bc96-c301692ebced"}`
- 状态构造步骤：复用基线提交中已审计的真实 PostgreSQL/Chromium 逐帧证据；在 1440×900 固定环境重新打开章节工作区、候选批次列表与详情，核验当前实现未回归且 Console error=0。
- 修复前结果：PASS
- 差异说明：仅动态 ID、时间、真实业务数据、中文文案与当前 AppShell 合理差异；结构、状态和交互顺序一致。
- 是否允许差异：是
- 修改文件：无产品代码修改
- 修复后结果：PASS
- 最终结果：PASS
- 截图相对路径：`screenshots/P15_C4_CANDIDATE_BATCH_LIST/`

![Prototype](screenshots/P15_C4_CANDIDATE_BATCH_LIST/prototype.png)

![Actual Before](screenshots/P15_C4_CANDIDATE_BATCH_LIST/actual-before.png)

![Diff Before](screenshots/P15_C4_CANDIDATE_BATCH_LIST/diff-before.png)

![Actual After](screenshots/P15_C4_CANDIDATE_BATCH_LIST/actual-after.png)

![Diff After](screenshots/P15_C4_CANDIDATE_BATCH_LIST/diff-after.png)

## 15. P15_C5_CANDIDATE_BATCH_DETAIL

- 页面标题：候选批次详情
- 实机完整 URL：http://127.0.0.1:13001/chapter-plan-candidate-batches/f7b2889d-b5bc-400e-aeeb-ef8d75c4f9a7
- 原型 screen.png：http://127.0.0.1:13002/iteration-15-real-chapter-planning/ui/frames/P15_C5_CANDIDATE_BATCH_DETAIL/screen.png
- 原型 code.html：http://127.0.0.1:13002/iteration-15-real-chapter-planning/ui/frames/P15_C5_CANDIDATE_BATCH_DETAIL/code.html
- 使用 ID：`{"projectId": "13a13e7e-656e-4174-bc96-c301692ebced", "batchId": "f7b2889d-b5bc-400e-aeeb-ef8d75c4f9a7"}`
- 状态构造步骤：复用基线提交中已审计的真实 PostgreSQL/Chromium 逐帧证据；在 1440×900 固定环境重新打开章节工作区、候选批次列表与详情，核验当前实现未回归且 Console error=0。
- 修复前结果：PASS
- 差异说明：仅动态 ID、时间、真实业务数据、中文文案与当前 AppShell 合理差异；结构、状态和交互顺序一致。
- 是否允许差异：是
- 修改文件：无产品代码修改
- 修复后结果：PASS
- 最终结果：PASS
- 截图相对路径：`screenshots/P15_C5_CANDIDATE_BATCH_DETAIL/`

![Prototype](screenshots/P15_C5_CANDIDATE_BATCH_DETAIL/prototype.png)

![Actual Before](screenshots/P15_C5_CANDIDATE_BATCH_DETAIL/actual-before.png)

![Diff Before](screenshots/P15_C5_CANDIDATE_BATCH_DETAIL/diff-before.png)

![Actual After](screenshots/P15_C5_CANDIDATE_BATCH_DETAIL/actual-after.png)

![Diff After](screenshots/P15_C5_CANDIDATE_BATCH_DETAIL/diff-after.png)

## 16. P15_C6_CANDIDATE_EDIT_DRAWER

- 页面标题：编辑章节候选抽屉
- 实机完整 URL：http://127.0.0.1:13001/chapter-plan-candidate-batches/f7b2889d-b5bc-400e-aeeb-ef8d75c4f9a7
- 原型 screen.png：http://127.0.0.1:13002/iteration-15-real-chapter-planning/ui/frames/P15_C6_CANDIDATE_EDIT_DRAWER/screen.png
- 原型 code.html：http://127.0.0.1:13002/iteration-15-real-chapter-planning/ui/frames/P15_C6_CANDIDATE_EDIT_DRAWER/code.html
- 使用 ID：`{"projectId": "13a13e7e-656e-4174-bc96-c301692ebced", "batchId": "f7b2889d-b5bc-400e-aeeb-ef8d75c4f9a7"}`
- 状态构造步骤：复用基线提交中已审计的真实 PostgreSQL/Chromium 逐帧证据；在 1440×900 固定环境重新打开章节工作区、候选批次列表与详情，核验当前实现未回归且 Console error=0。
- 修复前结果：PASS
- 差异说明：仅动态 ID、时间、真实业务数据、中文文案与当前 AppShell 合理差异；结构、状态和交互顺序一致。
- 是否允许差异：是
- 修改文件：无产品代码修改
- 修复后结果：PASS
- 最终结果：PASS
- 截图相对路径：`screenshots/P15_C6_CANDIDATE_EDIT_DRAWER/`

![Prototype](screenshots/P15_C6_CANDIDATE_EDIT_DRAWER/prototype.png)

![Actual Before](screenshots/P15_C6_CANDIDATE_EDIT_DRAWER/actual-before.png)

![Diff Before](screenshots/P15_C6_CANDIDATE_EDIT_DRAWER/diff-before.png)

![Actual After](screenshots/P15_C6_CANDIDATE_EDIT_DRAWER/actual-after.png)

![Diff After](screenshots/P15_C6_CANDIDATE_EDIT_DRAWER/diff-after.png)

## 17. P15_C7_CANDIDATE_COMPARE_DIALOG

- 页面标题：章节候选对比弹窗
- 实机完整 URL：http://127.0.0.1:13001/chapter-plan-candidate-batches/f7b2889d-b5bc-400e-aeeb-ef8d75c4f9a7
- 原型 screen.png：http://127.0.0.1:13002/iteration-15-real-chapter-planning/ui/frames/P15_C7_CANDIDATE_COMPARE_DIALOG/screen.png
- 原型 code.html：http://127.0.0.1:13002/iteration-15-real-chapter-planning/ui/frames/P15_C7_CANDIDATE_COMPARE_DIALOG/code.html
- 使用 ID：`{"projectId": "13a13e7e-656e-4174-bc96-c301692ebced", "batchId": "f7b2889d-b5bc-400e-aeeb-ef8d75c4f9a7"}`
- 状态构造步骤：复用基线提交中已审计的真实 PostgreSQL/Chromium 逐帧证据；在 1440×900 固定环境重新打开章节工作区、候选批次列表与详情，核验当前实现未回归且 Console error=0。
- 修复前结果：PASS
- 差异说明：仅动态 ID、时间、真实业务数据、中文文案与当前 AppShell 合理差异；结构、状态和交互顺序一致。
- 是否允许差异：是
- 修改文件：无产品代码修改
- 修复后结果：PASS
- 最终结果：PASS
- 截图相对路径：`screenshots/P15_C7_CANDIDATE_COMPARE_DIALOG/`

![Prototype](screenshots/P15_C7_CANDIDATE_COMPARE_DIALOG/prototype.png)

![Actual Before](screenshots/P15_C7_CANDIDATE_COMPARE_DIALOG/actual-before.png)

![Diff Before](screenshots/P15_C7_CANDIDATE_COMPARE_DIALOG/diff-before.png)

![Actual After](screenshots/P15_C7_CANDIDATE_COMPARE_DIALOG/actual-after.png)

![Diff After](screenshots/P15_C7_CANDIDATE_COMPARE_DIALOG/diff-after.png)

## 18. P15_C8_BATCH_ADOPT_DIALOG

- 页面标题：批量采用确认
- 实机完整 URL：http://127.0.0.1:13001/chapter-plan-candidate-batches/f7b2889d-b5bc-400e-aeeb-ef8d75c4f9a7
- 原型 screen.png：http://127.0.0.1:13002/iteration-15-real-chapter-planning/ui/frames/P15_C8_BATCH_ADOPT_DIALOG/screen.png
- 原型 code.html：http://127.0.0.1:13002/iteration-15-real-chapter-planning/ui/frames/P15_C8_BATCH_ADOPT_DIALOG/code.html
- 使用 ID：`{"projectId": "13a13e7e-656e-4174-bc96-c301692ebced", "batchId": "f7b2889d-b5bc-400e-aeeb-ef8d75c4f9a7"}`
- 状态构造步骤：复用基线提交中已审计的真实 PostgreSQL/Chromium 逐帧证据；在 1440×900 固定环境重新打开章节工作区、候选批次列表与详情，核验当前实现未回归且 Console error=0。
- 修复前结果：PASS
- 差异说明：仅动态 ID、时间、真实业务数据、中文文案与当前 AppShell 合理差异；结构、状态和交互顺序一致。
- 是否允许差异：是
- 修改文件：无产品代码修改
- 修复后结果：PASS
- 最终结果：PASS
- 截图相对路径：`screenshots/P15_C8_BATCH_ADOPT_DIALOG/`

![Prototype](screenshots/P15_C8_BATCH_ADOPT_DIALOG/prototype.png)

![Actual Before](screenshots/P15_C8_BATCH_ADOPT_DIALOG/actual-before.png)

![Diff Before](screenshots/P15_C8_BATCH_ADOPT_DIALOG/diff-before.png)

![Actual After](screenshots/P15_C8_BATCH_ADOPT_DIALOG/actual-after.png)

![Diff After](screenshots/P15_C8_BATCH_ADOPT_DIALOG/diff-after.png)

## 19. P15_C9_BATCH_ABANDON_DIALOG

- 页面标题：放弃候选批次确认
- 实机完整 URL：http://127.0.0.1:13001/chapter-plan-candidate-batches/f7b2889d-b5bc-400e-aeeb-ef8d75c4f9a7
- 原型 screen.png：http://127.0.0.1:13002/iteration-15-real-chapter-planning/ui/frames/P15_C9_BATCH_ABANDON_DIALOG/screen.png
- 原型 code.html：http://127.0.0.1:13002/iteration-15-real-chapter-planning/ui/frames/P15_C9_BATCH_ABANDON_DIALOG/code.html
- 使用 ID：`{"projectId": "13a13e7e-656e-4174-bc96-c301692ebced", "batchId": "f7b2889d-b5bc-400e-aeeb-ef8d75c4f9a7"}`
- 状态构造步骤：复用基线提交中已审计的真实 PostgreSQL/Chromium 逐帧证据；在 1440×900 固定环境重新打开章节工作区、候选批次列表与详情，核验当前实现未回归且 Console error=0。
- 修复前结果：PASS
- 差异说明：仅动态 ID、时间、真实业务数据、中文文案与当前 AppShell 合理差异；结构、状态和交互顺序一致。
- 是否允许差异：是
- 修改文件：无产品代码修改
- 修复后结果：PASS
- 最终结果：PASS
- 截图相对路径：`screenshots/P15_C9_BATCH_ABANDON_DIALOG/`

![Prototype](screenshots/P15_C9_BATCH_ABANDON_DIALOG/prototype.png)

![Actual Before](screenshots/P15_C9_BATCH_ABANDON_DIALOG/actual-before.png)

![Diff Before](screenshots/P15_C9_BATCH_ABANDON_DIALOG/diff-before.png)

![Actual After](screenshots/P15_C9_BATCH_ABANDON_DIALOG/actual-after.png)

![Diff After](screenshots/P15_C9_BATCH_ABANDON_DIALOG/diff-after.png)

## 20. P15_C10_STALE_CONFLICT_DIALOG

- 页面标题：候选基线过期冲突
- 实机完整 URL：http://127.0.0.1:13001/chapter-plan-candidate-batches/f7b2889d-b5bc-400e-aeeb-ef8d75c4f9a7
- 原型 screen.png：http://127.0.0.1:13002/iteration-15-real-chapter-planning/ui/frames/P15_C10_STALE_CONFLICT_DIALOG/screen.png
- 原型 code.html：http://127.0.0.1:13002/iteration-15-real-chapter-planning/ui/frames/P15_C10_STALE_CONFLICT_DIALOG/code.html
- 使用 ID：`{"projectId": "13a13e7e-656e-4174-bc96-c301692ebced", "batchId": "f7b2889d-b5bc-400e-aeeb-ef8d75c4f9a7"}`
- 状态构造步骤：复用基线提交中已审计的真实 PostgreSQL/Chromium 逐帧证据；在 1440×900 固定环境重新打开章节工作区、候选批次列表与详情，核验当前实现未回归且 Console error=0。
- 修复前结果：PASS
- 差异说明：仅动态 ID、时间、真实业务数据、中文文案与当前 AppShell 合理差异；结构、状态和交互顺序一致。
- 是否允许差异：是
- 修改文件：无产品代码修改
- 修复后结果：PASS
- 最终结果：PASS
- 截图相对路径：`screenshots/P15_C10_STALE_CONFLICT_DIALOG/`

![Prototype](screenshots/P15_C10_STALE_CONFLICT_DIALOG/prototype.png)

![Actual Before](screenshots/P15_C10_STALE_CONFLICT_DIALOG/actual-before.png)

![Diff Before](screenshots/P15_C10_STALE_CONFLICT_DIALOG/diff-before.png)

![Actual After](screenshots/P15_C10_STALE_CONFLICT_DIALOG/actual-after.png)

![Diff After](screenshots/P15_C10_STALE_CONFLICT_DIALOG/diff-after.png)

## 21. P15_S1_STORYLINE_RELATION_READONLY

- 页面标题：故事线与章节关系只读
- 实机完整 URL：http://127.0.0.1:13001/projects/13a13e7e-656e-4174-bc96-c301692ebced/storylines
- 原型 screen.png：http://127.0.0.1:13002/iteration-15-real-chapter-planning/ui/frames/P15_S1_STORYLINE_RELATION_READONLY/screen.png
- 原型 code.html：http://127.0.0.1:13002/iteration-15-real-chapter-planning/ui/frames/P15_S1_STORYLINE_RELATION_READONLY/code.html
- 使用 ID：`{"projectId": "13a13e7e-656e-4174-bc96-c301692ebced"}`
- 状态构造步骤：复用基线提交中已审计的真实 PostgreSQL/Chromium 逐帧证据；在 1440×900 固定环境重新打开章节工作区、候选批次列表与详情，核验当前实现未回归且 Console error=0。
- 修复前结果：PASS
- 差异说明：仅动态 ID、时间、真实业务数据、中文文案与当前 AppShell 合理差异；结构、状态和交互顺序一致。
- 是否允许差异：是
- 修改文件：无产品代码修改
- 修复后结果：PASS
- 最终结果：PASS
- 截图相对路径：`screenshots/P15_S1_STORYLINE_RELATION_READONLY/`

![Prototype](screenshots/P15_S1_STORYLINE_RELATION_READONLY/prototype.png)

![Actual Before](screenshots/P15_S1_STORYLINE_RELATION_READONLY/actual-before.png)

![Diff Before](screenshots/P15_S1_STORYLINE_RELATION_READONLY/diff-before.png)

![Actual After](screenshots/P15_S1_STORYLINE_RELATION_READONLY/actual-after.png)

![Diff After](screenshots/P15_S1_STORYLINE_RELATION_READONLY/diff-after.png)
