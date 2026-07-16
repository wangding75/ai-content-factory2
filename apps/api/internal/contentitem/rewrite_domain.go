package contentitem

import (
	"bytes"
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	ContentVersionSourceManualCreated = "manual_created"
	ContentVersionSourceMockGenerated = "mock_generated"
	ContentVersionSourceMockRewrite   = "mock_rewrite"

	ContentVersionStatusEditableDraft = "editable_draft"
	ContentVersionStatusFrozen        = "frozen"

	WorkflowProviderMock       = "mock"
	WorkflowKeyMockGenerate    = "content_mock_generate"
	WorkflowKeyMockReview      = "content_mock_review"
	WorkflowKeyMockRewrite     = "content_mock_rewrite"
	WorkflowRunStatusRunning   = "running"
	WorkflowRunStatusSucceeded = "succeeded"
	WorkflowRunStatusFailed    = "failed"

	ContentVersionListOrder = "version_no DESC, id DESC"
)

// ContentVersionLineage is a read relationship assembled from the existing
// ContentVersion, ReviewReport, and WorkflowRun records. It stores no second
// current-version state: callers derive currentness from ContentItem.CurrentVersionID.
type ContentVersionLineage struct {
	SourceContentVersion *ContentVersion
	SourceReviewReport   *ReviewReport
	RewriteWorkflowRun   *WorkflowRun
}

// Empty is the v1-compatible shape: Iteration 06 versions have no rewrite lineage.
func (v ContentVersionLineage) Empty() bool {
	return v.SourceContentVersion == nil && v.SourceReviewReport == nil && v.RewriteWorkflowRun == nil
}

// ValidateRewriteShape validates only the frozen Iteration 07 addition and
// deliberately leaves existing Iteration 06 source/status combinations intact.
func (v ContentVersion) ValidateRewriteShape() error {
	switch v.Source {
	case ContentVersionSourceManualCreated, ContentVersionSourceMockGenerated:
		return nil
	case ContentVersionSourceMockRewrite:
		if v.VersionNo == 2 && v.Status == ContentVersionStatusEditableDraft {
			return nil
		}
	}
	return ErrInvalidContentVersion
}

// ValidateRewriteShape keeps the domain contract aligned with Migration 000007
// and the frozen WorkflowRun detail without performing persistence work.
func (r WorkflowRun) ValidateRewriteShape() error {
	switch r.WorkflowKey {
	case WorkflowKeyMockGenerate, WorkflowKeyMockReview:
		return nil
	case WorkflowKeyMockRewrite:
	default:
		return ErrInvalidMockRewriteRun
	}
	if r.WorkflowKey != WorkflowKeyMockRewrite {
		return nil
	}
	if r.ProviderKey != WorkflowProviderMock || r.ContentItemID == uuid.Nil || r.ContentVersionID == uuid.Nil || r.SourceReviewReportID == nil {
		return ErrInvalidMockRewriteRun
	}
	switch r.Status {
	case WorkflowRunStatusRunning:
		if r.TargetContentVersionID == nil && r.FinishedAt == nil {
			return nil
		}
	case WorkflowRunStatusSucceeded:
		if r.TargetContentVersionID != nil && r.FinishedAt != nil && r.ErrorCode == nil && r.ErrorSummary == nil && !emptyJSONObject(r.OutputJSON) {
			return nil
		}
	case WorkflowRunStatusFailed:
		if r.TargetContentVersionID == nil && r.FinishedAt != nil && r.ErrorCode != nil && r.ErrorSummary != nil && emptyJSONObject(r.OutputJSON) && safeFailure(*r.ErrorCode, *r.ErrorSummary) {
			return nil
		}
	}
	return ErrInvalidMockRewriteRun
}

func emptyJSONObject(value []byte) bool {
	return len(bytes.TrimSpace(value)) == 0 || strings.EqualFold(string(bytes.TrimSpace(value)), "{}")
}

type ContentVersionPage struct {
	Items                []ContentVersion
	Total, Limit, Offset int
}

type ContentVersionPageOptions struct {
	Limit, Offset int
}

// ContentVersionRepository is the narrow contract for the frozen version
// history. pgx.Tx is the repository package's existing transaction abstraction.
type ContentVersionRepository interface {
	GetContentVersion(context.Context, uuid.UUID) (ContentVersion, error)
	ListContentVersions(context.Context, uuid.UUID, ContentVersionPageOptions) (ContentVersionPage, error)
	CountContentVersions(context.Context, uuid.UUID) (int, error)
	CreateContentVersion(context.Context, pgx.Tx, ContentVersion) (ContentVersion, error)
	GetContentVersionByNumber(context.Context, uuid.UUID, int) (ContentVersion, error)
}

// WorkflowRunRepository is the narrow contract for rewrite attempts. The
// application layer will compose its write methods in one existing transaction.
type WorkflowRunRepository interface {
	GetWorkflowRun(context.Context, uuid.UUID) (WorkflowRun, error)
	CreateMockRewriteRunning(context.Context, pgx.Tx, WorkflowRun) (WorkflowRun, error)
	MarkMockRewriteSucceeded(context.Context, pgx.Tx, uuid.UUID, uuid.UUID, []byte, time.Time) (WorkflowRun, error)
	MarkMockRewriteFailed(context.Context, pgx.Tx, uuid.UUID, string, string, time.Time) (WorkflowRun, error)
	FindMockRewriteByIdempotencyKey(context.Context, uuid.UUID, string) (WorkflowRun, error)
	FindMockRewriteByFingerprint(context.Context, uuid.UUID, string) (WorkflowRun, error)
	LatestMockRewrite(context.Context, uuid.UUID) (WorkflowRun, error)
}
