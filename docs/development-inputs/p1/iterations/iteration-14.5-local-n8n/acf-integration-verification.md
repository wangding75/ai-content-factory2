# ACF 本地 n8n 集成验证

## 验证范围

Iteration 14.5-03C 使用 ACF 正式 API 创建或复用本地 n8n Connection 与 Workflow Configuration，完成真实 Verify、项目 `chapter_planning` 绑定和 Chapter Planning Preflight。

本验证没有调用 Create Run，没有创建 WorkflowRun、Candidate Batch 或 Candidate，也没有直接写数据库。n8n execution 取证仅通过只读方式读取 n8n 自身的 SQLite execution 元数据。

## 固定目标

- 测试项目：`13a13e7e-656e-4174-bc96-c301692ebced`
- Connection 名称：`Local N8N`
- Connection ID：`8aa40c62-e2cc-42cf-a141-0b2012d5b0eb`
- Connection 类型：`n8n`
- Connection 内部地址：`http://n8n:5678`
- Workflow Configuration 名称：`Local N8N Iteration 15 Chapter Planning`
- Workflow Configuration ID：`c3c90c91-e7ec-4e39-b948-8923bae21d17`
- n8n Workflow ID：`f4e32de0-15a0-4100-8000-000000000001`
- applicable stage：`chapter_planning`
- Production Webhook：`http://n8n:5678/webhook/acf-iteration-15-local-chapter-planning`
- input/output contract version：`v1`

同名 Connection 和 Workflow Configuration 各只有一条，重复执行没有创建重复资源。

## 使用的正式 API

脚本只使用以下 ACF 正式 API 完成业务数据操作：

| 用途 | 方法与路径 |
| --- | --- |
| 查询/创建 Connection | `GET/POST /api/v1/workflow-connections` |
| 查询 Connection 详情 | `GET /api/v1/workflow-connections/{connectionId}` |
| Verify Connection | `POST /api/v1/workflow-connections/{connectionId}/verify` |
| 查询/创建 Workflow Configuration | `GET/POST /api/v1/workflow-configurations` |
| 查询 Workflow Configuration 详情 | `GET /api/v1/workflow-configurations/{workflowId}` |
| Verify Workflow Configuration | `POST /api/v1/workflow-configurations/{workflowId}/verify` |
| 查询项目绑定 | `GET /api/v1/projects/{projectId}/workflow-bindings` |
| 创建、替换或确认项目绑定 | `PUT /api/v1/projects/{projectId}/workflow-bindings/chapter_planning` |
| Chapter Planning Preflight | `POST /api/v1/projects/{projectId}/chapter-plan-runs/preflight` |

创建、Verify 和绑定写操作均使用操作级、稳定且可追踪的 `Idempotency-Key`。Verify 请求使用当前资源 `expectedVersion`；需要重新 Verify 时，幂等键由资源 ID 和当前版本派生。资源已经 `connected/enabled` 时，重复脚本只通过详情 API 确认状态，不再次增加版本。

所有成功响应均验证为单层 `{data, request_id}` Envelope。脚本在错误时只报告 HTTP 状态、错误代码和安全消息摘要。

## Verify 状态转换

### Connection

- 创建后：`integrationStatus=not_connected`、`enabled=false`、`version=1`
- 首次 Verify：HTTP 200，转为 `integrationStatus=connected`、`enabled=true`、`version=2`
- 相同 `Idempotency-Key` 重放：HTTP 200，响应 data 与首次相同，版本保持 `2`
- 重放期间 n8n workflow execution 增量为 `0`
- 重复脚本：复用 ID，查询确认 `connected/enabled`，版本保持 `2`

Connection Verify 真实访问 `http://n8n:5678/healthz`。同键重放由正式服务的 verification replay 路径在外部 probe 前返回；实际重放的资源版本和响应 data 均保持不变。

### Workflow Configuration

- 创建后：`integrationStatus=not_connected`、`enabled=false`、`version=1`
- 首次 Verify：HTTP 200，转为 `integrationStatus=connected`、`enabled=true`、`version=2`
- 相同 `Idempotency-Key` 重放：HTTP 200，响应 data 与首次相同，版本保持 `2`
- 重放前后 n8n execution 数量不变，证明没有第二次 webhook probe
- 重复脚本：复用 ID，查询确认 `connected/enabled`，版本保持 `2`

## n8n 探针结果

- ACF Workflow Verify 对应的 n8n execution ID：`20`
- execution mode：`webhook`
- execution status：`success`
- `probeType=acf_workflow_verification`
- `verified=true`
- `stage=chapter_planning`
- `contractVersion=v1`
- `requestId` 原样返回

脚本只从 execution `20` 提取上述非敏感请求和响应字段，不输出请求头、Cookie、凭据或其他 execution payload。

## 项目绑定

- 原 `chapter_planning` 绑定：未绑定
- 最终绑定 ID：`fad8d433-35a6-4c11-874c-dc6142ae27d6`
- 最终 Workflow Configuration ID：`c3c90c91-e7ec-4e39-b948-8923bae21d17`
- 最终绑定版本：`1`
- applicable stage：`chapter_planning`
- Connection 与 Workflow Configuration：均为 `connected/enabled`

重复执行时原绑定与最终绑定均为上述同一记录；PUT 返回 no-change 结果，绑定 ID 和版本没有变化。本阶段保留该绑定供 14.5-04 使用。

## Preflight passed

合法输入使用：

- `generationMode=range`
- 目标章节范围：第 1 章至第 1 章
- `storylineSelection.mode=auto_balanced`
- 四个正式 `contextOptions` 字段齐全
- `additionalInstructions=null`

验证结果：

- HTTP 200
- `result=passed`
- `status=passed`
- `blockers=[]`
- `inputSummary` 非空
- `executionConfigurationSummary` 非空
- summary 指向绑定 `fad8d433-35a6-4c11-874c-dc6142ae27d6`；该绑定指向本地 Workflow Configuration
- preflightToken 存在，但未记录其值
- inputDigest：`1e27b380a4136fbf49084dc28f6376f0130c35ea6b3c77f7735e578b76eeda72`
- 相同输入在单次执行内重复 Preflight，digest 一致
- 完整脚本重复执行后 digest 仍一致

验证前后该项目的 chapter-planning WorkflowRun 总数均为 `0`，Candidate Batch 总数均为 `0`。没有使用 preflightToken 创建 Run。

## API 与 n8n 日志

- API 容器 healthy，`/healthz` 与 `/readyz` 均为 HTTP 200；最新 API 日志只有正常启动信息。
- n8n 容器 healthy，固定工作流已激活；Verify execution `20` 及重复脚本产生的 Production Webhook health execution 均在 n8n execution 元数据中记录为 `success`。
- n8n 1.104.2 的 error reporter 在成功的 `respondToWebhook` execution 完成时仍输出 `node execution output incorrect data` 诊断。该诊断没有改变 HTTP 200 响应、execution `success` 状态或探针字段；取证结果以 execution `20` 的已持久化请求、响应和状态为准。
- n8n 另有禁用遥测后的 PostHog feature-flag 401 与 task-runner deprecation 提示，不影响本地服务健康或工作流执行。

## 执行方法与幂等结果

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\setup-local-n8n-workflow.ps1
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\verify-local-n8n-integration.ps1
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\verify-local-n8n-integration.ps1
```

首次完整执行结果为 PASS。验证脚本最初一次执行在 Connection 已通过正式 API 创建后，因 Windows Docker 参数转义导致只读 n8n execution 审计命令失败；该问题在进入 Verify 前修复，已创建 Connection 被正常复用，没有回滚或直接修改业务数据。

重复完整执行结果为 PASS：

- Connection ID 不变
- Workflow Configuration ID 不变
- 同名资源各一条
- 两者仍为 `connected/enabled`
- 两者版本均保持 `2`
- 项目绑定 ID、目标和版本不变
- Preflight 仍为 `passed`
- inputDigest 不变
- WorkflowRun 和 Candidate Batch 数量不变

## 14.5-04 起点

14.5-04 可从当前保留的项目绑定开始：

1. 先执行本验证脚本确认 API、n8n、Connection、Workflow Configuration 和绑定仍可用。
2. 使用相同合法输入重新执行 Preflight，获取新的短时效 preflightToken。
3. 仅在 14.5-04 的明确授权下调用 Create Run。

本文件不保存 preflightToken、Cookie、密码、API Token、数据库凭据或其他敏感信息。
