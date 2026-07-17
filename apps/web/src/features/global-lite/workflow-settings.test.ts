import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
const api = readFileSync(new URL("./global-lite-api.ts", import.meta.url), "utf8");
const workflows = readFileSync(new URL("./workflow-page.tsx", import.meta.url), "utf8");
const settings = readFileSync(new URL("./settings-page.tsx", import.meta.url), "utf8");
test("E3 uses real built-in and run clients with page states and safe navigation", () => { for (const text of ["listBuiltinWorkflows", "listGlobalWorkflowRuns", "正在加载流程中心", "暂无内置流程", "暂无执行记录", "暂时无法加载", "failureSummary", "AbortController"]) assert.match(`${api}\n${workflows}`, new RegExp(text)); assert.doesNotMatch(workflows, /执行流程|创建流程|编辑流程|删除流程/); });
test("E4 uses real read-only clients and retains safe states", () => { for (const text of ["listCapabilities", "listIntegrations", "enabled", "not_configured", "not_available", "正在加载设置状态", "暂时无法加载", "查看内置流程", "AbortController"]) assert.match(`${api}\n${settings}`, new RegExp(text)); assert.doesNotMatch(settings, /<input|<button[^>]*>[^<]*(保存|连接|授权)/); });
