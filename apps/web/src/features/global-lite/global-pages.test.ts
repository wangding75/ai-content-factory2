import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
const page = readFileSync(new URL("./global-pages.tsx", import.meta.url), "utf8");
const api = readFileSync(new URL("./global-lite-api.ts", import.meta.url), "utf8");
test("E1 and E2 use real mapped API clients with loading, retry, pagination, and cancellation", () => {
  for (const text of ["listGlobalMaterials", "listGlobalWorks", "正在加载全局素材", "暂无素材", "暂时无法加载", "上一页", "查看详情", "查看项目作品", "AbortController", "controller.abort"]) assert.match(page, new RegExp(text));
  assert.match(api, /listMaterialsFromApi\(\{ scope: "global"/); assert.match(api, /getMaterialFromApi/); assert.match(api, /apiRequest<Page<WorkVm>>/); assert.doesNotMatch(`${page}\n${api}`, /fixture|mock\?\)/i);
});
test("global client sends frozen scope and maps API DTOs before JSX", () => {
  for (const text of ["scope: \"global\"", "updated_at_desc", "mapWorkflowRun", "mapCapability", "mapIntegration", "ApiRequestInit"]) assert.match(api, new RegExp(text));
});
test("global workflow runs keep the legacy content-workflow-runs route", () => {
  assert.match(api, /`\/content-workflow-runs\$\{pageQuery\(query\)\}`/);
  assert.match(api, /workflowRuns: "\/api\/v1\/content-workflow-runs"/);
  assert.doesNotMatch(api, /`\/workflow-runs\$\{pageQuery\(query\)\}`/);
});
