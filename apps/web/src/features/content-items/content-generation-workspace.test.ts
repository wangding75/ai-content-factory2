import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import test from "node:test";

const root = join(process.cwd(), "src", "features", "content-items");
const editor = readFileSync(join(root, "content-editor-workspace.tsx"), "utf8");
const drawer = readFileSync(join(root, "content-generation-drawer.tsx"), "utf8");
const status = readFileSync(join(root, "content-generation-status.tsx"), "utf8");
const candidate = readFileSync(join(root, "content-candidate-compare.tsx"), "utf8");
const locale = readFileSync(join(root, "content-generation-locale.ts"), "utf8");
const reviewWorkspace = readFileSync(join(process.cwd(), "src", "features", "content-review", "content-review-workspace.tsx"), "utf8");
const reviewHistory = readFileSync(join(process.cwd(), "src", "features", "content-review", "content-review-history.tsx"), "utf8");
const rewriteWorkspace = readFileSync(join(process.cwd(), "src", "features", "project-works", "rewrite-workspace.tsx"), "utf8");
const rewriteResultPanel = readFileSync(join(process.cwd(), "src", "features", "project-works", "rewrite-result-panel.tsx"), "utf8");
const rewriteRoute = readFileSync(join(process.cwd(), "src", "app", "projects", "[projectId]", "works", "[workId]", "rewrite", "page.tsx"), "utf8");

test("content generation remains on the editor route with its three context tabs", () => {
  assert.match(editor, /getContentGenerationSummary/);
  assert.match(editor, /ContentGenerationDrawer/);
  assert.match(editor, /role="tablist"/);
  assert.doesNotMatch(editor, /content-generation-runs\/.*route/);
});

test("chapter goal context is an independent read-only area backed by the chapter plan", () => {
  assert.match(editor, /useState<"goal" \| "story" \| "materials">\("goal"\)/);
  assert.match(editor, /listChapterPlans\(/);
  assert.match(editor, /className="content-goal-panel"/);
  assert.match(editor, /来自当前章节规划的只读创作上下文/);
  assert.match(editor, /plan\?\.chapter_goal/);
  assert.match(editor, /plan\?\.creation_notes/);
  assert.match(editor, /plan\?\.summary/);
  assert.match(editor, /className="content-goal-readonly"/);
  assert.match(editor, /contextTab === "story"/);
  assert.match(editor, /contextTab === "materials"/);
});

test("story context resolves the current plan references without changing the editor", () => {
  assert.match(editor, /getStorylines\(/);
  assert.match(editor, /getForeshadowings\(/);
  assert.match(editor, /Promise\.all\(\[[\s\S]*getStorylines\(projectId, c\.signal\)[\s\S]*getForeshadowings\(projectId, c\.signal\)/);
  assert.match(editor, /className="content-story-panel"/);
  assert.match(editor, /关联故事线/);
  assert.match(editor, /关联伏笔/);
  assert.match(editor, /storylineName\(storyline\.name\)/);
  assert.match(editor, /foreshadowing\?\.description/);
});

test("materials context shows real project materials and an explicit empty state", () => {
  assert.match(editor, /listProjectMaterialsFromApi\(/);
  assert.match(editor, /material_refs_json/);
  assert.match(editor, /className="content-material-panel"/);
  assert.match(editor, /可用素材/);
  assert.match(editor, /本章引用/);
  assert.match(editor, /已引用/);
  assert.match(editor, /可引用/);
  assert.match(editor, /暂无可用素材/);
  assert.doesNotMatch(editor, /新增素材|编辑素材|删除素材/);
});

test("generation confirmation keeps the frozen preflight and create boundary while exposing run context", () => {
  const drawerSource = readFileSync(join(root, "content-generation-drawer.tsx"), "utf8");
  assert.match(drawerSource, /chapterLabel/);
  assert.match(drawerSource, /工作流概览/);
  assert.match(drawerSource, /目标章节/);
  assert.match(drawerSource, /候选版本/);
  assert.match(drawerSource, /contextSummary\.materialCount/);
  assert.match(drawerSource, /contextSummary\.storylineCount/);
  assert.match(drawerSource, /content-generation-confirm/);
  assert.match(drawerSource, /preflightContentGenerationRun/);
  assert.match(drawerSource, /createContentGenerationRun/);
  assert.match(drawerSource, /preflightToken/);
});

test("generation requirements stay visible, editable, and distinguish empty from filled", () => {
  const drawerSource = readFileSync(join(root, "content-generation-drawer.tsx"), "utf8");
  assert.match(drawerSource, /content-generation-requirements-state/);
  assert.match(drawerSource, /已填写生成要求/);
  assert.match(drawerSource, /尚未填写生成要求/);
  assert.match(drawerSource, /instructions\.trim\(\)/);
  assert.match(drawerSource, /value=\{instructions\}/);
  assert.match(drawerSource, /setInstructions/);
});

test("queued generation exposes run details and safe cancellation without candidate or progress actions", () => {
  assert.match(status, /summary\.state === "queued"/);
  assert.match(status, /href=\{`\/workflow-runs\/\$\{run\.id\}`\}/);
  assert.match(status, /content-generation-cancel/);
  assert.match(status, /cancelWorkflowRun/);
  assert.match(status, /content-generation-cancel:\$\{run\.id\}/);
  assert.doesNotMatch(status, /summary\.state === "queued"[^\n]*onCandidate/);
});

test("running generation keeps the editor-level status compact and shows real run timing and event context", () => {
  assert.match(status, /summary\.state === "running"/);
  assert.match(status, /run\.startedAt \?\? run\.createdAt/);
  assert.match(status, /latestEvent \? eventLabel\(latestEvent\.eventType\)/);
  assert.match(status, /content-generation-running-meta/);
  assert.match(status, /visibleEvents\.length/);
  assert.doesNotMatch(status, /content-editor-state/);
});

test("candidate-ready status presents only a real candidate without changing the current version", () => {
  assert.match(status, /summary\.state === "candidate_ready"/);
  assert.match(status, /summary\.latestCandidateVersion/);
  assert.match(status, /copy\.candidateReadyDetail\(candidate\.version_no\)/);
  assert.match(status, /copy\.candidateVersion\(candidate\.version_no\)/);
  assert.match(status, /candidate && <button onClick=\{onCandidate\}/);
  assert.match(locale, /candidateReadyDetail/);
  assert.match(locale, /candidateReadyMissing/);
  assert.doesNotMatch(status, /run\.status === "succeeded"/);
  assert.doesNotMatch(status, /setCurrentContentVersion/);
});

test("generation failures stay distinct, safe, and recoverable without a candidate success action", () => {
  assert.match(status, /copy\.titles\.runtime_failed/);
  assert.match(status, /copy\.titles\.output_validation_failed/);
  assert.match(status, /copy\.titles\.result_consumption_failed/);
  assert.match(status, /copy\.runtimeFailedDetail/);
  assert.match(status, /copy\.outputValidationFailedDetail/);
  assert.match(status, /copy\.resultConsumptionFailedDetail/);
  assert.match(status, /summary\.state === "result_consumption_failed"/);
  assert.match(status, /copy\.retryConsumption/);
  assert.match(status, /href=\{`\/workflow-runs\/\$\{run\.id\}`\}/);
  assert.match(status, /summary\.state === "candidate_ready" && candidate && <button onClick=\{onCandidate\}/);
});

test("candidate compare separates version navigation from the read-only content and adopts only after confirmation", () => {
  assert.match(candidate, /content-candidate-switch/);
  assert.match(candidate, /role="tablist"/);
  assert.match(candidate, /role="tab"/);
  assert.match(candidate, /aria-selected=\{selected === "current"\}/);
  assert.match(candidate, /aria-selected=\{selected === "candidate"\}/);
  assert.match(candidate, /content-candidate-content/);
  assert.match(candidate, /content-candidate-actions/);
  assert.match(candidate, /onClick=\{\(\) => setConfirming\(true\)\}/);
  assert.match(candidate, /confirmSetCurrent/);
  assert.match(candidate, /setCurrentContentVersion/);
  assert.doesNotMatch(candidate, /onClick=\{apply\}/);
});

test("unconfigured generation is a centered blocking state with a workflow binding entry", () => {
  assert.match(status, /summary\.state === "not_configured"/);
  assert.match(status, /content-generation-not-configured-body/);
  assert.match(status, /copy\.notConfiguredDetail/);
  assert.match(status, /tab=workflow-bindings/);
  assert.match(status, /copy\.configureWorkflow/);
  assert.match(status, /content-generation-status-\$\{summary\.state\}/);
  assert.match(editor, /generationSummary\?\.canGenerate === false/);
  assert.doesNotMatch(status, /summary\.state === "not_configured"[^]*confirmCreate/);
});

test("editor review entry stays in the actions bar and opens the existing review drawer only when allowed", () => {
  assert.match(editor, /className="content-review-link"/);
  assert.match(editor, /reviewCopy\.editor\.submit/);
  assert.match(editor, /onClick=\{\(\) => setReviewDrawer\(true\)\}/);
  assert.match(editor, /disabled=\{dirty \|\| !reviewSummary\?\.canStartReview\}/);
  assert.match(editor, /ContentReviewDrawer/);
  assert.match(editor, /window\.location\.assign\(reviewPath\)/);
});

test("review submission drawer keeps a real subject, scope summary, workflow section, and fixed footer", () => {
  const reviewDrawer = readFileSync(join(process.cwd(), "src", "features", "content-review", "content-review-drawer.tsx"), "utf8");
  assert.match(reviewDrawer, /review-drawer-source/);
  assert.match(reviewDrawer, /review-drawer-overview/);
  assert.match(reviewDrawer, /copy\.drawer\.target/);
  assert.match(reviewDrawer, /copy\.drawer\.scope/);
  assert.match(reviewDrawer, /selectedVersion\.version_no/);
  assert.match(reviewDrawer, /review-workflow-card/);
  assert.match(reviewDrawer, /review-drawer-scroll/);
  assert.match(reviewDrawer, /<footer>/);
  assert.match(reviewDrawer, /copy\.drawer\.confirm/);
  assert.match(reviewDrawer, /copy\.drawer\.cancel/);
});

test("review running state keeps editor and Run entry points without final issue statistics", () => {
  const runningState = reviewWorkspace.slice(reviewWorkspace.indexOf("function RunningState"), reviewWorkspace.indexOf("function FailureState"));
  assert.match(reviewWorkspace, /copy\.report\.openEditor/);
  assert.match(reviewWorkspace, /href=\{`\/projects\/\$\{projectId\}\/works\/\$\{workId\}`\}/);
  assert.match(reviewWorkspace, /Run ID: \{run\.runNumber\}/);
  assert.match(reviewWorkspace, /href=\{`\/workflow-runs\/\$\{run\.id\}`\}/);
  assert.match(reviewWorkspace, /run\.startedAt \?\? run\.createdAt/);
  assert.match(runningState, /review-progress-card/);
  assert.doesNotMatch(runningState, /review-report-summary/);
});

test("review result summary uses real issue severities and category labels before the issue entry list", () => {
  assert.match(reviewWorkspace, /const counts = detail\.issues\.reduce/);
  assert.match(reviewWorkspace, /const categoryCounts = detail\.issues\.reduce/);
  assert.match(reviewWorkspace, /issue\.categoryLabel/);
  assert.match(reviewWorkspace, /review-category-summary/);
  assert.match(reviewWorkspace, /copy\.report\.categorySummary/);
  assert.match(reviewWorkspace, /review-issue-list/);
  assert.match(reviewWorkspace, /realReviewConclusionLabel\(detail\.report\.conclusion\)/);
  assert.match(reviewWorkspace, /detail\.report\.passedRuleCount/);
});

test("issue detail and source view use real metadata and location fallback without editing the source", () => {
  assert.match(reviewWorkspace, /issue\.position/);
  assert.match(reviewWorkspace, /issue\.description/);
  assert.match(reviewWorkspace, /issue\.suggestion/);
  assert.match(reviewWorkspace, /reviewLocationLabel\(issue\.location\)/);
  assert.match(reviewWorkspace, /function IssueSourceView/);
  assert.match(reviewWorkspace, /className=\{paragraph\.highlighted \? "highlighted" : ""\}/);
  assert.match(reviewWorkspace, /function HighlightedText/);
  assert.match(reviewWorkspace, /copy\.report\.locationFallback/);
  assert.doesNotMatch(reviewWorkspace, /setContent\(.*source/);
});

test("review failure states keep source and Run context, separate recovery semantics, and never render a report", () => {
  const failureState = reviewWorkspace.slice(
    reviewWorkspace.indexOf("function FailureState"),
    reviewWorkspace.indexOf("function RealReportView"),
  );
  assert.match(failureState, /content\.current_version\.version_no/);
  assert.match(failureState, /run\.runNumber/);
  assert.match(failureState, /copy\.states\.validationRetry/);
  assert.match(failureState, /copy\.states\.errorCode/);
  assert.match(reviewWorkspace, /retryReviewResultConsumption/);
  assert.match(reviewWorkspace, /summary\.state === "runtime_failed"/);
  assert.match(reviewWorkspace, /summary\.state === "output_validation_failed"/);
  assert.match(reviewWorkspace, /summary\.state === "result_consumption_failed"/);
  assert.doesNotMatch(failureState, /review-report-summary/);
});

test("unconfigured review is a blocked business state with workflow repair guidance", () => {
  const notConfigured = reviewWorkspace.slice(
    reviewWorkspace.indexOf('if (summary.state === "not_configured")'),
    reviewWorkspace.indexOf('if (summary.state === "queued"'),
  );
  assert.match(notConfigured, /review-not-configured-grid/);
  assert.match(notConfigured, /copy\.states\.notConfiguredTitle/);
  assert.match(notConfigured, /copy\.states\.notConfiguredDescription/);
  assert.match(notConfigured, /copy\.states\.notConfiguredStepBind/);
  assert.match(notConfigured, /copy\.states\.notConfiguredStepConnection/);
  assert.match(notConfigured, /copy\.states\.notConfiguredStepReturn/);
  assert.match(notConfigured, /tab=workflow-bindings/);
  assert.match(notConfigured, /review-not-configured-statuses/);
  assert.match(notConfigured, /review-status-explanation/);
  assert.doesNotMatch(notConfigured, /copy\.common\.start/);
  assert.match(
    reviewWorkspace,
    /<button type="button" onClick=\{onStart\} disabled=\{!canStart\}/,
  );
});

test("review history stays real, chronological, filterable, and separates report from run entry points", () => {
  assert.match(reviewHistory, /listContentReviewHistory\(/);
  assert.match(reviewHistory, /sortHistoryItems/);
  assert.match(reviewHistory, /review-history-filters/);
  assert.match(reviewHistory, /statusFilter/);
  assert.match(reviewHistory, /versionFilter/);
  assert.match(reviewHistory, /searchQuery/);
  assert.match(reviewHistory, /reviewStateLabel\("runtime_failed"\)/);
  assert.match(reviewHistory, /item\.reportSummary\?\.id/);
  assert.match(reviewHistory, /run\.runNumber/);
  assert.match(reviewHistory, /copy\.history\.viewReport/);
  assert.match(reviewHistory, /copy\.history\.viewFailure/);
  assert.match(reviewHistory, /copy\.history\.viewProgress/);
  assert.doesNotMatch(reviewHistory, /setSelected\(.*MockReviewReport/);
});

test("review report rewrite entry follows selected open issues into the existing rewrite route", () => {
  assert.match(reviewWorkspace, /selectedRewriteIssueIds/);
  assert.match(reviewWorkspace, /toggleRewriteIssue/);
  assert.match(reviewWorkspace, /disabled=\{!selectedRewriteIssueIds\.length\}/);
  assert.match(reviewWorkspace, /issueIds=\$\{encodeURIComponent\(selectedRewriteIssueIds\.join\(\",\"\)\)\}/);
  assert.match(reviewWorkspace, /issue\.disposition !== "open"/);
  assert.match(reviewWorkspace, /copy\.report\.rewriteSelectionRequired/);
  assert.match(rewriteRoute, /issueIds\?: string/);
  assert.match(rewriteRoute, /issueIds=\{issueIds\}/);
  assert.match(rewriteWorkspace, /initialIssueIds/);
  assert.match(rewriteWorkspace, /initialIssueIds\.includes\(id\)/);
});

test("rewrite creation keeps report, selected issues, target source, and required-input blocking visible", () => {
  assert.match(rewriteWorkspace, /RewriteCreateView/);
  assert.match(rewriteWorkspace, /reviewDetail/);
  assert.match(rewriteWorkspace, /sourceContent/);
  assert.match(rewriteWorkspace, /selectedIssues/);
  assert.match(rewriteWorkspace, /sourceContentVersionSummary/);
  assert.match(rewriteWorkspace, /reviewDetail\?\.report\.summary/);
  assert.match(rewriteWorkspace, /if \(!availability\)/);
  assert.match(rewriteWorkspace, /disabled=\{checking \|\| !selected\.length\}/);
  assert.match(rewriteWorkspace, /setSelected\(initialIssueIds \? openIssueIds\.filter/);
  assert.match(rewriteWorkspace, /: \[\]\);/);
});

test("rewrite configuration drawer is a read-only summary with real binding status and settings escape hatch", () => {
  assert.match(rewriteWorkspace, /RewriteConfigurationDrawer/);
  assert.match(rewriteWorkspace, /configurationSummary/);
  assert.match(rewriteWorkspace, /bindingVersion/);
  assert.match(rewriteWorkspace, /connectionId/);
  assert.match(rewriteWorkspace, /inputContract/);
  assert.match(rewriteWorkspace, /outputContract/);
  assert.match(rewriteWorkspace, /role="status"/);
  assert.match(rewriteWorkspace, /rewrite-config-drawer-note/);
  assert.match(rewriteWorkspace, /\/projects\/\$\{projectId\}\/settings/);
  const configDrawerSource = rewriteWorkspace.slice(rewriteWorkspace.indexOf("function RewriteConfigurationDrawer"), rewriteWorkspace.indexOf("function RewriteCreateView"));
  assert.doesNotMatch(configDrawerSource, /onChange=/);
});

test("rewrite running state keeps real Run context, source issues, waiting result, and no premature candidate", () => {
  assert.match(rewriteWorkspace, /RewriteRunningState/);
  assert.match(rewriteWorkspace, /active\(summary\.state\)/);
  assert.match(rewriteWorkspace, /run\?\.runNumber/);
  assert.match(rewriteWorkspace, /summary\.selectedIssueSummary\?\.items/);
  assert.match(rewriteWorkspace, /formatWorkflowRunTime/);
  assert.match(rewriteWorkspace, /等待重写结果/);
  assert.match(rewriteWorkspace, /workflow-runs/);
  assert.match(rewriteWorkspace, /candidate_ready/);
  assert.match(rewriteWorkspace, /rewrite-running-banner/);
});

test("rewrite result panel keeps candidate, source issues, summary, and explicit current-version confirmation together", () => {
  assert.match(rewriteResultPanel, /getContentRewriteResult\(runId/);
  assert.match(rewriteResultPanel, /workflowRun\.runNumber/);
  assert.match(rewriteResultPanel, /selectedIssueSummary\.total/);
  assert.match(rewriteResultPanel, /rewrite-result-title/);
  assert.match(rewriteResultPanel, /rewrite-result-version-flow/);
  assert.match(rewriteResultPanel, /setCurrentContentVersion/);
  assert.match(rewriteResultPanel, /candidateIsCurrent/);
  assert.match(rewriteResultPanel, /role="dialog"/);
  assert.doesNotMatch(rewriteResultPanel, /createContentRewriteRun|preflightContentRewrite/);
});

test("summary polling is isolated from the editable draft refresh", () => {
  assert.match(editor, /\["queued", "running"\]/);
  assert.match(editor, /refreshGenerationSummary/);
  assert.doesNotMatch(editor, /const refreshGenerationSummary[\s\S]*?getContentItem[\s\S]*?const refreshWorkspace/);
  assert.match(editor, /const refreshWorkspace[\s\S]*?getContentItem/);
});

test("failure recovery and candidate presentation retain their frozen API boundaries", () => {
  assert.match(status, /retryContentGenerationResultConsumption/);
  assert.match(status, /retryWorkflowRun/);
  assert.match(candidate, /setCurrentContentVersion/);
  assert.match(candidate, /candidateCanBecomeCurrent/);
  assert.match(candidate, /candidate_source_stale/);
  assert.doesNotMatch(candidate, /force|override/);
});

test("drawer, status, and candidate visible business copy use the feature locale", () => {
  for (const source of [drawer, status, candidate]) assert.match(source, /content-generation-locale/);
  for (const key of ["title", "contextOptionLabels", "preflightPassed", "conditionsChanged", "confirmCreate"]) assert.match(drawer, new RegExp(`copy\\.drawer\\.${key}`));
  assert.match(locale, /drawer:/);
});
