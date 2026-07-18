# AI Content Factory 2.0 Agent Entry

本仓库中的执行型任务必须直接执行，不得重新制定计划，不得重新评估已冻结方案。

开始任务前依次读取：

1. `docs/agent/00-project.md`
2. `docs/agent/01-development-standard.md`
3. `docs/agent/02-review-standard.md`
4. 当前任务对应的冻结任务文件、迭代资料与 UI / API 契约

规则优先级：

1. 当前任务中的明确特殊约束
2. `docs/agent/02-review-standard.md`
3. `docs/agent/01-development-standard.md`
4. `docs/agent/00-project.md`

固定检查、测试、Docker、浏览器冒烟和提交操作优先使用 `scripts/agent/` 下的统一脚本。

任务结束后必须独立 commit、push、返回完整 commit hash，并停止，不得自动开始下一任务。
