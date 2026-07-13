import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const source = readFileSync(new URL("./edit-material-page.tsx", import.meta.url), "utf8");

test("global material editor loads and saves through the real material API", () => {
  assert.match(source, /getMaterialFromApi\(materialId/);
  assert.match(source, /updateMaterialFromApi\(materialId/);
  assert.match(source, /expected_version: saved\.version/);
  assert.doesNotMatch(source, /updateMaterial\(materialId/);
  assert.doesNotMatch(source, /getMaterial\(materialId, scenario/);
});

test("global material editor preserves input for conflicts and prevents duplicate saves", () => {
  assert.match(source, /VERSION_CONFLICT/);
  assert.match(source, /reloadLatest/);
  assert.match(source, /if \(!dirty \|\| saving\) return/);
  assert.match(source, /disabled=\{!dirty \|\| saving\}/);
});

test("global material editor updates only Material fields, not project Usage", () => {
  assert.match(source, /content_json: form\.content_json/);
  assert.match(source, /tags_json: form\.tags_json/);
  assert.doesNotMatch(source, /usage_type/);
  assert.doesNotMatch(source, /updateProjectMaterialUsage/);
});