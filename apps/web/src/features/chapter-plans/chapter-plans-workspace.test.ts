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

test("chapter plans use presentation mappers for filters, statuses, and sources", () => {
  assert.match(source, /chapterPlanStatusLabel/);
  assert.match(source, /chapterPlanGenerationSummary/);
  assert.doesNotMatch(source, /disabled title=/);
});
