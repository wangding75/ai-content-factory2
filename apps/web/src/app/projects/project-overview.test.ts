import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const source = readFileSync(new URL("./[projectId]/page.tsx", import.meta.url), "utf8");

test("project overview keeps real project types, progress-driven navigation, and the materials entry", () => {
  assert.match(source, /去完善策划/);
  assert.match(source, /<Link\s+className="overview-materials-link"\s+href=\{"\/projects\/" \+ id \+ "\/materials"\}\s*>\s*添加项目素材\s*<\/Link>/);
  assert.doesNotMatch(source, /<button disabled>添加项目素材<\/button>/);
  assert.doesNotMatch(source, /素材暂未开放/);
  assert.match(source, /types\[project\.type\]/);
  assert.match(source, /const nextStep = nextSteps\[project\.current_stage\]/);
  assert.match(source, /href=\{`\/projects\/\$\{id\}\/\$\{nextStep\.href\}`\}/);
});
