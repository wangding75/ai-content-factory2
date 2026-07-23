import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { ApiError } from "../../lib/api.ts";
import { cancelWorkflowRun, formatWorkflowRunTime, getWorkflowRun, listWorkflowRunEvents, retryWorkflowRun } from "./workflow-run-api.ts";

const source = readFileSync(new URL("./workflow-run-api.ts", import.meta.url), "utf8");
const page = readFileSync(new URL("./workflow-runs-page.tsx", import.meta.url), "utf8");

test("workflow run API keeps all frozen server-side filters and maps the Runtime list response", () => {
  for (const text of ["projectId", "stage", "status", "q", "startTime", "endTime", "limit", "offset", "response.items.map(mapWorkflowRun)"]) assert.ok(source.includes(text));
  assert.match(source, /apiRequest<WorkflowRunList>\(`\/workflow-runs\?\$\{workflowRunQuery\(query\)\}`/);
  assert.doesNotMatch(source, /content-workflow-runs/);
});

test("WorkflowRun presentation protects the list from invalid enum values and timestamps", () => {
  for (const text of ["未知环节", "未知状态", "未知来源", "return \"—\"", "Number.isNaN"]) assert.match(source, new RegExp(text));
});

test("workflow list page implements real loading, empty, error-retry, URL filters, and server filtering interactions", () => {
  for (const text of ["initialProjectId", "initialStage", "new AbortController", "listWorkflowRuns(query", "onClick={() => void load()}", "暂无符合筛选条件的运行记录", "搜索运行编号", "时间范围"]) assert.match(page, new RegExp(text.replace(/[(){}?]/g, "\\$&")));
  assert.doesNotMatch(page, /mockScenario|content-workflow-runs|href="#"/);
});

const originalFetch = global.fetch;
test.after(() => { global.fetch = originalFetch; });
const run = { id: "run/id", runNumber: "WR-1", projectId: "project", stage: "review", workflowConfigurationId: "config", triggerSource: "manual", status: "queued", inputPayload: {}, outputPayload: null, errorCode: null, errorMessage: null, errorDetails: null, configurationSnapshot: {}, startedAt: null, finishedAt: null, cancelledAt: null, createdAt: "invalid", updatedAt: "invalid", version: 3 };
test("Runtime detail API functions send the frozen paths, versions, keys, and complete retry replacement", async () => {
  const calls: Array<{ url: string; init?: RequestInit }> = [];
  global.fetch = async (url, init) => { calls.push({ url: String(url), init }); const body = String(url).endsWith("/events") ? { items: [{ id: "event", runId: "run/id", eventType: "queued", status: "queued", payload: {}, createdAt: "invalid" }] } : run; return new Response(JSON.stringify({ data: body, request_id: "req" }), { status: String(url).includes("retries") ? 201 : 200 }); };
  const detail = await getWorkflowRun("run/id"); const events = await listWorkflowRunEvents("run/id"); await cancelWorkflowRun("run/id", 3, "cancel-key"); await retryWorkflowRun("run/id", 3, "retry-key", { replacement: true });
  assert.equal(detail.createdAtLabel, "—"); assert.equal(events[0].title, "已创建运行"); assert.match(calls[0].url, /run%2Fid/); assert.equal((calls[2].init?.headers as Record<string,string>)["Idempotency-Key"], "cancel-key"); assert.deepEqual(JSON.parse(String(calls[3].init?.body)), { expectedVersion: 3, useCurrentConfiguration: false, inputOverride: { replacement: true } }); assert.equal(formatWorkflowRunTime("bad"), "—");
});
test("Runtime detail API functions preserve ErrorEnvelope failures", async () => { global.fetch = async () => new Response(JSON.stringify({ error: { code: "version_conflict", message: "changed", details: {} }, request_id: "req" }), { status: 409 }); await assert.rejects(getWorkflowRun("run"), (error: unknown) => error instanceof ApiError && error.code === "version_conflict"); });
