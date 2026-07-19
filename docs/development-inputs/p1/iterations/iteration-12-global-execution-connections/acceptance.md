# Iteration 12 — 全局配置中心 V2 — 验收标准

## 1. 原型和页面组织

- [ ] 12 个 Frame 目录全部存在，并同时包含非空 `screen.png` 和 `code.html`；
- [ ] Frame 命名与 `ui-manifest.json` 一致；
- [ ] 开发使用规范化 Frame 名称；
- [ ] 页面壳层复用共享组件，只开发标题下方工作区；
- [ ] 空状态仅保留工作区右上角一个添加按钮；
- [ ] 用户可见业务文案全部进入中文 locale/i18n 资源。

## 2. 范围边界

- [ ] Iteration 12 只实现类型目录、列表、创建、详情和修改；
- [ ] 正式 OpenAPI 不包含 `/verify`、`/enable`、`/disable` 或 LLM `/models`；
- [ ] 后端没有 LLM、n8n、微信公众号、抖音、YouTube 等 Adapter；
- [ ] 后端没有第三方 HTTP Client 调用路径；
- [ ] 自动测试能够证明本迭代业务请求不会发出第三方网络请求；
- [ ] 不创建 WorkflowRun；
- [ ] 不执行 OAuth；
- [ ] 不执行真实发布；
- [ ] 不实现 callback server；
- [ ] 不提供 DELETE 接口。

## 3. LLM Provider

- [ ] 空状态、列表、添加、详情和编辑闭环可用；
- [ ] Provider 类型创建后不可修改；
- [ ] Base URL、默认模型和超时只保存，不发起访问；
- [ ] API Key 创建可填写，编辑不回显；
- [ ] PATCH 不提供新 API Key 时保留原密钥；
- [ ] 保存后 `integrationStatus=not_connected`；
- [ ] 保存后 `enabled=false`；
- [ ] 验证、获取模型和启用控件禁用并提供后续开放说明。

## 4. 工作流连接

- [ ] 页面和 API 使用通用“连接”概念；
- [ ] 新增连接时连接类型可选择，当前选项为 n8n；
- [ ] 编辑连接时连接类型只读且后端拒绝修改；
- [ ] 列表展示连接类型；
- [ ] 连接凭证加密保存且不回显；
- [ ] 不访问 n8n Base URL；
- [ ] 不验证 API Key；
- [ ] 保存后 `integrationStatus=not_connected`；
- [ ] 保存后 `enabled=false`；
- [ ] 验证和启用控件禁用。

## 5. 工作流配置

- [ ] 创建工作流必须选择关联连接；
- [ ] 客户端不能提交或修改工作流类型；
- [ ] API 根据连接类型返回派生的 `workflowType`；
- [ ] 列表展示关联连接和连接类型；
- [ ] 不存在无连接的系统内置工作流记录；
- [ ] 修改关联连接后类型同步更新；
- [ ] 连接类型变化后旧类型专属字段被清空；
- [ ] 当前 n8n 类型仅执行 Workflow ID / Webhook Path 的本地 Schema 校验；
- [ ] 不访问 Workflow ID 或 Webhook Path；
- [ ] 保存后 `integrationStatus=not_connected`；
- [ ] 保存后 `enabled=false`。

## 6. 分发平台配置

- [ ] 支持微信公众号、抖音、YouTube 和自定义平台类型目录；
- [ ] 平台专属字段根据类型动态渲染；
- [ ] 平台类型创建后不可修改；
- [ ] 凭证创建可填写、编辑不回显；
- [ ] 不验证平台凭证；
- [ ] 不执行 OAuth；
- [ ] 不创建真实发布任务；
- [ ] 保存后 `integrationStatus=not_connected`；
- [ ] 保存后 `enabled=false`；
- [ ] 验证和启用控件禁用。

## 7. P0 开发修复项

- [ ] P0-1：工作流列表移除旧类型列和系统内置示例；
- [ ] P0-2：连接抽屉新增态连接类型可选，编辑态只读；
- [ ] P0-3：连接与工作流抽屉使用真实父列表页背景；
- [ ] P0-4：统一复用共享侧边栏和页面头部；
- [ ] 重复添加按钮统一为一个；
- [ ] n8n 特定文案只在选择 n8n 类型后显示。

## 8. 契约和安全

- [ ] 仓库正式 OpenAPI 是唯一契约源；
- [ ] DTO 不含密文或明文凭证；
- [ ] AES-256-GCM、随机 nonce、主密钥校验和日志脱敏测试通过；
- [ ] 乐观锁、幂等、审计和凭证保留/替换/清除语义测试通过；
- [ ] 审计动作不包含 verify、enable、disable、execute 或 publish；
- [ ] 任何日志和错误都不包含凭证。

## 9. 工程验收

- [ ] 先执行新增模块局部测试；
- [ ] 再执行 API/Web 分组测试；
- [ ] 最后只运行一次相关总门禁；
- [ ] 未定位根因前不得反复完整重跑；
- [ ] 类型检查、构建、契约测试、集成测试和 E2E 通过；
- [ ] 完成独立 Code Review；
- [ ] 最终报告包含修改清单、`git diff --name-status`、未跟踪文件和 `git status --short`。
