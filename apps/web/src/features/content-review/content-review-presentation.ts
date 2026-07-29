import type {
  RealReviewIssue,
  ReviewDimension,
  ReviewLocation,
  ReviewSeverity,
  ReviewState,
} from "./content-review-api.ts";
import { reviewCopy } from "./content-review-locale.ts";

export const reviewStateLabel = (value: ReviewState) =>
  reviewCopy.labels.states[value];
export const reviewDimensionLabel = (value: ReviewDimension) =>
  reviewCopy.labels.dimensions[value] ?? reviewCopy.labels.unknownDimension;
export const realReviewSeverityLabel = (value: ReviewSeverity) =>
  reviewCopy.labels.severities[value] ?? reviewCopy.labels.unknownSeverity;
export const realReviewConclusionLabel = (value: string) =>
  value === "passed"
    ? reviewCopy.labels.conclusions.passed
    : value === "needs_changes"
      ? reviewCopy.labels.conclusions.needs_changes
      : reviewCopy.labels.conclusions.unknown;
export const reviewDispositionLabel = (value: string) =>
  value === "ignored"
    ? reviewCopy.labels.dispositions.ignored
    : reviewCopy.labels.dispositions.open;

export const formatReviewTime = (value: string | null | undefined) => {
  if (!value) return reviewCopy.labels.timeMissing;
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return reviewCopy.labels.timeMissing;
  return new Intl.DateTimeFormat("zh-CN", {
    dateStyle: "medium",
    timeStyle: "short",
    hour12: false,
    timeZone: "Asia/Shanghai",
  }).format(date);
};

export const reviewLocationLabel = (location: ReviewLocation | null) =>
  location
    ? `${reviewCopy.labels.locationPrefix} ${location.paragraphStart}${
        location.paragraphEnd !== location.paragraphStart
          ? `–${location.paragraphEnd}`
          : ""
      } ${reviewCopy.labels.paragraph}`
    : reviewCopy.labels.locationMissing;

export interface LocatedParagraph {
  text: string;
  paragraph: number;
  highlighted: boolean;
  exactQuote: string | null;
}

export function locateIssueInSource(
  content: string,
  issue: Pick<RealReviewIssue, "location" | "evidence">,
): { paragraphs: LocatedParagraph[]; exact: boolean } {
  const paragraphs = content
    .split(/\n\s*\n/)
    .map((text) => text.trim())
    .filter(Boolean);
  const quote = issue.evidence.quote?.trim() || null;
  const byLocation = issue.location
    ? paragraphs.map((text, index) => ({
        text,
        paragraph: index + 1,
        highlighted:
          index + 1 >= issue.location!.paragraphStart &&
          index + 1 <= issue.location!.paragraphEnd,
        exactQuote: quote && text.includes(quote) ? quote : null,
      }))
    : paragraphs.map((text, index) => ({
        text,
        paragraph: index + 1,
        highlighted: false,
        exactQuote: quote && text.includes(quote) ? quote : null,
      }));
  const quoteMatched = !!quote && byLocation.some((item) => item.exactQuote);
  const locationMatched =
    !!issue.location && byLocation.some((item) => item.highlighted);
  if (quoteMatched || locationMatched)
    return { paragraphs: byLocation, exact: quoteMatched };
  return {
    paragraphs: [
      {
        text: quote || paragraphs.slice(0, 2).join("\n\n"),
        paragraph: issue.location?.paragraphStart ?? 1,
        highlighted: true,
        exactQuote: quote,
      },
    ],
    exact: false,
  };
}

export function safeReviewError(cause: unknown, fallback: string) {
  const error = cause as { status?: number; code?: string };
  if (error?.code === "review_not_configured")
    return reviewCopy.errors.notConfigured;
  if (
    error?.code === "preflight_token_expired" ||
    error?.code === "preflight_input_changed" ||
    error?.code === "source_content_version_changed"
  )
    return reviewCopy.errors.conditionsChanged;
  if (error?.code === "active_review_conflict")
    return reviewCopy.errors.activeReview;
  if (error?.code === "content_version_not_found")
    return reviewCopy.errors.contentVersionMissing;
  if (error?.code === "review_not_found")
    return reviewCopy.errors.reviewMissing;
  if (error?.code === "review_issue_not_found")
    return reviewCopy.errors.issueMissing;
  if (error?.code === "workflow_run_not_found")
    return reviewCopy.errors.runMissing;
  if (error?.code === "review_issue_version_conflict")
    return reviewCopy.errors.issueConflict;
  if (error?.code === "workflow_run_version_conflict")
    return reviewCopy.errors.runConflict;
  if (error?.status === 404) return reviewCopy.errors.unavailableRecord;
  if (error?.status === 409) return reviewCopy.errors.stateChanged;
  return fallback;
}
