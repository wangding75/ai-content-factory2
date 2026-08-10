import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import test from "node:test";

const root = join(process.cwd(), "src", "features", "content-items");
const editor = readFileSync(join(root, "content-editor-workspace.tsx"), "utf8");
const drawer = readFileSync(join(root, "content-generation-drawer.tsx"), "utf8");
const status = readFileSync(join(root, "content-generation-status.tsx"), "utf8");
const candidate = readFileSync(join(root, "content-candidate-compare.tsx"), "utf8");
const locale = readFileSync(join(root, "content-generation-locale.ts"), "utf8");

test("content generation remains on the editor route with its three context tabs", () => {
  assert.match(editor, /getContentGenerationSummary/);
  assert.match(editor, /ContentGenerationDrawer/);
  assert.match(editor, /role="tablist"/);
  assert.doesNotMatch(editor, /content-generation-runs\/.*route/);
});

test("chapter goal context is an independent read-only area backed by the chapter plan", () => {
  assert.match(editor, /useState<"goal" \| "story" \| "materials">\("goal"\)/);
  assert.match(editor, /listChapterPlans\(/);
  assert.match(editor, /className="content-goal-panel"/);
  assert.match(editor, /来自当前章节规划的只读创作上下文/);
  assert.match(editor, /plan\?\.chapter_goal/);
  assert.match(editor, /plan\?\.creation_notes/);
  assert.match(editor, /plan\?\.summary/);
  assert.match(editor, /className="content-goal-readonly"/);
  assert.match(editor, /contextTab === "story"/);
  assert.match(editor, /contextTab === "materials"/);
});

test("summary polling is isolated from the editable draft refresh", () => {
  assert.match(editor, /\["queued", "running"\]/);
  assert.match(editor, /refreshGenerationSummary/);
  assert.doesNotMatch(editor, /const refreshGenerationSummary[\s\S]*?getContentItem[\s\S]*?const refreshWorkspace/);
  assert.match(editor, /const refreshWorkspace[\s\S]*?getContentItem/);
});

test("failure recovery and candidate presentation retain their frozen API boundaries", () => {
  assert.match(status, /retryContentGenerationResultConsumption/);
  assert.match(status, /retryWorkflowRun/);
  assert.match(candidate, /setCurrentContentVersion/);
  assert.match(candidate, /candidateCanBecomeCurrent/);
  assert.match(candidate, /candidate_source_stale/);
  assert.doesNotMatch(candidate, /force|override/);
});

test("drawer, status, and candidate visible business copy use the feature locale", () => {
  for (const source of [drawer, status, candidate]) assert.match(source, /content-generation-locale/);
  for (const key of ["title", "contextOptionLabels", "preflightPassed", "conditionsChanged", "confirmCreate"]) assert.match(drawer, new RegExp(`copy\\.drawer\\.${key}`));
  assert.match(locale, /drawer:/);
});
