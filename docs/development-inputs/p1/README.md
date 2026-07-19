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

## 开发顺序

契约冻结后，后端先完成数据、安全和 Application Service 基础；前端基于冻结 OpenAPI 和规范化 Frame 并行开发。真实 API 联调前先完成人工 UI 验收。
