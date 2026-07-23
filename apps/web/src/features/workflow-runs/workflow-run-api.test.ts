import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

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
