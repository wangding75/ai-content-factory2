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
  assert.match(workspaceSource, /未配置章节规划工作流/);
  assert.match(workspaceSource, /生成失败且零候选写入/);
  assert.doesNotMatch(workspaceSource, /P15_C\d+/);
  assert.match(workspaceSource, /GenerationSettingsDrawer/);
  assert.match(workspaceSource, /PreflightReportDialog/);
  assert.match(workspaceSource, /preflightChapterPlanRun/);
  assert.match(workspaceSource, /createChapterPlanRun/);
});

test("active run banner separates result validation from generation progress", () => {
  assert.match(workspaceSource, /isResultValidating/);
  assert.match(workspaceSource, /hasStoredResult/);
  assert.match(workspaceSource, /结果校验中/);
  assert.match(workspaceSource, /候选尚未写入/);
  assert.match(workspaceSource, /预计生成\{reqCount\}个章节候选/);
  assert.doesNotMatch(workspaceSource, /const stageLabel = run\.stage === "validating" \? "校验生成结果" : "校验生成结果"/);
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
  assert.match(preflightSource, /blockerReasonLabel/);
  assert.match(preflightSource, /blockerTitleLabel/);
  assert.match(preflightSource, /retryActionLabel/);
  assert.doesNotMatch(preflightSource, /代码：\{item\.code\}/);
  assert.doesNotMatch(preflightSource, /\{item\.message\}/);
});

test("blocked preflight report omits unavailable summaries without dereferencing null", () => {
  assert.match(preflightSource, /const inputSummary = report\.inputSummary/);
  assert.match(preflightSource, /const modeLabel = inputSummary/);
  assert.match(preflightSource, /\{modeLabel && targetLabel && \(/);
  assert.doesNotMatch(preflightSource, /report\.inputSummary\.(generationMode|target)/);
});

test("chapter planning workspace exposes only the real preflight entry point", () => {
  const source = readFileSync(new URL("./chapter-plans-workspace.tsx", import.meta.url), "utf8");
  assert.doesNotMatch(source, /MockGenerateDialog|mockOpen|模拟生成/);
});
