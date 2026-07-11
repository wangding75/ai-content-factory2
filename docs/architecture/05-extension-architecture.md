# 扩展架构

## 1. Content Pack

Content Pack 只定义内容类型差异：

- 项目策划 Schema。
- 素材类型和用途类型。
- 生产单元类型。
- 章节计划扩展字段。
- 内容审核维度。

P0：

```text
pack_key=novel
production_unit=chapter
```

禁止在通用模块散落 `if project.type == novel`。

## 2. Workflow Provider

Provider 统一生成、审核和重写执行：

```text
novel.chapter_plan.mock_generate
novel.content.mock_generate
novel.review.mock_review
novel.rewrite.mock_rewrite
```

P0 只有 Mock Provider，但接口必须支持后续 LLM 和外部工作流适配器。

## 3. Adapter 演进

未来适配器：

- LLM Provider。
- n8n Adapter。
- Coze Adapter。
- ComfyUI Adapter。
- Publishing Adapter。

适配器不得直接修改领域表；必须通过 Application Port 和领域用例提交结果。
