# ACF 测试策略

## 1. 适用范围

本策略覆盖 AI Content Factory 2.0 的 Domain、Application、Repository、HTTP Contract、Web 和 E2E 测试。

P0/Mock 场景继续作为回归基线保留。Iteration 15～18 已完成浏览器 UI 验收，但真实 n8n、真实 LLM 和第二用户闭环最终验收尚未完成。

## 2. 测试分层

- Domain unit：状态机、值对象、领域规则和纯函数。
- Application：成功、失败、事务、幂等、版本冲突和权限边界。
- Repository integration：在真实 PostgreSQL 数据库 `ai_content_factory` 上验证 Repository 行为。
- HTTP contract：OpenAPI、JSON Schema、错误码、Envelope 和 `request_id`。
- Web logic：API Client、数据转换、校验和状态计算。
- Web interaction：页面状态、弹窗、表单、错误提示、轮询和刷新恢复。
- E2E：跨 Web、API、WorkflowRun 和数据库的用户闭环。
- 人工验收：视觉、业务路径和不可完全自动化的交互检查。

## 3. 唯一数据库规则

所有数据库相关开发、测试和验收只使用：

```text
ai_content_factory
```

不创建开发库、测试库、迭代库或验收库，不创建 `ai_content_factory_test`、`ai_content_factory_http_test` 等派生数据库。

数据库规则：

1. 所有迭代在当前数据库上顺序执行增量 Migration。
2. 已提交的历史 Migration 和 SQL 文件不得修改、重排、合并或重新生成。
3. 数据库变化只能新增 Migration。
4. 测试不得执行 `DROP DATABASE`、`DROP SCHEMA`、全库 `TRUNCATE`、Migration Down 或数据库全量重建。
5. Repository 测试优先使用事务包裹，并在用例结束时 Rollback。
6. 无法在同一事务中完成的跨事务测试使用唯一 Run ID、项目名或业务前缀隔离数据。
7. 测试数据只能根据明确主键、Run ID 或关联 ID 定向清理。
8. 测试前检查数据库名称和当前 Migration Head。
9. 测试后确认历史业务数据仍可读取，且未产生无关联或重复领域记录。
10. 缺少数据库连接或 Migration 状态不正确时，必须明确失败，不能将必要的数据库测试静默标记为 Skip 后宣称门禁通过。

## 4. 必测失败分支

基础回归：

- 非法项目请求不写入数据。
- 重复绑定素材不新增 Usage。
- 已确认计划不可编辑或删除。
- 未确认计划不能创建正文。
- 审核不修改源正文。
- 重写失败不产生半成品版本。
- 禁用集成不创建虚假运行记录。

第二用户闭环相关实现需要继续覆盖：

- 幂等请求不产生重复 Run、候选版本或报告。
- 当前版本变化时拒绝使用过期预检结果。
- 外部执行失败时不产生非法领域终态。
- 上游成功、结果消费失败时可恢复，且不重复调用或重复写入。
- 页面刷新后状态与数据库一致。

真实 LLM、完整 n8n 异常矩阵和最终关闭用例在进入对应最终联调阶段后另行冻结，当前不得以确定性工作流结果代替真实 LLM 验收。

## 5. 测试数据

- 保留固定系统用户和必要的 Mock Provider Seed。
- 每个测试使用独立项目命名空间或唯一 Run ID。
- 测试可重复运行，不依赖执行顺序。
- 数据清理必须定向执行，不清空 `ai_content_factory`。
- 失败数据如需保留取证，应使用明确前缀并在报告中记录。

## 6. 结果判定

测试报告必须区分：

- 实际执行并通过。
- 因外部条件缺失而未执行。
- 用户明确豁免。
- 已知测试债务。

浏览器 UI 验收、Mock 回归、真实 n8n 联调和真实 LLM 验收必须分别记录，不能合并为一个笼统的“全链路通过”。
