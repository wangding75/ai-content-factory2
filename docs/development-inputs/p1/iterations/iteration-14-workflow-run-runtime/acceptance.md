# Iteration 14 — n8n 适配与 WorkflowRun 异步运行 — 验收标准

## 1. n8n Adapter

- [ ] 通用 WorkflowAdapter 契约存在；
- [ ] n8n Adapter 实现连接验证、工作流验证和执行；
- [ ] 凭证只在 Worker 内短暂解密；
- [ ] 日志不包含凭证、Authorization Header 或原始响应；
- [ ] 超时、HTTP 错误和响应格式错误被归一化；
- [ ] Adapter 契约测试使用可控测试服务器或测试 n8n 环境。

## 2. 连接和工作流状态

- [ ] 连接可以从 not_connected/unverified 进入 verified 或 failed；
- [ ] 只有 verified 连接可以启用；
- [ ] 数据库保证最多一个启用连接；
- [ ] 启用新连接原子停用旧连接；
- [ ] 工作流真实验证 Workflow ID / Webhook Path；
- [ ] 只有连接已验证且启用，工作流才能启用；
- [ ] 配置变化后验证状态正确重置。

## 3. WorkflowRun

- [ ] API 可以创建幂等 queued 运行；
- [ ] Worker 将运行推进到 running；
- [ ] 成功推进到 succeeded；
- [ ] 失败推进到 failed；
- [ ] 刷新后恢复运行和事件；
- [ ] 绑定快照不受后续配置变更影响；
- [ ] 快照不包含凭证；
- [ ] 输出 Schema 非法时不写领域结果；
- [ ] 领域提交失败时不重复调用 n8n；
- [ ] 完整重试创建新运行并保留原记录；
- [ ] 领域提交重试只重试事务提交。

## 4. UI

- [ ] Workflow Center 列表、详情、事件和重试可用；
- [ ] 运行中、成功、失败和未配置状态使用中文资源；
- [ ] Iteration 12 连接和工作流页面的验证/启用控件在本迭代激活；
- [ ] 错误信息脱敏且提供恢复动作；
- [ ] 页面刷新状态一致。

## 5. 架构边界

- [ ] 不实现 callback server；
- [ ] n8n 结果通过 Webhook HTTP 响应返回；
- [ ] 不验证或调用 LLM Provider；
- [ ] 不获取 LLM 模型列表；
- [ ] 不验证分发平台；
- [ ] 不执行 OAuth 或内容发布；
- [ ] 不实现 Coze、ComfyUI 等 Adapter。

## 6. 工程验收

- [ ] 先执行 Adapter 和状态机局部测试；
- [ ] 再执行 API、Worker 和 Web 分组测试；
- [ ] 最后只运行一次全链路总门禁；
- [ ] 未定位根因前禁止反复完整重跑；
- [ ] 类型检查、构建、契约测试、集成测试和 E2E 通过；
- [ ] `git diff --name-status`、未跟踪文件和 `git status --short` 已记录；
- [ ] 独立 Code Review 完成。
