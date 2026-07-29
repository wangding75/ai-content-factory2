import { apiRequest, type ApiRequestInit } from "../../lib/api.ts";
import type {
  WorkflowRunDto,
  WorkflowRunEventDto,
} from "../workflow-runs/workflow-run-api.ts";
export type {
  WorkflowRunDto,
  WorkflowRunEventDto,
} from "../workflow-runs/workflow-run-api.ts";
import type {
  ContentVersion,
  ReviewDetail as MockReviewDetail,
  ReviewReport as MockReviewReport,
} from "../content-items/content-item-http-api.ts";

export type ReviewState =
  | "idle"
  | "not_configured"
  | "queued"
  | "running"
  | "review_ready"
  | "runtime_failed"
  | "output_validation_failed"
  | "result_consumption_failed";

export type ReviewDimension =
  | "compliance"
  | "factual_consistency"
  | "language_quality"
  | "structural_logic"
  | "character_consistency";

export type ReviewSeverity = "critical" | "warning" | "suggestion";
export type ReviewDisposition = "open" | "ignored";

export interface ReviewSourceVersionSummary {
  id: string;
  contentItemId: string;
  versionNo: number;
  version: number;
  title: string;
  wordCount: number;
  contentHash: string;
  frozen: boolean;
}

export interface ReviewConfigurationSummary {
  bindingVersion: number;
  workflowConfigurationId: string;
  workflowConfigurationName: string;
  workflowConfigurationVersion: number;
  connectionId: string;
  connectionVersion: number;
  inputContract: "review.input.v1";
  outputContract: "review.output.v1";
}

export interface ReviewPreflightCheck {
  code:
    | "source_version_saved"
    | "source_version_unchanged"
    | "project_binding_available"
    | "workflow_configuration_available"
    | "workflow_connection_available"
    | "active_review_run_absent"
    | "review_input_valid";
  status: "passed" | "warning" | "blocked";
  message: string;
}

export interface ReviewPreflightReport {
  status: "passed" | "blocked";
  checks: ReviewPreflightCheck[];
  sourceContentVersionSummary: ReviewSourceVersionSummary;
  reviewDimensions: ReviewDimension[];
  configurationSummary: ReviewConfigurationSummary | null;
  preflightToken: string | null;
  expiresAt: string | null;
}

export interface ReviewReportSummary {
  id: string;
  workflowRunId: string;
  sourceContentVersionId: string;
  conclusion: "passed" | "needs_changes";
  summary: string;
  completedAt: string;
}

export interface ReviewIssueSummary {
  total: number;
  critical: number;
  warning: number;
  suggestion: number;
  open: number;
  ignored: number;
}

export interface ReviewSafeError {
  code:
    | "runtime_failed"
    | "output_validation_failed"
    | "result_consumption_failed";
  message: string;
  correlationId: string;
  attemptCount: number;
  occurredAt: string;
}

export interface ContentReviewSummary {
  contentItemId: string;
  state: ReviewState;
  canStartReview: boolean;
  activeRun: WorkflowRunDto | null;
  latestRun: WorkflowRunDto | null;
  latestReport: ReviewReportSummary | null;
  issueSummary: ReviewIssueSummary | null;
  latestError: ReviewSafeError | null;
  configurationSummary: ReviewConfigurationSummary | null;
}

export interface ReviewEvidence {
  quote: string | null;
  sourceRefs: string[];
}

export interface ReviewLocation {
  paragraphStart: number;
  paragraphEnd: number;
  sentenceStart: number;
  sentenceEnd: number;
}

export interface RealReviewReport {
  id: string;
  contentItemId: string;
  sourceContentVersionId: string;
  sourceContentVersionVersion: number;
  sourceContentHash: string;
  workflowRunId: string;
  schemaVersion: "review.output.v1";
  conclusion: "passed" | "needs_changes";
  summary: string;
  passedRuleCount: number;
  createdAt: string;
  completedAt: string;
}

export interface RealReviewIssue {
  id: string;
  reviewId: string;
  issueKey: string;
  position: number;
  categoryKey: string;
  categoryLabel: string;
  severity: ReviewSeverity;
  title: string;
  description: string;
  evidence: ReviewEvidence;
  location: ReviewLocation | null;
  suggestion: string | null;
  disposition: ReviewDisposition;
  version: number;
  ignoredAt: string | null;
  ignoredBy: string | null;
  createdAt: string;
  updatedAt: string;
}

export interface RealReviewRecommendation {
  id: string;
  reviewId: string;
  position: number;
  priority: "low" | "medium" | "high";
  title: string;
  description: string;
  createdAt: string;
}

export interface RealReviewDetail {
  report: RealReviewReport;
  sourceContentVersionSummary: ReviewSourceVersionSummary;
  issues: RealReviewIssue[];
  recommendations: RealReviewRecommendation[];
  workflowRunSummary: WorkflowRunDto;
}

export interface ReviewHistoryItem {
  workflowRun: WorkflowRunDto;
  sourceContentVersionSummary: ReviewSourceVersionSummary;
  reportSummary: ReviewReportSummary | null;
  state: ReviewState;
  latestError: ReviewSafeError | null;
}

export type ReviewHistoryEntry = ReviewHistoryItem | MockReviewReport;

export interface ReviewHistoryPage {
  items: ReviewHistoryEntry[];
  total: number;
  limit: number;
  offset: number;
}

export interface ReviewableContentVersion {
  id: string;
  content_item_id: string;
  version_no: number;
  status: "editable_draft" | "frozen";
  title: string;
  word_count: number;
  created_at: string;
  frozen_at: string | null;
  is_current: boolean;
}

export interface RealReviewResult {
  report: RealReviewReport;
  issues: RealReviewIssue[];
  recommendations: RealReviewRecommendation[];
  workflowRun: WorkflowRunDto;
}

export type ReviewDetail = RealReviewDetail | MockReviewDetail;

const versionPath = (contentVersionId: string) =>
  `/content-versions/${encodeURIComponent(contentVersionId)}`;
const itemPath = (contentItemId: string) =>
  `/content-items/${encodeURIComponent(contentItemId)}`;
const reviewPath = (reviewId: string) =>
  `/reviews/${encodeURIComponent(reviewId)}`;

export const preflightContentReview = (
  contentVersionId: string,
  payload: {
    sourceContentVersionVersion: number;
    optionalInstructions: string | null;
  },
  init?: ApiRequestInit,
) =>
  apiRequest<ReviewPreflightReport>(
    `${versionPath(contentVersionId)}/review-runs/preflight`,
    {
      ...init,
      method: "POST",
      headers: { ...init?.headers, "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    },
  );

export const createContentReviewRun = (
  contentVersionId: string,
  preflightToken: string,
  idempotencyKey: string,
  init?: ApiRequestInit,
) =>
  apiRequest<WorkflowRunDto>(`${versionPath(contentVersionId)}/review-runs`, {
    ...init,
    method: "POST",
    headers: {
      ...init?.headers,
      "Content-Type": "application/json",
      "Idempotency-Key": idempotencyKey,
    },
    body: JSON.stringify({ preflightToken }),
  });

export const getContentReviewSummary = (
  contentItemId: string,
  init?: ApiRequestInit,
) =>
  apiRequest<ContentReviewSummary>(
    `${itemPath(contentItemId)}/review-summary`,
    init,
  );

export const listContentReviewHistory = (
  contentItemId: string,
  options: { limit?: number; offset?: number } = {},
  init?: ApiRequestInit,
) => {
  const query = new URLSearchParams({
    limit: String(options.limit ?? 20),
    offset: String(options.offset ?? 0),
  });
  return apiRequest<ReviewHistoryPage>(
    `${itemPath(contentItemId)}/reviews?${query}`,
    init,
  );
};

export const getReview = (reviewId: string, init?: ApiRequestInit) =>
  apiRequest<ReviewDetail>(reviewPath(reviewId), init);

export const updateReviewIssue = (
  reviewId: string,
  issueId: string,
  payload: { disposition: ReviewDisposition; expectedVersion: number },
  idempotencyKey: string,
  init?: ApiRequestInit,
) =>
  apiRequest<RealReviewIssue>(
    `${reviewPath(reviewId)}/issues/${encodeURIComponent(issueId)}`,
    {
      ...init,
      method: "PATCH",
      headers: {
        ...init?.headers,
        "Content-Type": "application/json",
        "Idempotency-Key": idempotencyKey,
      },
      body: JSON.stringify(payload),
    },
  );

export const retryReviewResultConsumption = (
  workflowRunId: string,
  expectedRunVersion: number,
  idempotencyKey: string,
  init?: ApiRequestInit,
) =>
  apiRequest<RealReviewResult>(
    `/workflow-runs/${encodeURIComponent(workflowRunId)}/review-result-consumption-retries`,
    {
      ...init,
      method: "POST",
      headers: {
        ...init?.headers,
        "Content-Type": "application/json",
        "Idempotency-Key": idempotencyKey,
      },
      body: JSON.stringify({ expectedRunVersion }),
    },
  );

export const getReviewSourceVersion = (
  contentVersionId: string,
  init?: ApiRequestInit,
) =>
  apiRequest<{
    content_version: ContentVersion;
    source_workflow_run?: { runNumber: string | null } | null;
    is_current: boolean;
  }>(versionPath(contentVersionId), init);

export const listReviewableContentVersions = (
  contentItemId: string,
  init?: ApiRequestInit,
) =>
  apiRequest<{
    items: ReviewableContentVersion[];
    total: number;
    limit: number;
    offset: number;
  }>(`${itemPath(contentItemId)}/versions?limit=100&offset=0`, init);

export const getReviewRunEvents = (
  workflowRunId: string,
  init?: ApiRequestInit,
) =>
  apiRequest<{ items: WorkflowRunEventDto[] }>(
    `/workflow-runs/${encodeURIComponent(workflowRunId)}/events`,
    init,
  );

export const isRealReviewDetail = (
  detail: ReviewDetail,
): detail is RealReviewDetail => "report" in detail;

export const isRealReviewHistoryItem = (
  item: ReviewHistoryEntry,
): item is ReviewHistoryItem => "workflowRun" in item;
