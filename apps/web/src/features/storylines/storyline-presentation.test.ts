import assert from "node:assert/strict";
import test from "node:test";
import { buildStorylineTree, chapterRange, childCount, formatChineseDate, isPlaceholderText, storylineName, storylineRelation, storylineStatus, storylineSummary, storylineType } from "./storyline-presentation.ts";
import type { StorylineNode } from "@/lib/api";

const node = (id: string, parent_id: string | null, name: string, start: number, end: number): StorylineNode => ({ id, project_id: "p", parent_id, type: parent_id ? "child" : "main", relation: parent_id ? "child" : "root", name, summary: "summary", start_chapter: start, end_chapter: end, status: "active", sort_order: 0, version: 1, created_at: "2026-07-15T05:58:13.263318Z", updated_at: "2026-07-15T05:58:13.263318Z", children: [] });

test("storyline presentation maps enums and safe text", () => {
  assert.equal(storylineType("main"), "主故事线"); assert.equal(storylineType("branch"), "支线"); assert.equal(storylineRelation("root"), "根节点"); assert.equal(storylineStatus("paused"), "已暂停"); assert.equal(storylineStatus("other"), "未知状态");
  assert.equal(isPlaceholderText("C2 ????"), true); assert.equal(isPlaceholderText("这是问题？"), false); assert.equal(storylineName("C2 ????"), "故事线名称待完善"); assert.equal(storylineSummary("?? C2 ??"), "暂无故事线摘要");
  assert.equal(chapterRange({ start_chapter: 1, end_chapter: 3 }), "第 1～3 章"); assert.equal(chapterRange({ start_chapter: 1, end_chapter: 1 }), "第 1 章"); assert.equal(chapterRange({ start_chapter: null, end_chapter: null }), "未设置"); assert.equal(childCount(0), "暂无子故事线"); assert.equal(childCount(3), "3 条子故事线"); assert.match(formatChineseDate("2026-07-15T05:58:13.263318Z"), /2026年7月15日/);
});

test("tree builder attaches flat items once and safely keeps invalid parents as roots", () => {
  const q = node("q", null, "q", 1, 1); const asda = node("asda", "q", "asda", 2, 0); const orphan = node("orphan", "missing", "orphan", 3, 3); const cycle = node("cycle", "cycle", "cycle", 4, 4);
  const tree = buildStorylineTree([q, asda, orphan, cycle]);
  assert.deepEqual(tree.map((item) => item.id), ["q", "orphan", "cycle"]);
  assert.deepEqual(tree[0].children.map((item) => item.id), ["asda"]);
  assert.equal(tree.flatMap((item) => [item, ...item.children]).length, 4);
});
