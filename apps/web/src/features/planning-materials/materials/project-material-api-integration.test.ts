import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const listSource = readFileSync(new URL("./materials-page.tsx", import.meta.url), "utf8");
const drawerSource = readFileSync(new URL("./material-detail-drawer.tsx", import.meta.url), "utf8");
const detailSource = readFileSync(new URL("./material-detail-page.tsx", import.meta.url), "utf8");

const whitespace = "\\s*";
const call = (name: string, args: string) => new RegExp(`${name}\\(${args}\\)`);

test("project material list sends searches, filters, sorting, and pages to the real API", () => {
  assert.match(listSource, call("listProjectMaterialsFromApi", `projectId,${whitespace}\\{q,type,sort,limit,offset\\},${whitespace}\\{signal\\}`));
  assert.match(listSource, /setOffset\(offset\+limit\)/);
  assert.match(listSource, /new AbortController\(\)/);
  assert.match(listSource, /controller\.abort\(\)/);
  assert.doesNotMatch(listSource, /listProjectMaterials\(/);
  assert.doesNotMatch(listSource, /material-repository/);
});

test("detail views combine real global detail with current usage from the project material API", () => {
  for (const source of [drawerSource, detailSource]) {
    assert.match(source, /getMaterialFromApi\(materialId,\s*\{\s*signal\s*\}\)/);
    assert.match(source, /listProjectMaterialsFromApi\(projectId,\s*\{\s*\},\s*\{\s*signal\s*\}\)/);
    assert.match(source, /items\.find\(\(item\)\s*=>\s*item\.material\.id\s*===\s*materialId\)\?\.usage/);
    assert.match(source, /new AbortController\(\)/);
    assert.doesNotMatch(source, /listProjectMaterials\(/);
    assert.doesNotMatch(source, /getMaterial\(materialId/);
    assert.doesNotMatch(source, /scenario\s*===\s*["']no-current-usage["']/);
  }
});