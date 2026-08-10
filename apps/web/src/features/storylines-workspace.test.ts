import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const source = readFileSync(new URL("./storylines-workspace.tsx", import.meta.url), "utf8");

test("storyline workspace loads real chapter plans for relation statistics", () => {
  assert.match(source, /listChapterPlans/);
  assert.match(source, /storyline_refs_json/);
  assert.match(source, /chapterPlans/);
});

test("storyline workspace renders read-only chapter relation counts and distribution", () => {
  assert.match(source, /关联章节统计/);
  assert.match(source, /只读/);
  assert.match(source, /关联章节/);
  assert.match(source, /待确认/);
  assert.match(source, /已确认/);
  assert.match(source, /章节分布/);
  assert.match(source, /暂无关联章节/);
  assert.doesNotMatch(source, /新增关联章节写接口/);
});
