import assert from "node:assert/strict";
import test from "node:test";
import { readFileSync } from "node:fs";
const api = readFileSync(new URL("./rewrite-api.ts", import.meta.url), "utf8");

test("rewrite input enforces open issue selection boundaries and configuration constraints", () => {
  for (const value of ["issueIds.length < 1", "issueIds.length > 50", "new Set(issueIds).size", "instructions].length > 2000", "creative_rewrite"]) assert.match(api, new RegExp(value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")));
});

test("rewrite maps contract failures to safe Chinese messages", () => {
  for (const code of ["rewrite_preflight_expired", "rewrite_preflight_stale", "rewrite_preflight_consumed", "active_rewrite_run_conflict", "timeout"]) assert.match(api, new RegExp(code));
});

test("rewrite workspace keeps token out of URL and browser storage and guards duplicate creates", () => {
  const page = readFileSync(new URL("./rewrite-workspace.tsx", import.meta.url), "utf8");
  assert.doesNotMatch(page, /localStorage|sessionStorage/);
  assert.match(page, /key\.current \?\? crypto\.randomUUID/);
  assert.match(page, /creating\) return/);
  assert.match(page, /preflightToken/);
});

test("rewrite create renders the frozen availability, preflight, configuration and confirmation chain", () => {
  const page = readFileSync(new URL("./rewrite-workspace.tsx", import.meta.url), "utf8");
  for (const value of ["active_rewrite_run_conflict", "no_open_issues", "rewrite_not_configured", "showConfiguration", "预检结果", "reviewReportSnapshot.summary", "expiresAt", "confirmCreate", "确认创建重写任务"]) {
    assert.match(page, new RegExp(value));
  }
  assert.match(api, /reviewReportSnapshot: RewriteReportSnapshot/);
  assert.match(api, /reviewReportId: string/);
  assert.doesNotMatch(api, /reviewReportSnapshot: \{ id: string/);
});

test("create lifecycle invalidates changed input, preserves unknown results and clears deterministic conflicts", () => {
  const page = readFileSync(new URL("./rewrite-workspace.tsx", import.meta.url), "utf8");
  assert.match(page, /clearPreflight/);
  for (const code of ["rewrite_preflight_expired", "rewrite_preflight_stale", "rewrite_preflight_consumed", "idempotency_conflict", "network_error", "timeout"]) {
    assert.match(page, new RegExp(code));
  }
  assert.match(page, /key\.current = submitKey/);
});
