package contentitem

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"

	"github.com/google/uuid"
)

var (
	ErrInvalidReviewParameters = errors.New("invalid review parameters")
	ErrMockReviewFailed        = errors.New("mock review failed")
	ErrInvalidPagination       = errors.New("invalid pagination")
)

// reviewStore is deliberately narrow. ListReviews and GetReview each delegate
// to one repository operation, avoiding application-side query orchestration.
type reviewStore interface {
	GetReviewContentVersion(context.Context, uuid.UUID, uuid.UUID) (ContentVersion, error)
	PersistMockReview(context.Context, ReviewRequest) (ReviewOutcome, error)
	RecordMockReviewFailure(context.Context, ReviewFailureRequest) (WorkflowRun, error)
	ListReviews(context.Context, uuid.UUID, int, int) (ReviewList, error)
	GetReview(context.Context, uuid.UUID) (ReviewDetail, error)
}

type ReviewGenerator interface {
	Review(MockReviewInput) (ReviewResult, error)
}

type MockReviewInput struct {
	ContentItemID, ContentVersionID uuid.UUID
	ContentVersion                  ContentVersion
}

type MockReviewCommand struct {
	ContentItemID, ContentVersionID uuid.UUID
	ExpectedVersion                 int
	IdempotencyKey                  string
}
type MockReviewResult struct {
	Detail          Detail
	Review          ReviewReport
	Findings        []ReviewFinding
	Recommendations []ReviewRecommendation
	WorkflowRun     WorkflowRun
	Fingerprint     string
}
type ListReviewsCommand struct {
	ContentItemID uuid.UUID
	Limit, Offset int
}
type GetReviewCommand struct{ ReviewID uuid.UUID }

func (a *Application) MockReview(ctx context.Context, c MockReviewCommand) (MockReviewResult, error) {
	if strings.TrimSpace(c.IdempotencyKey) == "" {
		return MockReviewResult{}, ErrIdempotencyKeyRequired
	}
	if c.ContentItemID == uuid.Nil || c.ContentVersionID == uuid.Nil || c.ExpectedVersion < 1 {
		return MockReviewResult{}, ErrInvalidReviewParameters
	}
	store, ok := a.store.(reviewStore)
	if !ok {
		return MockReviewResult{}, ErrInternal
	}
	version, err := store.GetReviewContentVersion(ctx, c.ContentItemID, c.ContentVersionID)
	if err != nil {
		return MockReviewResult{}, mapApplicationError(err)
	}
	if version.ID != c.ContentVersionID {
		return MockReviewResult{}, ErrContentVersionNotFound
	}
	// A frozen target still reaches PersistMockReview with deterministic input so
	// the repository can return an existing same-key/same-fingerprint outcome.
	// A new key remains a safe already-reviewed error and creates no new run.
	if version.Status != "editable_draft" && version.Status != "frozen" {
		return MockReviewResult{}, ErrContentVersionLocked
	}
	if version.Status != "frozen" && version.Version != c.ExpectedVersion {
		return MockReviewResult{}, ErrVersionConflict
	}
	fingerprint, canonical := ReviewFingerprint(c.ContentItemID, c.ContentVersionID, c.ExpectedVersion)
	generated, err := a.reviewGenerator.Review(MockReviewInput{ContentItemID: c.ContentItemID, ContentVersionID: c.ContentVersionID, ContentVersion: version})
	if err != nil {
		_, recordErr := store.RecordMockReviewFailure(ctx, ReviewFailureRequest{ContentItemID: c.ContentItemID, ContentVersionID: c.ContentVersionID, IdempotencyKey: c.IdempotencyKey, Fingerprint: fingerprint, InputJSON: canonical, ErrorCode: "mock_review_failed", ErrorSummary: "mock review failed"})
		if recordErr != nil {
			return MockReviewResult{}, mapApplicationError(recordErr)
		}
		return MockReviewResult{}, ErrMockReviewFailed
	}
	generated.InputJSON = canonical
	generated.OutputJSON = reviewOutputJSON(generated)
	generated.Findings = stableFindings(generated.Findings)
	generated.Recommendations = stableRecommendations(generated.Recommendations)
	out, err := store.PersistMockReview(ctx, ReviewRequest{ContentItemID: c.ContentItemID, ContentVersionID: c.ContentVersionID, ExpectedVersion: c.ExpectedVersion, IdempotencyKey: c.IdempotencyKey, Fingerprint: fingerprint, Result: generated})
	if err != nil {
		return MockReviewResult{}, mapApplicationError(err)
	}
	return MockReviewResult{Detail: out.Detail, Review: out.Review, Findings: out.Findings, Recommendations: out.Recommendations, WorkflowRun: out.WorkflowRun, Fingerprint: fingerprint}, nil
}

// ReviewFingerprint is SHA-256 over canonical JSON containing every formal
// frozen request field. The key, clock, random values, and internal fields are
// intentionally absent; encoding/json fixes the object-field order.
func ReviewFingerprint(itemID, versionID uuid.UUID, expectedVersion int) (string, []byte) {
	canonical, _ := json.Marshal(struct {
		ContentItemID    string `json:"content_item_id"`
		ContentVersionID string `json:"content_version_id"`
		ExpectedVersion  int    `json:"expected_version"`
	}{itemID.String(), versionID.String(), expectedVersion})
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), canonical
}

func (a *Application) ListReviews(ctx context.Context, c ListReviewsCommand) (ReviewList, error) {
	if c.ContentItemID == uuid.Nil || c.Limit < 1 || c.Limit > 100 || c.Offset < 0 {
		return ReviewList{}, ErrInvalidPagination
	}
	store, ok := a.store.(reviewStore)
	if !ok {
		return ReviewList{}, ErrInternal
	}
	out, err := store.ListReviews(ctx, c.ContentItemID, c.Limit, c.Offset)
	if err != nil {
		return ReviewList{}, mapApplicationError(err)
	}
	return out, nil
}

func (a *Application) GetReview(ctx context.Context, c GetReviewCommand) (ReviewDetail, error) {
	if c.ReviewID == uuid.Nil {
		return ReviewDetail{}, ErrInvalidReviewParameters
	}
	store, ok := a.store.(reviewStore)
	if !ok {
		return ReviewDetail{}, ErrInternal
	}
	out, err := store.GetReview(ctx, c.ReviewID)
	if err != nil {
		return ReviewDetail{}, mapApplicationError(err)
	}
	return out, nil
}

type DeterministicReviewGenerator struct{}

func (DeterministicReviewGenerator) Review(MockReviewInput) (ReviewResult, error) {
	return ReviewResult{Conclusion: "revise", Score: 70, Summary: "Deterministic mock review: revise pacing and reinforce foreshadowing.", Findings: []ReviewFinding{
		{Category: "pacing", Severity: "medium", Title: "Strengthen scene pacing", Description: "Tighten transitions between major beats.", LocationJSON: []byte(`{"start_offset":0,"end_offset":0}`)},
		{Category: "foreshadowing", Severity: "low", Title: "Reinforce foreshadowing", Description: "Make the setup for later developments more explicit."},
	}, Recommendations: []ReviewRecommendation{
		{Priority: "high", Title: "Revise pacing", Description: "Condense transitions and foreground the scene objective."},
		{Priority: "medium", Title: "Add a foreshadowing cue", Description: "Plant one concrete detail that supports the later turn."},
	}}, nil
}

func stableFindings(in []ReviewFinding) []ReviewFinding {
	out := append([]ReviewFinding(nil), in...)
	for i := range out {
		out[i].SortOrder = i
	}
	return out
}
func stableRecommendations(in []ReviewRecommendation) []ReviewRecommendation {
	out := append([]ReviewRecommendation(nil), in...)
	for i := range out {
		out[i].SortOrder = i
	}
	return out
}
func reviewOutputJSON(v ReviewResult) []byte {
	b, _ := json.Marshal(struct {
		Conclusion      string                 `json:"conclusion"`
		Score           int                    `json:"score"`
		Summary         string                 `json:"summary"`
		Findings        []ReviewFinding        `json:"findings"`
		Recommendations []ReviewRecommendation `json:"recommendations"`
	}{v.Conclusion, v.Score, v.Summary, v.Findings, v.Recommendations})
	return b
}
