# 验收原则

## 核心要求

每个迭代的核心需求必须有“创建数据 → 执行动作 → 读取结果”的可验证用例，不能只检查页面可打开或接口返回 200。

示例：

```text
新增一个项目
→ 返回项目列表
→ 列表展示刚新增的项目
→ 刷新页面后仍然展示
→ 数据库只有一条对应记录
```

## 测试分层

1. Domain Unit Test：状态机、业务规则、值对象。
2. Application Test：用例、事务、幂等、错误分支。
3. Repository Integration Test：PostgreSQL Testcontainers。
4. Contract Test：OpenAPI 请求、响应、错误码、request_id。
5. Web Test：页面状态、表单校验、API 调用。
6. Playwright E2E：真实页面 + API + 数据库闭环。
7. Iteration 09：从空数据库运行完整 P0。

## UI 验收

- 冻结 Frame 是业务和视觉基线。
- 不要求逐像素完全一致，但不允许字段、入口、状态和信息架构漂移。
- 页面必须覆盖 loading、empty、error、disabled 和 success。
- 模态框/抽屉的保存、取消、关闭必须回到正确来源。
