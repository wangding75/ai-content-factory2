# AI Content Factory 2.0 — P1 第二用户闭环开发输入

## 范围

- 真实章节规划；
- 真实正文生成；
- 真实内容审核；
- 真实正文重写；
- OpenAI-compatible LLM Provider；
- 通用工作流连接，当前连接类型仅支持 n8n；
- 全局可复用工作流配置；
- 分发平台配置管理；
- 项目四环节工作流绑定；
- WorkflowRun 异步运行管理。

## 目录规则

各迭代沿用第一闭环结构：

```text
iteration-xx-name/
├── acceptance.md
├── api-scope.yaml
├── closed-loop.md
├── data-model.md
├── iteration-plan.md
├── ui-manifest.json
├── ui-scope.md
└── ui/frames/<FRAME_ID>/
    ├── code.html
    └── screen.png
```

Iteration 12 额外包含：

- `prototype-review-and-development-fixes.md`；
- `prototype-source-mapping.md`。

## UI 验收结论

Iteration 11：有条件通过。

Iteration 12 全局设置 V2 原型：有条件通过，P0-1/P0-2/P0-3、页面壳层差异和重复按钮均在开发阶段强制修复，不再要求修改 Stitch 原型。

Iteration 13 项目四环节工作流绑定：已完成（completed, passed_with_user_waiver）。CF-13-05 人工 UI 验收 PASS，CF-13-06 真实 API 联调 PASS WITH WAIVER，历史 E2E 测试债务获用户豁免。下一迭代：14。

Iteration 14 WorkflowRun Runtime 与执行器抽象：已完成（completed, accepted）。机器验收 PASS，人工 UI 验收 PASS（2026-07-23）；最终业务代码为 `7b6d1e8fa64cb216e3b8645b6e596b503ce8379c`，验收证据保留在本地 `.ai-dev/reports/CF-14-03D/`。下一迭代：15。

Iteration 15 真实章节规划与候选批次闭环：已完成开发输入升级（prepared_for_contract_freeze）。开发输入入口为 `iterations/iteration-15-real-chapter-planning/`，旧版 5 Frame 已替换为 21 Frame，新增预检、候选批次、差异、采用、放弃、版本冲突和故事线章节职责修正。当前尚未冻结主 OpenAPI、尚未开始开发。最终真实生成验收依赖独立的 `CF-14-N8N-Integration`，Iteration 15 不重复建设通用 n8n Transport、Worker 或 Callback。

## 开发顺序

契约冻结后，后端先完成数据、安全和 Application Service 基础；前端基于冻结 OpenAPI 和规范化 Frame 并行开发。真实 API 联调前先完成人工 UI 验收。
