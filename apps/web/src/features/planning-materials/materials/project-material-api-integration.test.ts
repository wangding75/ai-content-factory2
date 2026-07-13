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
test("all project material routes and pages stay on real APIs without mock switching", () => {
  const sources = [
    "./materials-page.tsx",
    "./create-material-page.tsx",
    "./pick-material-page.tsx",
    "./material-detail-page.tsx",
    "./material-detail-drawer.tsx",
    "./edit-material-page.tsx",
    "./material-usage-page.tsx",
    "./unbind-material-page.tsx",
    "../../../app/projects/[projectId]/materials/page.tsx",
    "../../../app/projects/[projectId]/materials/new/page.tsx",
    "../../../app/projects/[projectId]/materials/pick/page.tsx",
    "../../../app/projects/[projectId]/materials/[materialId]/page.tsx",
    "../../../app/projects/[projectId]/materials/[materialId]/edit/page.tsx",
    "../../../app/projects/[projectId]/materials/[materialId]/usage/page.tsx",
    "../../../app/projects/[projectId]/materials/[materialId]/unbind/page.tsx",
  ].map((path) => readFileSync(new URL(path, import.meta.url), "utf8"));
  for (const source of sources) {
    assert.doesNotMatch(source, /mockScenario|parsePlanningMockScenario|planning-api|materials-api|material-usage-api|unbind-material-api|update-material-usage-api/);
  }
});