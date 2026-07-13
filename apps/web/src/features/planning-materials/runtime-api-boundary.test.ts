import assert from "node:assert/strict";
import { existsSync, readFileSync, readdirSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";import test from "node:test";

const featureRoot = new URL("./", import.meta.url);
const read = (path: string) => readFileSync(new URL(path, featureRoot), "utf8");

function productionFiles(directory: string): string[] {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const path = join(directory, entry.name);
    if (entry.isDirectory()) return productionFiles(path);
    return entry.isFile() && /\.tsx?$/.test(entry.name) && !entry.name.endsWith(".test.ts") ? [path] : [];
  });
}

test("production Planning and Material sources cannot import or switch to Mock runtime paths", () => {
  const files = productionFiles(fileURLToPath(featureRoot));
  assert.ok(files.length > 0);
  for (const file of files) {
    const source = readFileSync(file, "utf8");
    assert.doesNotMatch(source, /from\s+["'][^"']*\/(?:mock|fixtures?)(?:\/|["'])/);
    assert.doesNotMatch(source, /mockScenario|PlanningMockScenario|parsePlanningMockScenario/);
    assert.doesNotMatch(source, /catch[\s\S]{0,240}(?:mock|fixture|fallback)/i);
  }
  for (const retired of ["api/planning-api.ts", "api/material-detail-api.ts", "api/update-material-api.ts", "mock"]) {
    assert.equal(existsSync(new URL(retired, featureRoot)), false, `${retired} must not remain in production`);
  }
});

test("Planning page loads and saves exclusively with real GET and PUT API helpers", () => {
  const api = read("api/planning-http-api.ts");
  const page = read("planning/planning-page.tsx");
  assert.match(api, /apiRequest<PlanningResponse>\(`\/projects\/\$\{encodeURIComponent\(projectId\)\}\/planning`, init\)/);
  assert.match(api, /method: "PUT"/);
  assert.match(page, /getProjectPlanningFromApi\(projectId, \{ signal \}\)/);
  assert.match(page, /saveProjectPlanningToApi\(projectId,/);
  assert.match(page, /VERSION_CONFLICT/);
  assert.doesNotMatch(page, /planning-api|mockScenario/);
});

test("global Material list, detail, and editor use real HTTP APIs with loading, empty, retry, and conflict states", () => {
  const api = read("api/material-http-api.ts");
  const list = read("materials/global-materials-page.tsx");
  const detail = read("materials/material-detail-page.tsx");
  const editor = read("materials/edit-material-page.tsx");
  assert.match(api, /\/materials\$\{materialQuery\(query\)\}/);
  assert.match(api, /method: "POST"/);
  assert.match(api, /method: "PATCH"/);
  assert.match(list, /listMaterialsFromApi\(\{ q, type, sort, limit, offset \}/);
  assert.match(list, /onClick=\{\(\) => void load\(\)\}/);
  assert.match(detail, /getMaterialFromApi\(materialId,\{signal\}\)/);
  assert.match(editor, /getMaterialFromApi\(materialId/);
  assert.match(editor, /updateMaterialFromApi\(materialId/);
  assert.match(editor, /VERSION_CONFLICT/);
});

test("project Material list and all direct routes use real APIs, keep Usage isolated, and protect mutations", () => {
  const pages = [
    "materials/materials-page.tsx", "materials/create-material-page.tsx", "materials/pick-material-page.tsx",
    "materials/material-detail-page.tsx", "materials/edit-material-page.tsx", "materials/material-usage-page.tsx", "materials/unbind-material-page.tsx",
  ].map(read);
  for (const source of pages) assert.doesNotMatch(source, /mockScenario|material-repository|planning-api/);
  assert.match(pages[0], /listProjectMaterialsFromApi\(projectId,\{q,type,sort,limit,offset\}/);
  assert.match(pages[1], /createProjectMaterialFromApi\(projectId,/);
  assert.match(pages[1], /if \(saving\) return/);
  assert.match(pages[2], /bindProjectMaterialFromApi\(projectId, selected\.id,/);
  assert.match(pages[2], /if \(submitting \|\| !selected \|\| bound\.has\(selected\.id\)/);
  assert.match(pages[3], /listProjectMaterialsFromApi\(projectId,\{\},\{signal\}\)/);
  assert.match(pages[4], /updateMaterialFromApi\(materialId/);
  assert.match(pages[5], /updateProjectMaterialUsageFromApi\(projectId, materialId/);
  assert.match(pages[5], /VERSION_CONFLICT/);
  assert.match(pages[6], /unbindProjectMaterialFromApi\(projectId, materialId, item\.usage\.version\)/);
  assert.match(pages[6], /if \(busy\) return/);
  assert.doesNotMatch(pages[5], /updateMaterialFromApi/);
  assert.doesNotMatch(pages[6], /deleteMaterial|updateMaterial/);
});

test("route entrypoints support direct navigation and API requests stay on the same-origin rewrite", () => {
  for (const route of [
    "../../app/projects/[projectId]/planning/page.tsx", "../../app/projects/[projectId]/materials/page.tsx",
    "../../app/projects/[projectId]/materials/new/page.tsx", "../../app/projects/[projectId]/materials/pick/page.tsx",
    "../../app/projects/[projectId]/materials/[materialId]/page.tsx", "../../app/projects/[projectId]/materials/[materialId]/edit/page.tsx",
    "../../app/projects/[projectId]/materials/[materialId]/usage/page.tsx", "../../app/projects/[projectId]/materials/[materialId]/unbind/page.tsx",
    "../../app/materials/page.tsx",
  ]) {
    const source = readFileSync(new URL(route, featureRoot), "utf8");
    assert.doesNotMatch(source, /mockScenario|mock\//);
    assert.match(source, /Page|PlanningPage|MaterialsPage|Material/);
  }
  const rewrite = readFileSync(new URL("../../../next.config.ts", featureRoot), "utf8");
  assert.match(rewrite, /source: "\/api\/v1\/:path\*"/);
  assert.match(rewrite, /destination: `\$\{apiBaseUrl\}\/:path\*`/);
});