# 09.F6F P0 UI / API 验收

状态：PASS

## 环境与固定数据

- 基线：`228e8c21bf8a7adbbb31c2710180a58c040ad805` / `main`
- 环境：Docker Compose production，Web `http://127.0.0.1:13001`，API `http://127.0.0.1:18080/api/v1`
- 测试项目：`f6e00000-0000-4000-8000-000000000003`（F6E 星港进行中系列）
- 固定数据标识：`created_by=acf-test-data-f6e`；加载及校验脚本：`scripts/test-data/09-f6e-unified-p0.ps1 -Action Load|Verify`

## 页面与功能验收

| 页面/功能 | 固定数据与真实 API | 结果 |
| --- | --- | --- |
| 项目列表：类型、状态、名称筛选 | `GET /projects`，验证 `status=producing` 和 `q=F6E` 请求参数 | PASS |
| 项目创建入口与返回 | `/projects` → `/projects/new` → `/projects` | PASS |
| 项目概览、策划 | `GET /projects/{id}/workspace` | PASS |
| 项目素材 | `GET /projects/{id}/materials`、`GET /materials/{id}` | PASS |
| 故事线与子故事线 | `GET /projects/{id}/storylines`；主线及两条子线 | PASS |
| 伏笔三种状态 | `GET /projects/{id}/foreshadowings`；planned/planted/paid_off | PASS |
| 章节规划 | `GET /projects/{id}/chapter-plans`；两条 confirmed、一条 pending_confirmation | PASS |
| 正文与审核 | `GET /chapter-plans/{id}/content`、`GET /review-reports/*` | PASS |
| 项目作品、审核 Tab | `GET /projects/{id}/works`；正文、版本、审核与重写入口 | PASS |
| 工作流 | `GET /workflows`、`GET /workflow-runs` | PASS |
| 全局素材 | `GET /materials` | PASS |
| 全局作品 | `GET /works` | PASS |
| 全局流程、设置 | `GET /workflows`、`GET /settings` | PASS |

每个数据页均在 Chromium 中加载真实 API 后滚动到底复核。18 个 P0 路由的回归检查确认无 console/page error、失败请求、横向溢出及 UUID/provider/ISO 时间泄露。

## 本轮修复

- `apps/web/src/features/planning-materials/components/project-workspace-frame.tsx`
  - 原始错误：`/projects/{id}/works` 触发 `Minified React error #418`。
  - 根因：SSR 容器按 UTC 格式化 `project.updated_at`（`2026年1月4日 08:00`），浏览器按本地 Asia/Shanghai 格式化同一值（`16:00`），文本不一致导致 hydration 失败。
  - 修复：使用 `Intl.DateTimeFormat` 并明确 `timeZone: "Asia/Shanghai"`，使 SSR 与客户端输出稳定一致。
  - 局部验证：该路由无 hydration/page error，显示 `2026年1月4日 16:00`。

## ui-prototype-comparison 反馈闭环

| 反馈 | 闭环状态 | 说明 |
| --- | --- | --- |
| 项目列表胶囊筛选与新建模态 | 尚未实现 | 当前筛选与新建全页路径可用，视觉形态未在本轮改动。 |
| 流程中心/设置中文化 | 尚未实现 | 真实 API 页面可用；文案重构不属于本轮根因修复。 |
| 全局素材统计与类型胶囊 | 尚未实现 | 数据读取正常，视觉增强未实施。 |
| 正文编辑器右侧上下文 | 尚未实现 | 本轮正文真实数据、审核链路可用；编辑器信息架构未改。 |
| 章节规划 CTA/说明 | 尚未实现 | 确认与待确认状态均可读取，视觉 CTA 未改。 |
| 项目卡、概览、作品、故事线的视觉密度差异 | 非本轮范围 | 本任务修复真实 API 联调与确认缺陷，不重构冻结 UI。 |
| 历史 `????` 项目名 | 无法复现 | F6E 数据及当前页面未出现该名称。 |
| 概览故事线计数可能不一致 | 无法复现 | 固定数据下概览、故事线和章节读取均通过；未观察到不一致。 |

已修复：本轮没有可归因于上述历史视觉对比项的新增修复；本轮修复的是验收中确认的作品页 hydration 缺陷，见“本轮修复”。

## 验证证据

- 数据加载与校验：PASS。
- API 最小健康检查：`GET /api/v1/meta`，HTTP 200。
- 定向 lint：PASS；TypeScript typecheck：PASS。
- Docker production verification：PASS；本轮实际 build 1 次（api、migrate、web 均由既有指纹状态刷新触发，Web 包含本轮修复）。
- Chromium route verification：PASS，18 路由。
- 未新增临时截图、浏览器缓存或测试垃圾文件；运行报告仅位于已忽略的 `.ai-dev/reports/`。

## 剩余问题

无阻塞 P0 验收项。上述尚未实现的原型视觉差异需在独立 UI 设计任务处理，不影响本轮真实 API 功能验收。
