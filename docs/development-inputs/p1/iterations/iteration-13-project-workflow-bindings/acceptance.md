# Iteration 13 — 项目四环节工作流绑定 — 验收标准（冻结）

## 1. 功能闭环

- [x] 页面固定展示章节规划、内容生成、审核、改写四个环节；
- [x] 四个环节拥有独立且唯一的项目绑定；
- [x] 候选工作流按 `applicableStage` 过滤；
- [x] 已停用工作流不可作为新绑定选择；
- [x] 可以创建、替换和解除绑定；
- [x] 解除绑定不删除全局工作流，也不影响其他项目；
- [x] 刷新后从真实 API 与 PostgreSQL 恢复最新状态；
- [x] 不存在跨项目绑定串联；
- [x] 项目策划快捷入口和概览动态"下一步建议"统一跳转到项目设置的工作流绑定页。

## 2. 并发、幂等与 Audit

- [x] PUT 和 DELETE 使用 `Idempotency-Key`；
- [x] 相同 Key、相同载荷重放返回同一业务结果；
- [x] 相同 Key、不同载荷返回统一 409（idempotency_key_reused_with_different_payload）；
- [x] 更换和解除使用 `expectedVersion`（DELETE 通过 query parameter 承载，复用现有 DELETE 惯例）；
- [x] 409 冲突 details 包含 expectedVersion、currentVersion、projectId、stage；
- [x] 409 保留用户当前选择，并支持加载服务端最新配置；
- [x] 幂等重放不产生重复绑定或重复 Audit；
- [x] 创建、更换、解除的业务写入、幂等记录和 Audit 位于同一事务；
- [x] Audit 不包含 Secret、Credential、完整敏感 URL、配置正文或 Idempotency-Key 明文；
- [x] 401 返回 `unauthenticated`，403 返回 `forbidden`，解绑 404 返回 `workflow_binding_not_found`。

## 3. UI 验收

- [x] 使用现有项目工作区壳层，没有重复侧栏、顶部栏、项目标题、面包屑和项目 Tab；
- [x] `P13-01` 只作为入口参考，本迭代没有扩展项目基础信息或其他设置；
- [x] 内层 Tab 统一为"基本信息 / 工作流绑定 / 其他设置"；
- [x] 四个环节术语统一为"章节规划 / 内容生成 / 审核 / 改写"；
- [x] 2×2 卡片顺序固定、等高、操作区对齐；
- [x] 长名称、窄屏和滚动状态不溢出；
- [x] 选择/更换抽屉具有固定 Header/Footer、内部滚动、焦点锁定和 Escape 关闭；
- [x] 已停用候选不可选；
- [x] 未接入、已停用和连接异常的语义明确；
- [x] 健康绑定可以更换或解除；
- [x] 无候选状态提供全局设置入口和返回项目上下文；
- [x] 409 不丢失用户选择；
- [x] 用户可见业务文案全部来自中文 locale/i18n；
- [x] 不出现 Secret、Credential、Webhook、原始 UUID、JSON 或执行按钮。

## 4. API 与数据

- [x] OpenAPI、数据库模型、实现和 UI 字段一致；
- [x] `UNIQUE(project_id, stage)` 生效；
- [x] 不存在 `ChapterPlanningParameters` 等项目参数覆盖模型；
- [x] 不存在 validate/enable/disable 项目绑定接口；
- [x] 不创建 `WorkflowRun`；
- [x] 本迭代没有第三方出站请求；
- [x] 全局工作流后续异常不会静默删除现有绑定。

## 5. 测试与工程门禁

- [x] Domain、Repository、Service 和 Handler 定向测试通过；
- [x] 真实 PostgreSQL 集成测试通过；
- [x] 四环节创建、换绑、解绑、幂等、409 和跨项目隔离测试通过；
- [x] 前端列表、抽屉、无候选、异常和冲突测试通过；
- [x] ESLint、Typecheck、Production Build 和 Contract Test 通过；
- [x] 浏览器真实 API 回归通过，未使用 Mock；
- [x] Console 无新增 Error，Network 无非预期业务 4xx/5xx；
- [x] 先局部、再分组、最后总门禁，未定位根因前不反复完整重跑；
- [x] `git diff --name-status`、未跟踪文件和 `git status --short` 已记录；
- [x] 独立 Code Review 和人工 UI 验收完成。

## 6. 冻结状态

验收标准已冻结。所有验收条目与冻结契约一致。

## 7. 验收结论与用户豁免记录（已关闭）

- **CF-13-05 人工 UI 验收**：`PASS`
  - 人工 UI 验收通过；
  - 人工验收基线 Commit：`2e8f3cd7e1a36c7d1e53d266666b2723f609ea6f`。
- **CF-13-06 四环节真实 API 联调**：`PASS WITH WAIVER`
  - 四环节真实 API 绑定、换绑、no-op、解绑通过；
  - `applicableStage`、`expectedVersion`、`Idempotency-Key`、409 冲突通过；
  - 后端完整测试通过；
  - 前端 test、typecheck、lint、build 通过；
  - Chromium Route Smoke 通过；
  - 未修改代码完成真实 API 联调。
- **最终 E2E 结果与用户豁免**：
  - 7 个测试中 5 PASS、2 FAIL、0 SKIP；
  - 两个失败均归类为历史测试债务（1. `iteration-02-project-creation.spec.ts` 使用过期空态 UI 断言；2. `iteration06` race-conditions 与 review-ui 共用可变 Fixture，存在顺序污染）；
  - 用户明确批准跳过历史 E2E 修复，不作为 Iteration 13 阻塞项；
  - 两项问题不属于 Iteration 13 工作流绑定业务缺陷。
- **已知测试债务记录**：
  - Iteration 02 空态断言与当前 UI 不一致；
  - Iteration 06 E2E Fixture 缺乏 Spec 级数据隔离；
  - 后续独立测试治理任务再处理，不带入 Iteration 14 业务范围。
- **Iteration 13 最终状态**：
  - 状态：`completed`；
  - 验收结论：`passed_with_user_waiver`；
  - `next_iteration`：`14`；
  - 不要求历史数据库版本回滚验收。