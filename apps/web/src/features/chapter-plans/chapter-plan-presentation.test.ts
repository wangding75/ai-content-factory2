import assert from "node:assert/strict";
import test from "node:test";
import { chapterPlanStatusLabel, relationValues } from "./chapter-plan-presentation.ts";

test("chapter plan statuses are presented in Chinese with a safe fallback", () => {
  assert.equal(chapterPlanStatusLabel("pending_confirmation"), "待确认");
  assert.equal(chapterPlanStatusLabel("confirmed"), "已确认");
  assert.equal(chapterPlanStatusLabel("unexpected"), "未知状态");
});

test("relation values use loaded names and a safe empty state", () => {
  assert.deepEqual(relationValues(["a"], new Map([["a", "主线"]]), "暂无关联故事线"), ["主线"]);
  assert.deepEqual(relationValues(["missing"], new Map(), "暂无关联故事线"), ["暂无关联故事线"]);
});
