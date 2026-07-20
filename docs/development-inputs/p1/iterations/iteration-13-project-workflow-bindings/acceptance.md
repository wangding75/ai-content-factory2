# Iteration 13 — 项目四环节工作流绑定 — 验收标准（冻结）

## 1. 功能闭环

- [ ] 页面固定展示章节规划、内容生成、审核、改写四个环节；
- [ ] 四个环节拥有独立且唯一的项目绑定；
- [ ] 候选工作流按 `applicableStage` 过滤；
- [ ] 已停用工作流不可作为新绑定选择；
- [ ] 可以创建、替换和解除绑定；
- [ ] 解除绑定不删除全局工作流，也不影响其他项目；
- [ ] 刷新后从真实 API 与 PostgreSQL 恢复最新状态；
- [ ] 不存在跨项目绑定串联；
- [ ] 项目策划快捷入口和概览动态"下一步建议"统一跳转到项目设置的工作流绑定页。

## 2. 并发、幂等与 Audit

- [ ] PUT 和 DELETE 使用 `Idempotency-Key`；
- [ ] 相同 Key、相同载荷重放返回同一业务结果；
- [ ] 相同 Key、不同载荷返回统一 409（idempotency_key_reused_with_different_payload）；
- [ ] 更换和解除使用 `expectedVersion`；
- [ ] 409 冲突 details 包含 expectedVersion、currentVersion、projectId、stage；
- [ ] 409 保留用户当前选择，并支持加载服务端最新配置；
- [ ] 幂等重放不产生重复绑定或重复 Audit；
- [ ] 创建、更换、解除的业务写入、幂等记录和 Audit 位于同一事务；
- [ ] Audit 不包含 Secret、Credential、完整敏感 URL、配置正文或 Idempotency-Key 明文。

## 3. UI 验收

- [ ] 使用现有项目工作区壳层，没有重复侧栏、顶部栏、项目标题、面包屑和项目 Tab；
- [ ] `P13-01` 只作为入口参考，本迭代没有扩展项目基础信息或其他设置；
- [ ] 内层 Tab 统一为"基本信息 / 工作流绑定 / 其他设置"；
- [ ] 四个环节术语统一为"章节规划 / 内容生成 / 审核 / 改写"；
- [ ] 2×2 卡片顺序固定、等高、操作区对齐；
- [ ] 长名称、窄屏和滚动状态不溢出；
- [ ] 选择/更换抽屉具有固定 Header/Footer、内部滚动、焦点锁定和 Escape 关闭；
- [ ] 已停用候选不可选；
- [ ] 未接入、已停用和连接异常的语义明确；
- [ ] 健康绑定可以更换或解除；
- [ ] 无候选状态提供全局设置入口和返回项目上下文；
- [ ] 409 不丢失用户选择；
- [ ] 用户可见业务文案全部来自中文 locale/i18n；
- [ ] 不出现 Secret、Credential、Webhook、原始 UUID、JSON 或执行按钮。

## 4. API 与数据

- [ ] OpenAPI、数据库模型、实现和 UI 字段一致；
- [ ] `UNIQUE(project_id, stage)` 生效；
- [ ] 不存在 `ChapterPlanningParameters` 等项目参数覆盖模型；
- [ ] 不存在 validate/enable/disable 项目绑定接口；
- [ ] 不创建 `WorkflowRun`；
- [ ] 本迭代没有第三方出站请求；
- [ ] 全局工作流后续异常不会静默删除现有绑定。

## 5. 测试与工程门禁

- [ ] Domain、Repository、Service 和 Handler 定向测试通过；
- [ ] 真实 PostgreSQL 集成测试通过；
- [ ] 四环节创建、换绑、解绑、幂等、409 和跨项目隔离测试通过；
- [ ] 前端列表、抽屉、无候选、异常和冲突测试通过；
- [ ] ESLint、Typecheck、Production Build 和 Contract Test 通过；
- [ ] 浏览器真实 API 回归通过，未使用 Mock；
- [ ] Console 无新增 Error，Network 无非预期业务 4xx/5xx；
- [ ] 先局部、再分组、最后总门禁，未定位根因前不反复完整重跑；
- [ ] `git diff --name-status`、未跟踪文件和 `git status --short` 已记录；
- [ ] 独立 Code Review 和人工 UI 验收完成。

## 6. 冻结状态

验收标准已冻结。所有验收条目与冻结契约一致。