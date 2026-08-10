import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const actionDialogsSource = readFileSync(
  new URL("./candidate-action-dialogs.tsx", import.meta.url),
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

test("batch adopt dialog handles itemized outcomes and partial success without compressing results", () => {
  assert.match(actionDialogsSource, /adoptChapterPlanCandidates/);
  assert.match(actionDialogsSource, /ItemizedOutcomeRow/);
  assert.match(actionDialogsSource, /已采用/);
  assert.match(actionDialogsSource, /无变化/);
  assert.match(actionDialogsSource, /基线过期/);
  assert.match(actionDialogsSource, /版本冲突/);
  assert.doesNotMatch(actionDialogsSource, /候选 ID:/);
});

test("batch adopt confirmation explains selected count and impact before submit", () => {
  assert.match(actionDialogsSource, /确认批量采用候选/);
  assert.match(actionDialogsSource, /批量采用摘要/);
  assert.match(actionDialogsSource, /本次采用影响/);
  assert.match(actionDialogsSource, /新设章节/);
  assert.match(actionDialogsSource, /替换候选/);
  assert.match(actionDialogsSource, /无变化/);
  assert.match(actionDialogsSource, /存在冲突/);
  assert.match(actionDialogsSource, /采用说明/);
  assert.match(actionDialogsSource, /不会直接确认章节，也不会触发正文生产/);
  assert.match(actionDialogsSource, /确认批量采用/);
});

test("batch abandon dialog requires acknowledgeAdoptedChaptersRemain: true and shows non-rollback notice", () => {
  assert.match(actionDialogsSource, /abandonChapterPlanCandidateBatch/);
  assert.match(actionDialogsSource, /acknowledgeAdoptedChaptersRemain: true/);
  assert.match(actionDialogsSource, /已采用的 .*个章节及其修订记录将完整保留/);
});

test("batch abandon confirmation separates irreversible impact from safe cancellation", () => {
  assert.match(actionDialogsSource, /放弃候选批次/);
  assert.match(actionDialogsSource, /候选批次摘要/);
  assert.match(actionDialogsSource, /放弃影响说明/);
  assert.match(actionDialogsSource, /当前已采用章节内容不会被本操作修改/);
  assert.match(actionDialogsSource, /已采用章节不会撤销/);
  assert.match(actionDialogsSource, /chapter-plan-button danger/);
  assert.match(actionDialogsSource, /确认放弃候选批次/);
  assert.match(actionDialogsSource, /请先勾选确认条款/);
});

test("stale conflict dialog offers recompare and refresh options without force adopt", () => {
  assert.match(actionDialogsSource, /候选基线已过期/);
  assert.match(actionDialogsSource, /重新比较/);
  assert.match(actionDialogsSource, /刷新最新版本/);
  assert.match(actionDialogsSource, /已禁止强制覆盖/);
  assert.doesNotMatch(actionDialogsSource, /force adopt/i);
});

test("stale conflict dialog explains baseline/current version relation and safe recovery", () => {
  assert.match(actionDialogsSource, /版本冲突关系/);
  assert.match(actionDialogsSource, /候选基线版本/);
  assert.match(actionDialogsSource, /当前章节版本/);
  assert.match(actionDialogsSource, /线上版本已变更/);
  assert.match(actionDialogsSource, /处理建议/);
  assert.match(actionDialogsSource, /保留当前章节/);
  assert.match(actionDialogsSource, /不能直接采用或覆盖当前章节/);
});

test("candidate batch detail page wires single adopt, discard, bulk adopt, and abandon actions with idempotency keys", () => {
  assert.match(detailPageSource, /adoptChapterPlanCandidate/);
  assert.match(detailPageSource, /discardChapterPlanCandidate/);
  assert.match(detailPageSource, /BatchAdoptDialog/);
  assert.match(detailPageSource, /BatchAbandonDialog/);
  assert.match(detailPageSource, /idempotencyKey/i);
  assert.match(detailPageSource, /expectedCandidateVersion/);
  assert.match(apiSource, /Idempotency-Key/i);
});

test("FE-F4 API contracts export adopt, discard, bulk adopt, and abandon functions", () => {
  assert.match(apiSource, /export function adoptChapterPlanCandidate/);
  assert.match(apiSource, /export function discardChapterPlanCandidate/);
  assert.match(apiSource, /export function adoptChapterPlanCandidates/);
  assert.match(apiSource, /export function abandonChapterPlanCandidateBatch/);
});
