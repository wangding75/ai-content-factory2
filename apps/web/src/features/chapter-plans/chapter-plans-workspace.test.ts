import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const source = readFileSync(new URL("./chapter-plans-workspace.tsx", import.meta.url), "utf8");

test("chapter plans batch-load relation names and never expose identifiers", () => {
  assert.match(source, /Promise\.all\(\[[\s\S]*getStorylines\(projectId, signal\),[\s\S]*listProjectMaterialsFromApi\(projectId, \{ limit: 100 \}, \{ signal \}\),[\s\S]*getForeshadowings\(projectId, signal\)/);
  assert.match(source, /createRelationNames/);
  assert.doesNotMatch(source, /slice\(0, 8\)/);
  assert.doesNotMatch(source, /"\?\?\?"/);
});

test("chapter plans render client-side search, relation filters, statistics, and batch confirmation controls", () => {
  assert.match(source, /chapterPlanStatusLabel/);
  assert.match(source, /createChapterPlanStats/);
  assert.match(source, /搜索章节标题或章节编号/);
  assert.match(source, /故事线筛选/);
  assert.match(source, /伏笔筛选/);
  assert.match(source, /批量确认章节规划/);
  assert.doesNotMatch(source, /disabled title=/);
});
