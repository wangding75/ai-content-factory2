import assert from "node:assert/strict";
import test from "node:test";
import { readFileSync } from "node:fs";

test("D3 exposes all deterministic mock states", () => {
  const source=readFileSync(new URL("./project-work-api.ts",import.meta.url),"utf8");
  for (const state of ["loading","success","single","empty","error","invalid-pagination","not-found","succeeded","failed","no-run","paged"]) assert.match(source,new RegExp(`"${state}"`));
});
test("D3 page keeps fixture, mapper, pagination and navigation outside raw JSX", () => {
  const source=readFileSync(new URL("./project-works-workspace.tsx",import.meta.url),"utf8");
  assert.match(source,/listProjectWorks\(projectId/);assert.match(source,/toProjectWorkView/);assert.match(source,/setOffset\(offset\+limit\)/);assert.match(source,/创建重写版本/);assert.match(source,/aria-busy/);assert.doesNotMatch(source,/href="#"/);assert.doesNotMatch(source,/JSON\.stringify/);
});
test("D3 routes carry all available contract identifiers", () => {
  const source=readFileSync(new URL("./project-work-presentation.ts",import.meta.url),"utf8");
  for(const key of ["projectId","workId","contentItemId","versionId","sourceVersionId","reviewReportId"])assert.match(source,new RegExp(key));
  assert.match(source,/\/works\/\$\{work\.work_id\}\/rewrite/);
});
