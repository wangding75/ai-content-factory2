import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const api = readFileSync(new URL("./global-lite-api.ts", import.meta.url), "utf8");
const workflows = readFileSync(new URL("./workflow-page.tsx", import.meta.url), "utf8");
const settings = readFileSync(new URL("./settings-page.tsx", import.meta.url), "utf8");

test("E3 maps real data to Chinese workflow, status, step and time view models", () => {
  for (const text of ["listBuiltinWorkflows", "listGlobalWorkflowRuns", "workflowPresentation", "runStatusLabel", "formatTime", "正在加载流程中心", "暂无内置流程", "暂无执行记录", "暂时无法加载", "failureSummary", "AbortController", "workflow-stats", "workflow-steps"]) assert.match(`${api}\n${workflows}`, new RegExp(text));
  assert.doesNotMatch(workflows, /item\.workflowKey\b|item\.provider\b|item\.startedAt\b|item\.finishedAt\b|item\.subject\b|trigger_type|workflow_type|JSON|Prompt/);
});

test("E4 has frozen tabs and only renders localized read-only view models", () => {
  for (const text of ["listCapabilities", "listIntegrations", "capabilityPresentation", "integrationPresentation", "已启用", "已停用", "正在加载设置状态", "暂时无法加载", "settings-tabs", "AbortController"]) assert.match(`${api}\n${settings}`, new RegExp(text));
  assert.doesNotMatch(settings, /item\.key|item\.enabled|item\.available|item\.provider|<input|<button[^>]*>(保存|连接|授权)/);
});

test("global navigation highlights workflows and settings without placeholder actions", () => {
  const shell = readFileSync(new URL("../../components/ui/app-shell.tsx", import.meta.url), "utf8");
  assert.match(shell, /active===i\.key/);
  assert.match(shell, /active==="settings"/);
  assert.doesNotMatch(`${workflows}\n${settings}\n${shell}`, /href="#"|onClick=\{\(\)\s*=>\s*\{\s*\}\}/);
});
