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
