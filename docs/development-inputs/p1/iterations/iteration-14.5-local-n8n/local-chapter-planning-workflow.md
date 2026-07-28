# 本地确定性章节规划工作流

## 目标与边界

工作流 `ACF Iteration 15 Local Chapter Planning` 为本地 n8n 提供一个已激活的 Production Webhook，用于验证确定性的章节规划输入和输出传输。

它不调用 LLM、不使用外部凭据、不访问 ACF 数据库，也不会创建 ACF Integration、项目绑定、Preflight 或 Create Run。本阶段只直接调用 n8n Webhook，不进行 Iteration 15 联调。

## 地址与节点

- 固定 Path：`acf-iteration-15-local-chapter-planning`
- 宿主机地址：<http://127.0.0.1:15678/webhook/acf-iteration-15-local-chapter-planning>
- Docker 内部地址：`http://n8n:5678/webhook/acf-iteration-15-local-chapter-planning`

节点顺序为：Production Webhook → 请求结构校验 → 确定性章节候选生成 → 返回 Runtime 输出或契约错误。

## 输入与输出

请求使用 ExecutionRequest 已有的运行上下文加 Runtime `inputPayload`：

```json
{
  "runId": "<WorkflowRun UUID>",
  "projectId": "<project UUID>",
  "stage": "chapter_planning",
  "input": {
    "generationContextDigest": "<64-character digest>",
    "target": { "startChapterNo": 1, "endChapterNo": 1, "requestedChapterCount": 1 },
    "stage": "chapter_planning",
    "generationContext": {
      "inputDigest": "<64-character digest>",
      "inputSnapshot": { "generationMode": "range" },
      "storylineSnapshot": { "available": [{ "id": "<storyline UUID>" }] },
      "baseChapterPlans": []
    }
  }
}
```

成功时响应严格采用 `NormalizedChapterPlanOutput`：`projectId`、`generationMode`、`target`、`sourceWorkflowRunId`、`candidates` 和 `metadata`。`sourceWorkflowRunId` 复制 `runId`，`metadata.inputDigest` 复制冻结上下文的 `inputDigest`。没有由 n8n 生成 `diffType`：候选的 `new`、`replace` 或 `no_change` 由 ACF 后端基于 Base Plan 计算。

缺失或无效字段返回 HTTP 400 与明确的 `{ "error": { "code", "message", "field" } }` 契约错误。相同输入总是返回相同输出；`generatedAt` 为固定值，因此不会引入时间不确定性。

## 导入、激活与幂等更新

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\setup-local-n8n-workflow.ps1
```

脚本使用 n8n 1.104.2 已验证的官方 CLI：`import:workflow`、`update:workflow --active=true` 和 `export:workflow`。工作流 JSON 有固定 ID；重复执行导入同一 ID、重新激活，并断言同名工作流最终仅有一份。

## 直接烟测与 Execution 验证

对宿主机 Production Webhook 发送符合上述结构的 POST 请求。检查 HTTP 200，响应字段与 Runtime Consumer Schema 一致，并用相同请求再次调用确认响应一致。

在 n8n UI 的 Executions 中确认已产生成功的 Production Execution。本任务的直接烟测不调用 ACF；未来 Integration Verify 可用上述 Docker 内部地址探测 webhook。
