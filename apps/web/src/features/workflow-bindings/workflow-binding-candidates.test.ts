import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const api = readFileSync(new URL("./workflow-binding-api.ts", import.meta.url), "utf8");
const page = readFileSync(new URL("./workflow-bindings-page.tsx", import.meta.url), "utf8");

test("workflow binding drawer loads evaluated candidates instead of the generic workflow list", () => {
  assert.match(api, /\/workflow-bindings\/\$\{stage\}\/candidates/);
  assert.match(page, /listWorkflowBindingCandidates\(projectId,drawer\.stage\.stage,query/);
  assert.doesNotMatch(page, /listApplicableWorkflows\(/);
});

test("workflow binding drawer renders actual eligibility facts and blocks unselectable candidates", () => {
  for (const value of [
    "版本：v{w.version}",
    "连接：{candidate.connectionSummary.name}",
    "LLM：{llmPolicy}",
    "不可选原因：{formatIneligibilityReason(primaryReason)}",
    "前往修复",
    "const disabled = !candidate.selectable",
    "disabled={disabled}",
  ]) {
    assert.ok(page.includes(value), `missing ${value}`);
  }
  assert.doesNotMatch(page, /onClick=\{\(e\) => \{ if \(disabled\) e\.preventDefault\(\); \}\}/);
});
