import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const listPageSource = readFileSync(
  new URL("./candidate-batch-list-page.tsx", import.meta.url),
  "utf8",
);

const detailPageSource = readFileSync(
  new URL("./candidate-batch-detail-page.tsx", import.meta.url),
  "utf8",
);

const apiSource = readFileSync(
  new URL("./chapter-plan-http-api.ts", import.meta.url),
  "utf8",
);

const presentationSource = readFileSync(
  new URL("./chapter-plan-presentation.ts", import.meta.url),
  "utf8",
);

test("candidate batch presentation helpers format labels", () => {
  assert.match(presentationSource, /export function candidateBatchStatusLabel/);
  assert.match(presentationSource, /export function candidateBatchModeLabel/);
  assert.match(presentationSource, /export function candidateStatusLabel/);
  assert.match(presentationSource, /export function candidateDiffTypeLabel/);
  assert.match(presentationSource, /全量采用/);
  assert.match(presentationSource, /基线冲突/);
});

test("candidate batch and candidate query helpers are exported in chapter-plan-http-api", () => {
  assert.match(apiSource, /export function candidateBatchQuery/);
  assert.match(apiSource, /export function candidateQuery/);
  assert.match(apiSource, /export function listChapterPlanCandidateBatches/);
  assert.match(apiSource, /export function getChapterPlanCandidateBatch/);
  assert.match(apiSource, /export function listChapterPlanCandidates/);
  assert.match(apiSource, /export function getChapterPlanCandidate/);
});

test("candidate batch list page renders 7 filters and pagination", () => {
  assert.match(listPageSource, /批次状态筛选/);
  assert.match(listPageSource, /生成模式筛选/);
  assert.match(listPageSource, /来源任务筛选/);
  assert.match(listPageSource, /创建起始时间/);
  assert.match(listPageSource, /创建截止时间/);
  assert.match(listPageSource, /listChapterPlanCandidateBatches/);
  assert.match(listPageSource, /上一页/);
  assert.match(listPageSource, /下一页/);
});

test("candidate batch list shows status, source run, scope, time, and independent states", () => {
  assert.match(listPageSource, /批次 \/ 来源/);
  assert.match(listPageSource, /Run ID：\{batch\.sourceWorkflowRunId\}/);
  assert.match(listPageSource, /candidateBatchStatusLabel\(batch\.status\)/);
  assert.match(listPageSource, /创建时间/);
  assert.match(listPageSource, /查看详情/);
  assert.match(listPageSource, /hasFilters/);
  assert.match(listPageSource, /候选批次加载失败/);
  assert.match(listPageSource, /暂无候选批次/);
});

test("candidate batch detail page renders candidate list, search, and diff tags", () => {
  assert.match(detailPageSource, /getChapterPlanCandidateBatch/);
  assert.match(detailPageSource, /listChapterPlanCandidates/);
  assert.match(detailPageSource, /全部候选状态/);
  assert.match(detailPageSource, /全部差异类型/);
  assert.match(detailPageSource, /搜索候选标题、摘要或目的/);
  assert.match(detailPageSource, /candidateDiffTypeLabel/);
});
