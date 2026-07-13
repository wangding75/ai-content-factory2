import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const source = readFileSync(new URL("./planning-page.tsx", import.meta.url), "utf8");

test("Planning load error retries the real GET and links back to the project overview", () => {
  assert.match(source, /getProjectPlanningFromApi\(projectId, \{ signal \}\)/);
  assert.match(source, /<PlanningLoadError projectId=\{projectId\} retry=\{load\} \/>/);
  assert.match(source, /<button onClick=\{retry\}>重试<\/button>/);
  assert.match(source, /<Link href=\{`\/projects\/\$\{projectId\}`\}>返回项目概览<\/Link>/);
  assert.doesNotMatch(source, /planning-api|mockScenario/);
});
test("planning source derives read-only completion from persisted version and content", async () => {
  const planningSource = source;
  assert.match(source, /isPlanningCompleted/);
  assert.match(source, /编辑策划方案/);
  assert.match(source, /策划已完成/);
  assert.doesNotMatch(source, /策划推进中/);
});