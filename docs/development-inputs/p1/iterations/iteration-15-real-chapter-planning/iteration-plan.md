# Iteration 15 — 真实章节规划

## 1. 目标

通过项目绑定的内置 LLM 或 n8n 工作流生成待确认章节规划候选。

## 2. 用户闭环

真实工作流生成候选，用户确认后进入正文生产；非法输出不写入候选。

## 3. 数据模型

- `WorkflowRun(stage=chapter_planning)`
- `ChapterPlan.sourceWorkflowRunId`

详细字段和约束见 `data-model.md`。

## 4. API

- POST /api/v1/projects/{projectId}/chapter-plan-runs
- 复用 ChapterPlan 编辑/删除/确认 API

冻结范围见 `api-scope.yaml`。

## 5. UI 与原型关联

- `C1_CHAPTER_PLANNING_V2` → `ui/frames/C1_CHAPTER_PLANNING_V2/screen.png`
- `C2_GENERATE_CHAPTER_PLAN_DRAWER_V2` → `ui/frames/C2_GENERATE_CHAPTER_PLAN_DRAWER_V2/screen.png`
- `STATE_TASK_RUNNING_BAR` → `ui/frames/STATE_TASK_RUNNING_BAR/screen.png`
- `STATE_TASK_FAILED_NOTICE` → `ui/frames/STATE_TASK_FAILED_NOTICE/screen.png`
- `STATE_NOT_CONFIGURED_EMPTY` → `ui/frames/STATE_NOT_CONFIGURED_EMPTY/screen.png`

详细状态和开发约束见 `ui-scope.md` 与 `ui-manifest.json`。

## 6. 实施顺序

1. 读取冻结 OpenAPI、数据模型和原型；
2. 后端先实现领域模型、迁移、Repository、Service 与 API；
3. 前端可基于冻结契约和原型并行开发；
4. 先使用 Mock Adapter 验证全部页面状态；
5. 人工 UI 验收后接入真实 API；
6. 局部测试通过后执行分组测试；
7. 最终执行全链路 E2E、Code Review 和 Git 验收。

## 7. 不在范围

- 不接入 Coze、ComfyUI 或其他工作流平台；
- 不建设 n8n 可视化编辑器或字段映射器；
- 不实现多 n8n 实例路由、费用大盘或自动模型路由；
- 不允许业务页面临时切换工作流；
- 不静默改变第一闭环 Mock 契约。

## UI 条件通过与开发修正规则

Iteration 11 的人工验收结论为：**有条件通过**。

已知问题：部分 Stitch 原型文案为英文，尤其可能出现在左侧一级菜单、状态标签、表头、按钮、辅助说明和技术占位文案中。原型中的英文不构成最终产品文案冻结。

开发必须满足：

1. 默认中文环境下，用户可见文案全部使用统一中文资源，不得直接复制 HTML 中的英文硬编码；
2. 左侧一级菜单统一为：首页、项目、素材、作品、工作流、设置；
3. 状态统一为：排队中、运行中、已成功、已失败、未验证、验证成功、已停用、配置异常；
4. `Run ID`、`Workflow ID`、Schema 版本、模型名、API 名称等技术标识可以保留英文；
5. 所有业务文案进入前端 i18n/locale 资源；组件不得内嵌不可替换英文；
6. 人工 UI 验收增加“中文文案与术语一致性”专项，发现英文用户文案即不通过。

## 8. 完成定义

- [ ] 数据模型、API 和 UI 三者一致；
- [ ] 所有用户动作都有反馈、数据结果和失败恢复；
- [ ] 原型关联文件存在且可打开；
- [ ] 验收项全部通过；
- [ ] 变更报告、测试报告与 Git 状态完整。
