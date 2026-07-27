import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const workspaceSource = readFileSync(
  new URL("./chapter-plans-workspace.tsx", import.meta.url),
  "utf8",
);
const drawerSource = readFileSync(
  new URL("./generation-settings-drawer.tsx", import.meta.url),
  "utf8",
);
const preflightSource = readFileSync(
  new URL("./preflight-dialogs.tsx", import.meta.url),
  "utf8",
);

test("chapter plans batch-load relation names and summary", () => {
  assert.match(
    workspaceSource,
    /Promise\.allSettled\(\[[\s\S]*listChapterPlans\(projectId, \{ limit: 100 \}, \{ signal \}\),[\s\S]*getProjectChapterPlanningSummary\(projectId, \{ signal \}\)/,
  );
  assert.match(workspaceSource, /createRelationNames/);
  assert.doesNotMatch(workspaceSource, /"\?\?\?"/);
});

test("chapter plans render active run banners and preflight controls", () => {
  assert.match(workspaceSource, /SummaryRunBanner/);
  assert.match(workspaceSource, /P15_C1_NOT_CONFIGURED/);
  assert.match(workspaceSource, /P15_C1_FAILED_ATOMIC/);
  assert.match(workspaceSource, /GenerationSettingsDrawer/);
  assert.match(workspaceSource, /PreflightReportDialog/);
  assert.match(workspaceSource, /preflightChapterPlanRun/);
  assert.match(workspaceSource, /createChapterPlanRun/);
});

test("generation settings drawer validates input ranges and options", () => {
  assert.match(drawerSource, /生成模式/);
  assert.match(drawerSource, /起始章节/);
  assert.match(drawerSource, /结束章节/);
  assert.match(drawerSource, /结束章节不能小于起始章节/);
  assert.match(drawerSource, /请至少选择一条指定故事线/);
  assert.match(drawerSource, /includeProjectMaterials/);
});

test("preflight report dialog differentiates passed and blocked status", () => {
  assert.match(preflightSource, /预检通过/);
  assert.match(preflightSource, /预检阻断/);
  assert.match(preflightSource, /阻断原因列表/);
  assert.match(preflightSource, /确认发起生成/);
});
