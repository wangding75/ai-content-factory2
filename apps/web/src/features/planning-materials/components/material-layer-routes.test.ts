import assert from "node:assert/strict";
import test from "node:test";
import {closeMaterialLayer, materialDetailRoute, projectMaterialsRoute} from "./material-layer-routes.ts";

const projectId = "00000000-0000-4000-8000-000000000001";
const materialId = "20000000-0000-4000-8000-000000000001";

test("six material layers close to their immediate route parent", () => {
  const list = projectMaterialsRoute(projectId);
  const detail = materialDetailRoute(projectId, materialId);
  assert.equal(closeMaterialLayer("create", projectId), list);
  assert.equal(closeMaterialLayer("pick", projectId), list);
  assert.equal(closeMaterialLayer("detail", projectId, materialId), list);
  assert.equal(closeMaterialLayer("edit", projectId, materialId), detail);
  assert.equal(closeMaterialLayer("usage", projectId, materialId), detail);
  assert.equal(closeMaterialLayer("unbind", projectId, materialId), detail);
});

test("route targets keep the current dynamic identifiers", () => {
  assert.equal(materialDetailRoute("project-a", "material-b"), "/projects/project-a/materials/material-b");
});
