import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const editDrawerSource = readFileSync(
  new URL("./candidate-edit-drawer.tsx", import.meta.url),
  "utf8",
);

const compareDialogSource = readFileSync(
  new URL("./candidate-compare-dialog.tsx", import.meta.url),
  "utf8",
);

const revisionsDialogSource = readFileSync(
  new URL("./chapter-plan-revisions-dialog.tsx", import.meta.url),
  "utf8",
);

const apiSource = readFileSync(
  new URL("./chapter-plan-http-api.ts", import.meta.url),
  "utf8",
);

test("candidate edit drawer supports currentSnapshot editing, expectedVersion, and version conflict recovery", () => {
  assert.match(editDrawerSource, /updateChapterPlanCandidate/);
  assert.match(editDrawerSource, /expectedCandidateVersion/);
  assert.match(editDrawerSource, /version_conflict/);
  assert.match(editDrawerSource, /当前快照 \(可编辑\)/);
  assert.match(editDrawerSource, /生成初始快照 \(只读\)/);
  assert.match(editDrawerSource, /对比基线快照 \(只读\)/);
  assert.match(editDrawerSource, /刷新最新版本/);
});

test("candidate edit drawer groups editable fields and keeps fixed safe footer", () => {
  assert.match(editDrawerSource, /候选摘要/);
  assert.match(editDrawerSource, /候选章节字段/);
  assert.match(editDrawerSource, /章节概要/);
  assert.match(editDrawerSource, /候选关联信息/);
  assert.match(editDrawerSource, /生成背景/);
  assert.match(editDrawerSource, /只读/);
  assert.match(editDrawerSource, /不直接编辑当前已采用章节/);
  assert.match(editDrawerSource, /candidate-edit-drawer-footer/);
  assert.match(editDrawerSource, /保存修改/);
  assert.match(editDrawerSource, /不会覆盖当前章节内容/);
  assert.match(editDrawerSource, /ReferenceGroup/);
});

test("candidate compare dialog displays field-level diff, stale warning, and recompare action without auto-adopting", () => {
  assert.match(compareDialogSource, /compareChapterPlanCandidate/);
  assert.match(compareDialogSource, /recompareChapterPlanCandidate/);
  assert.match(compareDialogSource, /候选基线已过期/);
  assert.match(compareDialogSource, /字段级差异列表/);
  assert.match(compareDialogSource, /重新比较/);
  assert.match(compareDialogSource, /candidateDiffFieldLabel/);
  // Ensure recompare does not call adopt endpoint
  assert.doesNotMatch(compareDialogSource, /\/adopt/);
});

test("candidate compare dialog separates read-only current and candidate versions", () => {
  assert.match(compareDialogSource, /候选对比摘要/);
  assert.match(compareDialogSource, /当前章节仅供对照/);
  assert.match(compareDialogSource, /候选内容只读展示/);
  assert.match(compareDialogSource, /当前章节与候选章节只读对比/);
  assert.match(compareDialogSource, /只读对比/);
  assert.match(compareDialogSource, /<th>当前章节<\/th>/);
  assert.match(compareDialogSource, /<th>候选章节<\/th>/);
  assert.match(compareDialogSource, /采用请从候选批次详情流程发起/);
});

test("chapter plan revision dialog renders revision history and changeType labels", () => {
  assert.match(revisionsDialogSource, /listChapterPlanRevisions/);
  assert.match(revisionsDialogSource, /revisionChangeTypeLabel/);
  assert.match(revisionsDialogSource, /修订历史记录/);
  assert.match(revisionsDialogSource, /候选采用/);
});

test("FE-F3 API contracts export update, compare, recompare, and revisions functions", () => {
  assert.match(apiSource, /export function updateChapterPlanCandidate/);
  assert.match(apiSource, /export function compareChapterPlanCandidate/);
  assert.match(apiSource, /export function recompareChapterPlanCandidate/);
  assert.match(apiSource, /export function listChapterPlanRevisions/);
});
