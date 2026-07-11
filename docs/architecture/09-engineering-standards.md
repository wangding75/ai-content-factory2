# 工程规范

## 1. 迭代流程

```text
计划
→ 审核
→ 契约
→ 实现
→ 自动化验收
→ 差异说明
→ 单次 Git commit
→ 工作区干净
```

## 2. Git

- 每个迭代一个主要提交。
- 提交前运行 `git diff --check`。
- 不提交 `.env`、构建产物和本地数据。
- 状态记录在 `.ai-dev/state.json`。

## 3. 后端

- Domain 规则必须有单元测试。
- Repository 使用 PostgreSQL 集成测试。
- Handler 只负责协议映射。
- 错误使用稳定 code，不依赖字符串判断。

## 4. Web

- 页面不得直接调用 fetch 拼装匿名结构。
- API DTO 与 View Model 分离。
- 加载、空、错误和禁用状态不可省略。
- E2E 必须验证真实数据变化。

## 5. 文档

任何需求或实现漂移必须更新相关 PRD、规则、契约、迭代文档和追踪矩阵。
