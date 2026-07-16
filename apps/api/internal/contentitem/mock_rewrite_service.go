package contentitem

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type MockRewriteCommand struct {
	ContentItemID, SourceContentVersionID, ReviewReportID uuid.UUID
	ExpectedVersion                                       int
	IdempotencyKey, BusinessInputID                       string
	Parameters                                            MockRewriteParameters
}

type MockRewriteResult struct {
	SourceContentVersion ContentVersion
	TargetContentVersion *ContentVersion
	WorkflowRun          WorkflowRun
	Reused               bool
}

type mockRewriteStore interface {
	ContentVersionRepository
	WorkflowRunRepository
	GetByID(context.Context, uuid.UUID) (Detail, error)
	GetReview(context.Context, uuid.UUID) (ReviewDetail, error)
}

type RewriteTransactionRunner interface {
	InTransaction(context.Context, func(pgx.Tx) error) error
}

type PgxRewriteTransactionRunner struct{ pool *pgxpool.Pool }

func NewPgxRewriteTransactionRunner(pool *pgxpool.Pool) PgxRewriteTransactionRunner {
	return PgxRewriteTransactionRunner{pool: pool}
}
func (r PgxRewriteTransactionRunner) InTransaction(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type MockRewriteService struct {
	store        mockRewriteStore
	provider     MockRewriteProvider
	transactions RewriteTransactionRunner
}

func NewMockRewriteService(store mockRewriteStore, provider MockRewriteProvider, transactions RewriteTransactionRunner) *MockRewriteService {
	if provider == nil {
		provider = DeterministicMockRewriteProvider{}
	}
	return &MockRewriteService{store: store, provider: provider, transactions: transactions}
}

func MockRewriteFingerprint(c MockRewriteCommand) string {
	b, _ := json.Marshal(struct {
		ContentItemID, SourceContentVersionID, ReviewReportID string
		ExpectedVersion                                       int
		BusinessInputID                                       string
		Focus                                                 []string
		PreserveEnding                                        bool
		Instructions                                          *string
	}{c.ContentItemID.String(), c.SourceContentVersionID.String(), c.ReviewReportID.String(), c.ExpectedVersion, c.BusinessInputID, c.Parameters.RewriteFocus, c.Parameters.PreserveEnding, c.Parameters.Instructions})
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func (s *MockRewriteService) Rewrite(ctx context.Context, c MockRewriteCommand) (out MockRewriteResult, err error) {
	if err = ctx.Err(); err != nil {
		return out, err
	}
	if s.store == nil || s.transactions == nil || c.ContentItemID == uuid.Nil || c.SourceContentVersionID == uuid.Nil || c.ReviewReportID == uuid.Nil || c.ExpectedVersion < 1 || strings.TrimSpace(c.IdempotencyKey) == "" || strings.TrimSpace(c.BusinessInputID) == "" {
		return out, ErrInvalidRewriteParameters
	}
	detail, err := s.store.GetByID(ctx, c.ContentItemID)
	if err != nil {
		return out, mapRewriteServiceError(err)
	}
	source, err := s.store.GetContentVersion(ctx, c.SourceContentVersionID)
	if err != nil {
		return out, mapRewriteServiceError(err)
	}
	reviewDetail, err := s.store.GetReview(ctx, c.ReviewReportID)
	if err != nil {
		return out, mapRewriteServiceError(err)
	}
	if source.ContentItemID != detail.Item.ID || reviewDetail.Review.ProjectID != detail.Item.ProjectID || reviewDetail.Review.ContentItemID != detail.Item.ID {
		return out, ErrCrossProjectRelation
	}
	if source.VersionNo != 1 {
		return out, ErrSourceVersionMismatch
	}
	if source.Status != ContentVersionStatusFrozen {
		return out, ErrContentVersionNotFrozen
	}
	if reviewDetail.Review.Status != "completed" {
		return out, ErrReviewNotCompleted
	}
	if reviewDetail.Review.ContentVersionID != source.ID {
		return out, ErrSourceVersionMismatch
	}
	if source.Version != c.ExpectedVersion {
		return out, ErrVersionConflict
	}
	input := MockRewriteInput{ContentItem: detail.Item, SourceContentVersion: source, SourceReviewReport: reviewDetail.Review, Parameters: c.Parameters, BusinessInputID: c.BusinessInputID}
	if err = validateMockRewriteInput(input); err != nil {
		return out, err
	}
	fingerprint := MockRewriteFingerprint(c)
	if prior, priorErr := s.store.FindMockRewriteByIdempotencyKey(ctx, c.ContentItemID, c.IdempotencyKey); priorErr == nil {
		return s.reused(ctx, source, prior, fingerprint)
	} else if !errors.Is(priorErr, ErrWorkflowRunNotFound) {
		return out, mapRewriteServiceError(priorErr)
	}
	if prior, priorErr := s.store.FindMockRewriteByFingerprint(ctx, c.ContentItemID, fingerprint); priorErr == nil {
		return s.reused(ctx, source, prior, fingerprint)
	} else if !errors.Is(priorErr, ErrWorkflowRunNotFound) {
		return out, mapRewriteServiceError(priorErr)
	}

	var providerFailure bool
	err = s.transactions.InTransaction(ctx, func(tx pgx.Tx) error {
		run, createErr := s.store.CreateMockRewriteRunning(ctx, tx, WorkflowRun{ID: uuid.New(), ProjectID: detail.Item.ProjectID, ContentItemID: detail.Item.ID, ContentVersionID: source.ID, SourceReviewReportID: &reviewDetail.Review.ID, ProviderKey: WorkflowProviderMock, WorkflowKey: WorkflowKeyMockRewrite, SubjectType: "content_item", SubjectID: detail.Item.ID, Status: WorkflowRunStatusRunning, IdempotencyKey: c.IdempotencyKey, RequestFingerprint: fingerprint, InputJSON: rewriteInputJSON(c)})
		if createErr != nil {
			return createErr
		}
		generated, providerErr := s.provider.Rewrite(ctx, input)
		if providerErr != nil {
			failed, markErr := s.store.MarkMockRewriteFailed(ctx, tx, run.ID, "mock_rewrite_failed", "mock rewrite failed", time.Now().UTC())
			if markErr != nil {
				return markErr
			}
			out = MockRewriteResult{SourceContentVersion: source, WorkflowRun: failed}
			providerFailure = true
			return nil
		}
		target, createErr := s.store.CreateContentVersion(ctx, tx, ContentVersion{ID: uuid.New(), ContentItemID: detail.Item.ID, VersionNo: 2, Version: 1, Title: generated.Title, Content: generated.Content, Summary: &generated.Summary, WordCount: generated.WordCount, Source: ContentVersionSourceMockRewrite, Status: ContentVersionStatusEditableDraft, GenerationParameters: generated.OutputSummary})
		if createErr != nil {
			return createErr
		}
		run, createErr = s.store.MarkMockRewriteSucceeded(ctx, tx, run.ID, target.ID, generated.OutputSummary, time.Now().UTC())
		if createErr != nil {
			return createErr
		}
		out = MockRewriteResult{SourceContentVersion: source, TargetContentVersion: &target, WorkflowRun: run}
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrIdempotencyConflict) {
			if prior, readErr := s.store.FindMockRewriteByIdempotencyKey(ctx, c.ContentItemID, c.IdempotencyKey); readErr == nil {
				return s.reused(ctx, source, prior, fingerprint)
			}
		}
		return MockRewriteResult{}, mapRewriteServiceError(err)
	}
	if providerFailure {
		return out, ErrMockRewriteFailed
	}
	return out, nil
}

func (s *MockRewriteService) reused(ctx context.Context, source ContentVersion, run WorkflowRun, fingerprint string) (MockRewriteResult, error) {
	if run.RequestFingerprint != fingerprint {
		return MockRewriteResult{}, ErrIdempotencyConflict
	}
	out := MockRewriteResult{SourceContentVersion: source, WorkflowRun: run, Reused: true}
	if run.Status == WorkflowRunStatusFailed {
		return out, ErrMockRewriteFailed
	}
	if run.Status != WorkflowRunStatusSucceeded || run.TargetContentVersionID == nil {
		return MockRewriteResult{}, ErrInternal
	}
	target, err := s.store.GetContentVersion(ctx, *run.TargetContentVersionID)
	if err != nil {
		return MockRewriteResult{}, mapRewriteServiceError(err)
	}
	out.TargetContentVersion = &target
	return out, nil
}

func rewriteInputJSON(c MockRewriteCommand) []byte {
	b, _ := json.Marshal(struct {
		SourceVersionID, ReviewReportID string
		Focus                           []string
		PreserveEnding                  bool
	}{c.SourceContentVersionID.String(), c.ReviewReportID.String(), c.Parameters.RewriteFocus, c.Parameters.PreserveEnding})
	return b
}
func mapRewriteServiceError(err error) error {
	if err == nil || errors.Is(err, context.Canceled) {
		return err
	}
	for _, known := range []error{ErrContentItemNotFound, ErrContentVersionNotFound, ErrReviewNotFound, ErrContentVersionNotFrozen, ErrReviewNotCompleted, ErrSourceVersionMismatch, ErrVersionConflict, ErrInvalidRewriteParameters, ErrIdempotencyConflict, ErrCrossProjectRelation, ErrMockRewriteFailed} {
		if errors.Is(err, known) {
			return err
		}
	}
	return ErrInternal
}
