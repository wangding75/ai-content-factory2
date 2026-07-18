# Agent 执行规范

本目录用于沉淀 AI Content Factory 2.0 的长期固定执行规则，减少后续 Codex 指令中的重复内容。

## 必读顺序

每个开发任务开始前依次读取：

1. `00-project.md`
2. `01-development-standard.md`
3. `02-review-standard.md`

任务指令只描述：

- 当前业务目标
- 允许与禁止的特殊范围
- 本任务特有验收项
- 推荐模型与推理等级

固定环境、工程规范、测试门禁、提交规范和回执格式不再重复写入任务指令。

## 规则优先级

发生冲突时按以下顺序执行：

1. 当前任务指令中的明确特殊约束
2. `02-review-standard.md`
3. `01-development-standard.md`
4. `00-project.md`

不得将模糊描述视为对固定规范的覆盖。只有明确写出的例外才有效。

## 后续最简任务模板

```text
读取并遵守：

- docs/agent/00-project.md
- docs/agent/01-development-standard.md
- docs/agent/02-review-standard.md

执行：<任务编号>

目标：
- <业务目标 1>
- <业务目标 2>

范围：
- 允许：<特殊允许范围>
- 禁止：<特殊禁止范围>

特殊验收：
- <仅本任务新增的验收项>

完成后独立 commit、push，返回完整 commit hash，然后停止。
```
