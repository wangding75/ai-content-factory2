import assert from "node:assert/strict";
import test from "node:test";
import { chapterPlanDetail, chapterPlanStatusLabel, chapterPlanSummary, relationValues } from "./chapter-plan-presentation.ts";

test("chapter plan statuses are presented in Chinese with a safe fallback", () => {
  assert.equal(chapterPlanStatusLabel("pending_confirmation"), "待确认");
  assert.equal(chapterPlanStatusLabel("confirmed"), "已确认");
  assert.equal(chapterPlanStatusLabel("unexpected"), "未知状态");
});

test("technical generation parameters are converted to a safe natural-language summary", () => {
  assert.equal(
    chapterPlanSummary("medium pace balanced chapter 1; main=true children=true materials=true unpaid_foreshadowings=true prior_summaries=true"),
    "中等节奏推进，并参考主线、支线、项目素材、未回收伏笔、前文摘要。",
  );
  assert.equal(chapterPlanDetail("Advance ????", "暂未设置章节目标"), "暂未设置章节目标");
});

test("relation values use loaded names and a safe empty state", () => {
  assert.deepEqual(relationValues(["a"], new Map([["a", "主线"]]), "暂无关联故事线"), ["主线"]);
  assert.deepEqual(relationValues(["missing"], new Map(), "暂无关联故事线"), ["暂无关联故事线"]);
});
