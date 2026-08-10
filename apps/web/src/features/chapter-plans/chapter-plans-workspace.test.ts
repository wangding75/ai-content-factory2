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
const blockerSource = readFileSync(
  new URL("../../components/workflow-preflight-blocker.tsx", import.meta.url),
  "utf8",
);
const preflightProgressSource = preflightSource.slice(
  preflightSource.indexOf("export function PreflightProgressDialog"),
  preflightSource.indexOf("export interface PreflightReportDialogProps"),
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
  assert.match(workspaceSource, /生成失败，本次没有新候选写入/);
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

test("chapter plan statistics and filters use real status values without sample fallbacks", () => {
  assert.match(workspaceSource, /createChapterPlanStats\(plans \?\? \[\]\)/);
  assert.match(workspaceSource, /matchesChapterPlanStatus/);
  assert.match(workspaceSource, /plan\.status === "draft_generated"/);
  assert.match(workspaceSource, /候选批次[^\n]*summary\?\.candidateBatchCounts\?\.ready \?\? 0/);
  assert.doesNotMatch(workspaceSource, /candidateBatchCounts\?\.ready \?\? 2/);
});

test("failed chapter plan runs expose zero-write copy and only safe recovery actions", () => {
  assert.match(workspaceSource, /任务失败。[^\n]*本次没有新候选被采用或写入/);
  assert.match(workspaceSource, /summaryError\.code === "result_consumption_failed"/);
  assert.match(workspaceSource, /重试结果消费/);
  assert.match(workspaceSource, /查看运行详情/);
});

test("unconfigured chapter planning keeps existing plans and routes setup outside generation", () => {
  assert.match(workspaceSource, /workflowNotConfigured/);
  assert.match(workspaceSource, /disabled=\{workflowNotConfigured\}/);
  assert.match(workspaceSource, /尚未完成执行配置/);
  assert.match(workspaceSource, /前往项目设置/);
  assert.match(workspaceSource, /settings\?tab=workflow-bindings/);
  assert.match(workspaceSource, /重新检查/);
  assert.match(workspaceSource, /plans\?\.length \? "未找到匹配章节" : "暂无章节规划"/);
});

test("generation settings drawer validates input ranges and options", () => {
  assert.match(drawerSource, /生成模式/);
  assert.match(drawerSource, /起始章节/);
  assert.match(drawerSource, /结束章节/);
  assert.match(drawerSource, /结束章节不能小于起始章节/);
  assert.match(drawerSource, /请至少选择一条指定故事线/);
  assert.match(drawerSource, /includeProjectMaterials/);
});

test("generation settings drawer groups parameters and repeats a preflight-safe summary", () => {
  assert.match(drawerSource, /生成范围/);
  assert.match(drawerSource, /章节范围 \/ 主线选择/);
  assert.match(drawerSource, /其他生成参数/);
  assert.match(drawerSource, /generationRangeSummary/);
  assert.match(drawerSource, /本次生成范围/);
  assert.match(drawerSource, /仅执行预检，不创建任务，不调用 n8n 或 LLM/);
  assert.match(drawerSource, /确认并预检/);
  assert.match(drawerSource, /onSubmit\(payload\)/);
});

test("preflight progress dialog exposes one active check without a second submit", () => {
  assert.match(preflightProgressSource, /正在执行章节规划预检/);
  assert.match(preflightProgressSource, /预检进行中/);
  assert.match(preflightProgressSource, /检查工作流配置与连接/);
  assert.match(preflightProgressSource, /预检阶段不会创建任务，也不会调用 n8n 或 LLM/);
  assert.match(preflightProgressSource, /aria-current="step"/);
  assert.doesNotMatch(preflightProgressSource, /确认发起生成/);
  assert.doesNotMatch(preflightProgressSource, /onCreateRun/);
});

test("passed preflight report shows checks, execution summary, and the create-task action", () => {
  assert.match(preflightSource, /确认章节规划任务/);
  assert.match(preflightSource, /预检通过，具备生成条件/);
  assert.match(preflightSource, /任务概览/);
  assert.match(preflightSource, /执行配置/);
  assert.match(preflightSource, /预检结果详情/);
  assert.match(preflightSource, /preflightCheckLabel/);
  assert.match(preflightSource, /workflowBindingVersion/);
  assert.match(preflightSource, /onCreateRun\(passedReport\.preflightToken\)/);
  assert.match(preflightSource, /创建任务/);
  assert.doesNotMatch(preflightSource, /确认发起生成/);
});

test("blocked preflight report lists repair directions and has no create-task action", () => {
  assert.match(preflightSource, /预检阻断，暂时无法创建任务/);
  assert.match(preflightSource, /WorkflowPreflightBlocker/);
  assert.match(preflightSource, /返回修改/);
  assert.doesNotMatch(preflightSource, /blockedReport[\s\S]*确认发起生成/);
});

test("preflight report dialog differentiates passed and blocked status", () => {
  assert.match(preflightSource, /预检通过/);
  assert.match(preflightSource, /预检阻断/);
  assert.match(preflightSource, /创建任务/);
  assert.doesNotMatch(preflightSource, /blockerReasonLabel|blockerTitleLabel|retryActionLabel/);
  assert.doesNotMatch(preflightSource, /代码：\{item\.code\}/);
  assert.doesNotMatch(preflightSource, /\{item\.message\}/);
});

test("chapter planning uses the shared preflight blocker with title, reason, and repair direction", () => {
  assert.match(blockerSource, /当前无法启动工作流/);
  assert.match(blockerSource, /blockerTitle/);
  assert.match(blockerSource, /repairDirection/);
  assert.match(blockerSource, /建议操作：/);
  assert.match(blockerSource, /前往修复/);
  assert.match(preflightSource, /toWorkflowPreflightReasons\(blockedReport\.blockers\)/);
});

test("blocked preflight report omits unavailable summaries without dereferencing null", () => {
  assert.match(preflightSource, /const inputSummary = report\.inputSummary/);
  assert.match(preflightSource, /const modeLabel = inputSummary/);
  assert.match(preflightSource, /\{modeLabel && targetLabel && \(/);
  assert.doesNotMatch(preflightSource, /report\.inputSummary\.(generationMode|target)/);
});

test("run-created dialog distinguishes task creation from result completion", () => {
  assert.match(preflightSource, /RunCreatedDialog\(\{ runId, onClose \}: RunCreatedDialogProps\)/);
  assert.match(preflightSource, /任务创建成功/);
  assert.match(preflightSource, /Run ID/);
  assert.match(preflightSource, /Run 已创建，当前等待执行/);
  assert.match(preflightSource, /任务创建成功不代表章节规划结果已经生成/);
  assert.match(preflightSource, /workflow-runs\/\$\{encodeURIComponent\(runId\)\}/);
  assert.match(preflightSource, /查看运行详情/);
  assert.doesNotMatch(preflightSource, /章节规划生成任务已完成/);
});

test("chapter planning workspace exposes only the real preflight entry point", () => {
  const source = readFileSync(new URL("./chapter-plans-workspace.tsx", import.meta.url), "utf8");
  assert.doesNotMatch(source, /MockGenerateDialog|mockOpen|模拟生成/);
});
