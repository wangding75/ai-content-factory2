import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const panel = readFileSync(new URL("./rewrite-result-panel.tsx", import.meta.url), "utf8");
const history = readFileSync(new URL("./rewrite-history.tsx", import.meta.url), "utf8");
const api = readFileSync(new URL("./rewrite-api.ts", import.meta.url), "utf8");
const workspace = readFileSync(new URL("./rewrite-workspace.tsx", import.meta.url), "utf8");

test("Set Current uses fresh Result, Summary and ContentItem state with one idempotency key", () => {
  for (const value of ["getContentRewriteResult", "getContentRewriteSummary", "getContentItem", "expectedCurrentVersionId: item.current_version.id", "expectedCurrentVersion: item.current_version.version", "key.current ?? crypto.randomUUID", "if (!result || !item || submitting", "timeout", "network_error"]) assert.match(panel, new RegExp(value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")));
});

test("Set Current conflict reloads rather than force-overwriting and requires confirmation", () => {
  assert.match(panel, /content_version_conflict/);
  assert.match(panel, /setCasNotice\(true\); await load\(\)/);
  assert.doesNotMatch(panel, /force.?overwrite/i);
  assert.match(panel, /不会删除旧版本，也不会修改 ReviewReport 或 Issue/);
});

test("Rewrite history uses its frozen endpoint, stable pages, safe state labels and route recovery", () => {
  assert.match(api, /rewrite-history/);
  for (const value of ["PAGE_SIZE", "offset", "retryOfRunId", "candidateIsCurrent", "查看 Candidate", "查看运行"]) assert.match(history, new RegExp(value));
  assert.doesNotMatch(history, /inputPayload|outputPayload|configurationSnapshot|webhook|token/i);
});

test("workflowRunId restores the exact validated Run instead of Summary active/latest", () => {
  for (const value of ["getWorkflowRun(workflowRunId", "detail.stage !== \"rewrite\"", "detail.subjectType !== \"review_report\"", "detail.projectId !== projectId", "inputItemId !== workId", "selectedRun ?? next.activeRun ?? next.latestRun", "exactRunSummary", "result_consumed"]) {
    assert.match(workspace, new RegExp(value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")));
  }
});

test("Cancel owns an independent reusable key and never shares Runtime or consumption retry keys", () => {
  assert.match(workspace, /const cancelKey = useRef<string \| null>\(null\)/);
  assert.match(workspace, /cancelKey\.current = crypto\.randomUUID/);
  assert.match(workspace, /cancelWorkflowRun\(run\.id, run\.version, key\)/);
  assert.match(workspace, /api\.code !== "timeout" && api\.code !== "network_error"/);
  assert.doesNotMatch(workspace, /cancelWorkflowRun\(run\.id, run\.version, crypto\.randomUUID\(\)\)/);
});
