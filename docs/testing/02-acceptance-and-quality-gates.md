# 验收与质量门禁

每个迭代完成前必须通过：

1. 核心需求验收用例。
2. Go test。
3. Repository integration tests。
4. OpenAPI / Schema contract tests。
5. Web lint 和 typecheck。
6. 本迭代 Playwright E2E。
7. Migration 空库验证。
8. `git diff --check`。
9. 文档和 `.ai-dev` 状态更新。
10. 工作区干净。

不接受：

- 只验证页面能打开。
- 只验证 HTTP 200。
- 使用前端 Mock 代替最终验收。
- 未验证刷新持久化。
- 未验证失败分支数据库状态。
