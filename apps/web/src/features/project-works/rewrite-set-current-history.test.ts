import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const panel = readFileSync(new URL("./rewrite-result-panel.tsx", import.meta.url), "utf8");
const history = readFileSync(new URL("./rewrite-history.tsx", import.meta.url), "utf8");
const api = readFileSync(new URL("./rewrite-api.ts", import.meta.url), "utf8");

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
