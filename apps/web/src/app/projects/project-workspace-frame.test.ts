import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const root = new URL("./", import.meta.url);
const read = (path: string) => readFileSync(new URL(path, root), "utf8");

test("project workspace routes render the shared frame with their assigned tab", () => {
  const routes = [
    ["[projectId]/page.tsx", "overview"],
    ["[projectId]/planning/page.tsx", "planning"],
    ["[projectId]/materials/page.tsx", "materials"],
    ["[projectId]/storylines/page.tsx", "storylines"],
    ["[projectId]/chapter-plans/page.tsx", "chapter-plans"],
    ["[projectId]/settings/page.tsx", "settings"],
  ] as const;
  for (const [path, tab] of routes) {
    const source = read(path);
    assert.match(source, /ProjectWorkspaceFrame/);
    assert.match(source, new RegExp(`active=[{\"]${tab}`));
  }
  assert.match(read("[projectId]/storylines/page.tsx"), /variant="wide"/);
  const worksRoute = read("[projectId]/works/page.tsx");
  const worksWorkspace = read("../../features/project-works/project-works-workspace.tsx");
  assert.match(worksRoute, /ProjectWorksWorkspace/);
  assert.match(worksRoute, /view === "review"/);
  assert.match(worksWorkspace, /ProjectWorkspaceFrame/);
  assert.match(worksWorkspace, /active="review"/);
  assert.match(worksWorkspace, /active="works"/);
});

test("business workspaces no longer render a second project header or navigation", () => {
  for (const path of [
    "../../features/planning-materials/planning/planning-page.tsx",
    "../../features/planning-materials/materials/materials-page.tsx",
    "../../features/chapter-plans/chapter-plans-workspace.tsx",
  ]) {
    const source = read(path);
    assert.doesNotMatch(source, /ProjectWorkspaceNav/);
    assert.doesNotMatch(source, /project-header|project-head/);
  }
});
