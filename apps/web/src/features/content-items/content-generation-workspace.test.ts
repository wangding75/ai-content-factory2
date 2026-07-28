import assert from "node:assert/strict";
import test from "node:test";
import { readFileSync } from "node:fs";
import { join } from "node:path";
const root = join(process.cwd(), "src", "features", "content-items");
const editor = readFileSync(join(root, "content-editor-workspace.tsx"), "utf8");
const drawer = readFileSync(join(root, "content-generation-drawer.tsx"), "utf8");
const status = readFileSync(join(root, "content-generation-status.tsx"), "utf8");
const candidate = readFileSync(join(root, "content-candidate-compare.tsx"), "utf8");
test("编辑器加载 Summary，并以同一路由 Tab 和抽屉组织正文生成", () => {
  assert.match(editor, /getContentGenerationSummary/);
  assert.match(editor, /章节目标/); assert.match(editor, /故事情报/); assert.match(editor, /素材库/);
  assert.match(editor, /ContentGenerationDrawer/); assert.doesNotMatch(editor, /content-generation-runs\/.*route/);
});
test("预检阻断不会创建任务，确认提交使用 Token 且防止重复提交", () => {
  assert.match(drawer, /report\.status !== "passed"/); assert.match(drawer, /preflightToken/);
  assert.match(drawer, /submitted\.current/); assert.match(drawer, /getOrCreateOperation/);
  assert.match(drawer, /createContentGenerationRun/); assert.match(drawer, /await onCreated/);
  assert.match(drawer, /重新预检/); assert.match(drawer, /当前条件已变化，请重新预检/);
});
test("排队和运行状态只按持久化 Summary 轮询，并展示安全 Run/Event 详情", () => {
  assert.match(editor, /\["queued", "running"\]/);
  assert.match(editor, /window\.setInterval/);
  assert.match(status, /getWorkflowRunEvents/);
  assert.match(status, /正文生成正在执行，进度以运行事件为准/);
  assert.doesNotMatch(status, /\d+%/);
});
test("三类失败严格映射到冻结恢复动作", () => {
  assert.match(status, /summary\.state === "runtime_failed"/);
  assert.match(status, /summary\.state === "output_validation_failed"/);
  assert.match(status, /summary\.state === "result_consumption_failed"/);
  assert.match(status, /retryContentGenerationResultConsumption/);
  assert.match(status, /retryWorkflowRun/);
});
test("候选比较与 CAS 切换不会覆盖当前版本，且 stale 禁止强制覆盖", () => {
  assert.match(candidate, /getContentVersion\(summary\.currentVersionId/);
  assert.match(candidate, /candidateCanBecomeCurrent/);
  assert.match(candidate, /candidateVersionId: candidate\.id/);
  assert.match(candidate, /expectedCurrentVersionId: summary\.currentVersionId/);
  assert.match(candidate, /expectedCurrentVersion: summary\.currentVersion\.version/);
  assert.match(candidate, /candidate_source_stale/);
  assert.doesNotMatch(candidate, /force|override/);
});
test("未配置状态跳转既有项目工作流绑定入口", () => {
  assert.match(status, /settings\?tab=workflow-bindings/);
  assert.match(status, /尚未配置正文生成工作流/);
});
