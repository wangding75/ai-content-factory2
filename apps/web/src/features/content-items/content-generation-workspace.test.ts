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

test("story context resolves the current plan references without changing the editor", () => {
  assert.match(editor, /getStorylines\(/);
  assert.match(editor, /getForeshadowings\(/);
  assert.match(editor, /Promise\.all\(\[[\s\S]*getStorylines\(projectId, c\.signal\)[\s\S]*getForeshadowings\(projectId, c\.signal\)/);
  assert.match(editor, /className="content-story-panel"/);
  assert.match(editor, /关联故事线/);
  assert.match(editor, /关联伏笔/);
  assert.match(editor, /storylineName\(storyline\.name\)/);
  assert.match(editor, /foreshadowing\?\.description/);
});

test("materials context shows real project materials and an explicit empty state", () => {
  assert.match(editor, /listProjectMaterialsFromApi\(/);
  assert.match(editor, /material_refs_json/);
  assert.match(editor, /className="content-material-panel"/);
  assert.match(editor, /可用素材/);
  assert.match(editor, /本章引用/);
  assert.match(editor, /已引用/);
  assert.match(editor, /可引用/);
  assert.match(editor, /暂无可用素材/);
  assert.doesNotMatch(editor, /新增素材|编辑素材|删除素材/);
});

test("generation confirmation keeps the frozen preflight and create boundary while exposing run context", () => {
  const drawerSource = readFileSync(join(root, "content-generation-drawer.tsx"), "utf8");
  assert.match(drawerSource, /chapterLabel/);
  assert.match(drawerSource, /工作流概览/);
  assert.match(drawerSource, /目标章节/);
  assert.match(drawerSource, /候选版本/);
  assert.match(drawerSource, /contextSummary\.materialCount/);
  assert.match(drawerSource, /contextSummary\.storylineCount/);
  assert.match(drawerSource, /content-generation-confirm/);
  assert.match(drawerSource, /preflightContentGenerationRun/);
  assert.match(drawerSource, /createContentGenerationRun/);
  assert.match(drawerSource, /preflightToken/);
});

test("generation requirements stay visible, editable, and distinguish empty from filled", () => {
  const drawerSource = readFileSync(join(root, "content-generation-drawer.tsx"), "utf8");
  assert.match(drawerSource, /content-generation-requirements-state/);
  assert.match(drawerSource, /已填写生成要求/);
  assert.match(drawerSource, /尚未填写生成要求/);
  assert.match(drawerSource, /instructions\.trim\(\)/);
  assert.match(drawerSource, /value=\{instructions\}/);
  assert.match(drawerSource, /setInstructions/);
});

test("queued generation exposes run details and safe cancellation without candidate or progress actions", () => {
  assert.match(status, /summary\.state === "queued"/);
  assert.match(status, /href=\{`\/workflow-runs\/\$\{run\.id\}`\}/);
  assert.match(status, /content-generation-cancel/);
  assert.match(status, /cancelWorkflowRun/);
  assert.match(status, /content-generation-cancel:\$\{run\.id\}/);
  assert.doesNotMatch(status, /summary\.state === "queued"[^\n]*onCandidate/);
});

test("running generation keeps the editor-level status compact and shows real run timing and event context", () => {
  assert.match(status, /summary\.state === "running"/);
  assert.match(status, /run\.startedAt \?\? run\.createdAt/);
  assert.match(status, /latestEvent \? eventLabel\(latestEvent\.eventType\)/);
  assert.match(status, /content-generation-running-meta/);
  assert.match(status, /visibleEvents\.length/);
  assert.doesNotMatch(status, /content-editor-state/);
});

test("candidate-ready status presents only a real candidate without changing the current version", () => {
  assert.match(status, /summary\.state === "candidate_ready"/);
  assert.match(status, /summary\.latestCandidateVersion/);
  assert.match(status, /copy\.candidateReadyDetail\(candidate\.version_no\)/);
  assert.match(status, /copy\.candidateVersion\(candidate\.version_no\)/);
  assert.match(status, /candidate && <button onClick=\{onCandidate\}/);
  assert.match(locale, /candidateReadyDetail/);
  assert.match(locale, /candidateReadyMissing/);
  assert.doesNotMatch(status, /run\.status === "succeeded"/);
  assert.doesNotMatch(status, /setCurrentContentVersion/);
});

test("generation failures stay distinct, safe, and recoverable without a candidate success action", () => {
  assert.match(status, /copy\.titles\.runtime_failed/);
  assert.match(status, /copy\.titles\.output_validation_failed/);
  assert.match(status, /copy\.titles\.result_consumption_failed/);
  assert.match(status, /copy\.runtimeFailedDetail/);
  assert.match(status, /copy\.outputValidationFailedDetail/);
  assert.match(status, /copy\.resultConsumptionFailedDetail/);
  assert.match(status, /summary\.state === "result_consumption_failed"/);
  assert.match(status, /copy\.retryConsumption/);
  assert.match(status, /href=\{`\/workflow-runs\/\$\{run\.id\}`\}/);
  assert.match(status, /summary\.state === "candidate_ready" && candidate && <button onClick=\{onCandidate\}/);
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
