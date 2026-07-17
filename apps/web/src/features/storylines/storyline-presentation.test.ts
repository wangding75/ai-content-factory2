import assert from "node:assert/strict";
import test from "node:test";
import { chapterRange, childCount, formatChineseDate, isPlaceholderText, storylineName, storylineRelation, storylineStatus, storylineSummary, storylineType } from "./storyline-presentation.ts";

test("storyline presentation maps enums and safe text", () => {
  assert.equal(storylineType("main"), "主故事线"); assert.equal(storylineType("branch"), "支线"); assert.equal(storylineRelation("root"), "根节点"); assert.equal(storylineStatus("paused"), "已暂停"); assert.equal(storylineStatus("other"), "未知状态");
  assert.equal(isPlaceholderText("C2 ????"), true); assert.equal(isPlaceholderText("这是问题？"), false); assert.equal(storylineName("C2 ????"), "故事线名称待完善"); assert.equal(storylineSummary("?? C2 ??"), "暂无故事线摘要");
  assert.equal(chapterRange({ start_chapter: 1, end_chapter: 3 }), "第 1～3 章"); assert.equal(chapterRange({ start_chapter: 1, end_chapter: 1 }), "第 1 章"); assert.equal(chapterRange({ start_chapter: null, end_chapter: null }), "未设置"); assert.equal(childCount(0), "暂无子故事线"); assert.equal(childCount(3), "3 条子故事线");
  assert.match(formatChineseDate("2026-07-15T05:58:13.263318Z"), /2026年7月15日/);
});
