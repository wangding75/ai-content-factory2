import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const api = readFileSync(new URL("./workflow-binding-api.ts", import.meta.url), "utf8");
const page = readFileSync(new URL("./workflow-bindings-page.tsx", import.meta.url), "utf8");

test("workflow binding drawer loads evaluated candidates instead of the generic workflow list", () => {
  assert.match(api, /\/workflow-bindings\/\$\{stage\}\/candidates/);
  assert.match(page, /listWorkflowBindingCandidates\(projectId,stage\.stage,query/);
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

test("workflow binding conflict preserves the selection and refreshes both binding and candidates", () => {
  for (const value of [
    "Promise.all([",
    "listProjectWorkflowBindings(projectId)",
    "listWorkflowBindingCandidates(projectId,stage.stage,query)",
    "setStage(latestStage)",
    "selectedStillAvailable",
    "selectedName",
    'role="alertdialog"',
    "绑定配置已被其他操作更新",
    "你刚才选择",
    "加载最新配置",
    "重新确认选择",
    "stage.binding?.version",
  ]) {
    assert.ok(page.includes(value), `missing ${value}`);
  }
  assert.doesNotMatch(page, /workflow-conflict" role="alert"/);
});
