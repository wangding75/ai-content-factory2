# AUTO-E2E-001 第一/第二闭环联调验收报告

## 结论

`READY_FOR_NOVEL_CREATION = PASS`

本报告记录通过真实浏览器 UI 完成的第一轮项目准备与第二轮章节规划、正文生成、审核、重写闭环。业务写入均由 UI 操作完成；API 与 n8n 查询仅用于只读核验。

## 第一轮：项目准备

- 项目：`E2E闭环小说-20260811-第一二轮`
- projectId：`1e7e1c22-05a9-44b5-843e-649132cd98ff`
- 规划信息已在 UI 填写并保存：雾港失踪案、目标读者、卖点、风格、语气与剧情。
- 主线故事已在 UI 创建并保存：`失踪的七码头`，章节范围 1–6。
- 四个阶段均通过设置页 UI 完成绑定、验证并启用（显示“已绑定 · 可执行”）：章节规划、正文生成、审核、重写。

## 第二轮：真实闭环结果

| 阶段 | UI 创建的 ACF run | n8n execution | 结果 |
| --- | --- | --- | --- |
| 章节规划 | `9117d6e0-cea2-4fdd-9e14-51683ebb41cc` / `WR-30C40AC4` | `87` | succeeded；范围 1–2 产生 2 个候选 |
| 正文生成 | `bf9407e5-cc06-4164-a098-35af2d54f3ae` / `WR-BC15FEB0` | `88` | succeeded；产生正文候选 v2 |
| 审核 | `bf065d79-b271-4b78-b007-46d1de57e3d1` / `WR-8CE12A82` | `89` | succeeded；needs_changes，产生 1 条角色一致性问题 |
| 重写 | `65a56eaa-9d91-48ec-9618-8ca00b39e9b5` / `WR-3DD457A2` | `90` | succeeded；问题处理完成，v3 设为当前 |

### 业务落库核验

- 章节候选批次：`23f3673e-73c6-4708-a9ec-c1a94c6ad30d`，UI 确认采用 2 个候选，批次状态 `adopted`，待处理数 0。
- 已确认章节：`4843577e-d19a-41b5-8471-b1bf5064bdb3`、`28fa9278-80bd-464e-b3de-94cec9921b75`。
- 内容版本：v1 `6ca8ee1a-a82b-43dd-bc84-561d685fd2d5`；v2 `802bc904-9639-4578-af12-6f98c064c834`；v3 `a1aca7af-e9cc-4f6a-9b57-633f5f68a26c`。
- 刷新编辑器后仍显示 v3 为当前版本，来源为 `workflow_rewrite`，并保留 v2→v3 来源链路。
- 审核报告：`d721b338-3cad-480e-bf24-989d77597093`；问题：`35899dd9-1d57-4d79-b39a-40681f4c82d0`，重写后未解决问题数为 0。
- 工作流运行页刷新后四条运行均显示“已成功”，且显示对应 E2E 工作流、本地连接和无 LLM 标记。

## 本地 n8n 与验收校验

- `scripts/setup-local-n8n-workflow.ps1` 已导入、激活并核验 Iteration 15–18 四个固定 ID 工作流。
- 只读 n8n REST 审计：execution `87`、`88`、`89`、`90` 均 HTTP 200 / `success` / `webhook`，无凭据写入报告。
- Web：`verify-web.ps1 -TaskId AUTO-E2E-001` 通过（251/251 tests、typecheck、定向 lint、production build、diff check）。
- 路由：`verify-routes.ps1 -TaskId AUTO-E2E-001` 通过；项目、规划、主线、章节规划、编辑器、审核、重写、工作流运行页均无 console/page error、失败请求或横向溢出。
- 生产：`verify-production.ps1 -TaskId AUTO-E2E-001` 通过；postgres、migrate、api、web 均就绪，API/Web 健康检查通过。

## 本任务修复

正文候选“设为当前版本”原请求体多发送了后端严格契约不接受的 `contentItemId`，导致真实 UI 操作返回 400。已收敛为契约字段 `candidateVersionId`、`expectedCurrentVersionId`、`expectedCurrentVersion`；内容项 ID 仍仅用于浏览器幂等作用域。修复后通过真实 UI 完成 v2→v3 当前版本切换。

## 截图证据

- [最终编辑器（v3 当前）](./auto-e2e-001/final-editor.png)
- [工作流运行页（四条成功）](./auto-e2e-001/workflow-runs.png)
- [批次采用确认](./page-fixes/UI-049/UI-049_AFTER.png)
- [故事上下文](./page-fixes/UI-054/UI-054_AFTER.png)
- [素材上下文](./page-fixes/UI-055/UI-055_AFTER.png)
