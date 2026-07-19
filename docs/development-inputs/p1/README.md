# AI Content Factory 2.0 — P1 第二用户闭环开发输入

## 范围

- 真实章节规划；
- 真实正文生成；
- 真实内容审核；
- 真实正文重写；
- OpenAI-compatible LLM Provider；
- 单 n8n 实例；
- 项目四环节工作流绑定；
- WorkflowRun 异步运行管理。

## 目录规则

各迭代严格沿用第一闭环结构：

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

## UI 验收结论

Iteration 11：有条件通过。开发必须修正用户可见英文文案，尤其是一级菜单、状态、按钮和表头；技术标识可保留英文。

## 开发顺序

契约冻结后后端先行，前端基于冻结 OpenAPI 和原型并行开发；真实 API 联调前完成人工 UI 验收。
