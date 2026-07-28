import assert from "node:assert/strict";
import test from "node:test";
import { readFileSync } from "node:fs";
import { join } from "node:path";
const root = join(process.cwd(), "src", "features", "content-items");
const editor = readFileSync(join(root, "content-editor-workspace.tsx"), "utf8");
const drawer = readFileSync(join(root, "content-generation-drawer.tsx"), "utf8");
test("编辑器加载 Summary，并以同一路由 Tab 和抽屉组织正文生成", () => {
  assert.match(editor, /getContentGenerationSummary/);
  assert.match(editor, /章节目标/); assert.match(editor, /故事情报/); assert.match(editor, /素材库/);
  assert.match(editor, /ContentGenerationDrawer/); assert.doesNotMatch(editor, /content-generation-runs\/.*route/);
});
test("预检阻断不会创建任务，确认提交使用 Token 且防止重复提交", () => {
  assert.match(drawer, /report\.status !== "passed"/); assert.match(drawer, /preflightToken/);
  assert.match(drawer, /submitted\.current/); assert.match(drawer, /getOrCreateOperation/);
  assert.match(drawer, /createContentGenerationRun/); assert.match(drawer, /await onCreated/);
  assert.match(drawer, /重新预检/); assert.match(drawer, /当前条件已变化，请重新预检/);
});
