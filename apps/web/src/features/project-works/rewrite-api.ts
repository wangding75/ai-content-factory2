import { apiRequest, type ApiRequestInit } from "@/lib/api";
import type { WorkflowRunDto } from "@/features/workflow-runs/workflow-run-api";

export type RewriteStrategy = "targeted_fix" | "creative_rewrite";
export type RewriteOptions = { strategy: RewriteStrategy };
export type RewriteIssue = { id: string; reviewId: string; issueKey: string; position: number; categoryLabel: string; severity: string; title: string; description: string; disposition: "open" | "ignored"; version: number };
export type RewriteSource = { id: string; contentItemId: string; versionNo: number; version: number; title: string; wordCount: number; contentHash: string };
export type RewriteConfiguration = { workflowConfigurationName: string; inputContract: "rewrite.input.v1"; outputContract: "rewrite.output.v1" };
export type RewriteAvailability = { reviewReportId: string; contentItemId: string; sourceContentVersionSummary: RewriteSource; available: boolean; reason: "review_not_completed" | "no_open_issues" | "rewrite_not_configured" | "active_rewrite_run_conflict" | null; openIssueCount: number; activeRun: WorkflowRunDto | null; configurationSummary: RewriteConfiguration | null };
export type RewriteCheck = { code: string; status: "passed" | "blocked"; message: string };
export type RewritePreflight = { status: "passed" | "blocked"; checks: RewriteCheck[]; reviewReportSnapshot: { id: string; conclusion: string; summary: string; completedAt: string }; sourceContentVersionSummary: RewriteSource; selectedIssueSummary: { total: number; items: Array<{ reviewIssueId: string; issueKey: string; position: number; title: string; severity: string }> }; rewriteOptions: RewriteOptions; optionalInstructions: string | null; configurationSummary: RewriteConfiguration | null; preflightToken: string | null; expiresAt: string | null };

const reviewPath = (reviewId: string) => `/reviews/${encodeURIComponent(reviewId)}`;
export const getContentRewriteAvailability = (reviewId: string, init?: ApiRequestInit) => apiRequest<RewriteAvailability>(`${reviewPath(reviewId)}/rewrite-availability`, init);
export const preflightContentRewrite = (reviewId: string, body: { selectedIssueIds: string[]; optionalInstructions: string | null; rewriteOptions: RewriteOptions }, init?: ApiRequestInit) => apiRequest<RewritePreflight>(`${reviewPath(reviewId)}/rewrites/preflight`, { ...init, method: "POST", headers: { ...init?.headers, "Content-Type": "application/json" }, body: JSON.stringify(body) });
export const createContentRewriteRun = (reviewId: string, preflightToken: string, key: string, init?: ApiRequestInit) => apiRequest<WorkflowRunDto>(`${reviewPath(reviewId)}/rewrites`, { ...init, method: "POST", headers: { ...init?.headers, "Content-Type": "application/json", "Idempotency-Key": key }, body: JSON.stringify({ preflightToken }) });

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
