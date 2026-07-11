# ADR-0003：生成、审核、重写统一使用 Workflow Provider

- 状态：Accepted
- 决策：所有执行都通过 Provider 接口并创建 WorkflowRun。
- 原因：隔离 Mock、LLM 和外部工作流差异，保持领域模型稳定。
- 后果：Provider 不得直接写领域表，必须返回结果给 Application 用例。
