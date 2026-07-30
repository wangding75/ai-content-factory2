import { apiRequest, type ApiRequestInit } from "@/lib/api";
import type { WorkflowRunDto } from "@/features/workflow-runs/workflow-run-api";
import type { ContentVersion } from "@/features/content-items/content-item-http-api";

export type RewriteStrategy = "targeted_fix" | "creative_rewrite";
export type RewriteOptions = { strategy: RewriteStrategy };
export type RewriteIssue = { id: string; reviewId: string; issueKey: string; position: number; categoryLabel: string; severity: string; title: string; description: string; disposition: "open" | "ignored"; version: number };
export type RewriteSource = { id: string; contentItemId: string; versionNo: number; version: number; title: string; wordCount: number; contentHash: string };
export type RewriteConfiguration = { bindingVersion: number; workflowConfigurationId: string; workflowConfigurationName: string; workflowConfigurationVersion: number; connectionId: string; connectionVersion: number; inputContract: "rewrite.input.v1"; outputContract: "rewrite.output.v1" };
export type RewriteAvailability = { reviewReportId: string; contentItemId: string; sourceContentVersionSummary: RewriteSource; available: boolean; reason: "review_not_completed" | "no_open_issues" | "rewrite_not_configured" | "active_rewrite_run_conflict" | null; openIssueCount: number; activeRun: WorkflowRunDto | null; configurationSummary: RewriteConfiguration | null };
export type RewriteCheck = { code: string; status: "passed" | "blocked"; message: string };
export type RewriteReportSnapshot = { reviewReportId: string; sourceContentVersionId: string; sourceContentVersionVersion: number; sourceContentHash: string; conclusion: string; summary: string; completedAt: string };
export type RewritePreflight = { status: "passed" | "blocked"; checks: RewriteCheck[]; reviewReportSnapshot: RewriteReportSnapshot; sourceContentVersionSummary: RewriteSource; selectedIssueSummary: { total: number; items: Array<{ reviewIssueId: string; issueKey: string; position: number; title: string; severity: string }> }; rewriteOptions: RewriteOptions; optionalInstructions: string | null; configurationSummary: RewriteConfiguration | null; preflightToken: string | null; expiresAt: string | null };
export type RewriteState = "idle" | "not_configured" | "queued" | "running" | "candidate_ready" | "runtime_failed" | "output_validation_failed" | "result_consumption_failed";
export type RewriteSafeError = { code: "runtime_failed" | "output_validation_failed" | "result_consumption_failed"; message: string; correlationId: string; occurredAt: string };
export type RewriteIssueOutcome = { reviewIssueId: string; summary: string };
export type RewriteCandidate = ContentVersion & { contentItemId?: string; versionNo?: number; wordCount?: number; source_content_version_id?: string; source_content_version_version?: number; source_workflow_run_id?: string };
export type RewriteSummary = { reviewReportId: string; contentItemId: string; state: RewriteState; canStartRewrite: boolean; activeRun: WorkflowRunDto | null; latestRun: WorkflowRunDto | null; sourceContentVersionSummary: RewriteSource; selectedIssueSummary: RewritePreflight["selectedIssueSummary"] | null; candidateVersion: RewriteCandidate | null; candidateIsCurrent: boolean; canSetCurrent: boolean; latestError: RewriteSafeError | null; configurationSummary: RewriteConfiguration | null };
export type RewriteResult = { reviewReportSnapshot: RewriteReportSnapshot; sourceContentVersionSummary: RewriteSource; selectedIssueSummary: NonNullable<RewriteSummary["selectedIssueSummary"]>; workflowRun: WorkflowRunDto; output: { title: string; content: string; summary: string; addressedIssues: RewriteIssueOutcome[]; unresolvedIssues: RewriteIssueOutcome[]; warnings: string[]; metadata: { changeSummary: string | null } | null }; candidateVersion: RewriteCandidate; candidateIsCurrent: boolean; canSetCurrent: boolean };
export type RewriteHistoryItem = { reviewReportSnapshot: RewriteResult["reviewReportSnapshot"]; sourceContentVersionSummary: RewriteSource; workflowRun: WorkflowRunDto; state: RewriteState; candidateVersion: RewriteCandidate | null; candidateIsCurrent: boolean; latestError: RewriteSafeError | null };
export type RewriteHistoryPage = { items: RewriteHistoryItem[]; total: number; limit: number; offset: number };

const reviewPath = (reviewId: string) => `/reviews/${encodeURIComponent(reviewId)}`;
export const getContentRewriteAvailability = (reviewId: string, init?: ApiRequestInit) => apiRequest<RewriteAvailability>(`${reviewPath(reviewId)}/rewrite-availability`, init);
export const preflightContentRewrite = (reviewId: string, body: { selectedIssueIds: string[]; optionalInstructions: string | null; rewriteOptions: RewriteOptions }, init?: ApiRequestInit) => apiRequest<RewritePreflight>(`${reviewPath(reviewId)}/rewrites/preflight`, { ...init, method: "POST", headers: { ...init?.headers, "Content-Type": "application/json" }, body: JSON.stringify(body) });
export const createContentRewriteRun = (reviewId: string, preflightToken: string, key: string, init?: ApiRequestInit) => apiRequest<WorkflowRunDto>(`${reviewPath(reviewId)}/rewrites`, { ...init, method: "POST", headers: { ...init?.headers, "Content-Type": "application/json", "Idempotency-Key": key }, body: JSON.stringify({ preflightToken }) });
export const getContentRewriteSummary = (reviewId: string, init?: ApiRequestInit) => apiRequest<RewriteSummary>(`${reviewPath(reviewId)}/rewrite-summary`, init);
export const listContentRewriteHistory = (contentItemId: string, options: { limit?: number; offset?: number } = {}, init?: ApiRequestInit) => {
  const query = new URLSearchParams({ limit: String(options.limit ?? 20), offset: String(options.offset ?? 0) });
  return apiRequest<RewriteHistoryPage>(`/content-items/${encodeURIComponent(contentItemId)}/rewrite-history?${query}`, init);
};
export const getContentRewriteResult = (runId: string, init?: ApiRequestInit) => apiRequest<RewriteResult>(`/workflow-runs/${encodeURIComponent(runId)}/rewrite-result`, init);
export const retryContentRewriteResultConsumption = (runId: string, expectedRunVersion: number, key: string, init?: ApiRequestInit) => apiRequest<RewriteResult>(`/workflow-runs/${encodeURIComponent(runId)}/rewrite-result-consumption-retries`, { ...init, method: "POST", headers: { ...init?.headers, "Content-Type": "application/json", "Idempotency-Key": key }, body: JSON.stringify({ expectedRunVersion }) });

export const rewriteErrorMessage = (code?: string) => ({
  rewrite_not_configured: "当前项目尚未配置可用的重写工作流。",
  rewrite_not_available: "当前审核结果暂时不能创建重写。",
  rewrite_preflight_expired: "预检已过期，请重新进行预检。",
  rewrite_preflight_stale: "审核数据已变化，请重新进行预检。",
  rewrite_preflight_consumed: "该预检已被使用，请重新进行预检。",
  active_rewrite_run_conflict: "已有重写任务正在执行，已为你恢复该任务。",
  idempotency_conflict: "本次提交状态不一致，请修改内容后重新预检。",
  validation_error: "输入内容不符合要求，请检查后重试。",
  timeout: "请求超时，创建结果暂未确认。请使用原提交重试。",
  network_error: "网络连接异常，创建结果暂未确认。请使用原提交重试。",
}[code ?? ""] ?? "暂时无法完成操作，请稍后重试。");

export const validRewriteInput = (issueIds: string[], instructions: string, options: RewriteOptions) => {
  if (issueIds.length < 1 || issueIds.length > 50) return "请选择 1 至 50 个待处理问题。";
  if (new Set(issueIds).size !== issueIds.length) return "选择的问题不能重复。";
  if ([...instructions].length > 2000) return "补充要求不能超过 2000 个字符。";
  if (options.strategy !== "targeted_fix" && options.strategy !== "creative_rewrite") return "请选择有效的重写策略。";
  return null;
};
