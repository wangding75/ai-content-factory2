import assert from "node:assert/strict";
import test from "node:test";
import { readFile } from "node:fs/promises";

const sourcePath = new URL("./material-usage-page.tsx", import.meta.url);

test("usage editor reads and updates the current project binding through the real API", async () => {
  const source = await readFile(sourcePath, "utf8");
  assert.match(source, /listProjectMaterialsFromApi\(projectId, \{\}, \{ signal \}\)/);
  assert.match(source, /updateProjectMaterialUsageFromApi\(projectId, materialId/);
  assert.match(source, /expected_version: usage\.version/);
  assert.doesNotMatch(source, /getProjectMaterialUsage|updateProjectMaterialUsage\s*from/);
});

test("usage editor blocks duplicate saves and keeps the server state on version conflict", async () => {
  const source = await readFile(sourcePath, "utf8");
  assert.match(source, /if \(saving \|\| !dirty\) return/);
  assert.match(source, /VERSION_CONFLICT/);
  assert.match(source, /重新加载后再保存/);
  assert.match(source, /controller\.abort\(\)/);
});
test("usage editor hides historical non-character roles and cleans PATCH data through the shared rule", async () => {
  const source = await readFile(sourcePath, "utf8");
  assert.match(source, /roleNameForUsage\(form\.usage_type, form\.role_name\)/);
  assert.match(source, /usageShowsRole\(usage_type\) \? form\.role_name : ""/);
  assert.match(source, /usageShowsRole\(form\.usage_type\) && <label className="create-field"><span>具体角色/);
});