# P0 测试策略

## 1. 测试金字塔

- Domain unit：状态机、值对象、规则。
- Application：成功、失败、事务、幂等和权限边界。
- Repository integration：真实 PostgreSQL。
- HTTP contract：OpenAPI、错误码、request_id。
- Web component：视图状态和交互。
- E2E：跨 Web、API 和数据库闭环。

## 2. 必测失败分支

- 非法项目请求无数据。
- 重复绑定素材不新增 Usage。
- 已确认计划不可编辑或删除。
- 未确认计划不能创建正文。
- 审核不修改源正文。
- 重写失败不产生半成品版本。
- 禁用集成不创建虚假运行记录。

## 3. Test Data

- 固定系统用户。
- 固定 Mock Provider seed。
- 每个测试创建独立项目命名空间。
- 测试可重复运行，不依赖执行顺序。
