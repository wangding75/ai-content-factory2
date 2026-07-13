import assert from "node:assert/strict";
import test from "node:test";
import { readFile } from "node:fs/promises";

const sourcePath = new URL("./unbind-material-page.tsx", import.meta.url);

test("unbind page loads its binding and deletes it through the real project material API", async () => {
  const source = await readFile(sourcePath, "utf8");
  assert.match(source, /listProjectMaterialsFromApi\(projectId, \{\}, \{ signal \}\)/);
  assert.match(source, /unbindProjectMaterialFromApi\(projectId, materialId, item\.usage\.version\)/);
  assert.doesNotMatch(source, /getProjectMaterialUsage|unbindProjectMaterial\s*from/);
  assert.doesNotMatch(source, /deleteMaterial|updateMaterial/);
});

test("unbind page prevents duplicate deletion and its cancel path is a link", async () => {
  const source = await readFile(sourcePath, "utf8");
  assert.match(source, /if \(busy\) return/);
  assert.match(source, /disabled=\{busy\}/);
  assert.match(source, /<Link href=\{`\/projects\/\$\{projectId\}\/materials\/\$\{materialId\}`\}>取消<\/Link>/);
  assert.match(source, /VERSION_CONFLICT/);
});