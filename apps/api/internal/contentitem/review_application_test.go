package contentitem

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
)

type fakeReviewStore struct {
	*fakeContentStore
	version                                                          ContentVersion
	getVersionErr, persistErr, failureErr, listErr, getErr           error
	getVersionCalls, persistCalls, failureCalls, listCalls, getCalls int
	persistReq                                                       ReviewRequest
	persisted                                                        *ReviewOutcome
	failureReq                                                       ReviewFailureRequest
	list                                                             ReviewList
	detail                                                           ReviewDetail
}

func (f *fakeReviewStore) GetReviewContentVersion(_ context.Context, _, _ uuid.UUID) (ContentVersion, error) {
	f.getVersionCalls++
	return f.version, f.getVersionErr
}
func (f *fakeReviewStore) PersistMockReview(_ context.Context, r ReviewRequest) (ReviewOutcome, error) {
	f.persistCalls++
	f.persistReq = r
	if f.persistErr != nil {
		return ReviewOutcome{}, f.persistErr
	}
	if f.persisted != nil {
		return *f.persisted, nil
	}
	return ReviewOutcome{Review: ReviewReport{ID: uuid.New(), ContentVersionID: r.ContentVersionID}, Findings: r.Result.Findings, Recommendations: r.Result.Recommendations, WorkflowRun: WorkflowRun{ID: uuid.New(), Status: "succeeded"}}, nil
}
func (f *fakeReviewStore) RecordMockReviewFailure(_ context.Context, r ReviewFailureRequest) (WorkflowRun, error) {
	f.failureCalls++
	f.failureReq = r
	if f.failureErr != nil {
		return WorkflowRun{}, f.failureErr
	}
	return WorkflowRun{ID: uuid.New(), Status: "failed", ErrorCode: ptr(r.ErrorCode), ErrorSummary: ptr(r.ErrorSummary)}, nil
}
func (f *fakeReviewStore) ListReviews(_ context.Context, _ uuid.UUID, _, _ int) (ReviewList, error) {
	f.listCalls++
	return f.list, f.listErr
}
func (f *fakeReviewStore) GetReview(_ context.Context, _ uuid.UUID) (ReviewDetail, error) {
	f.getCalls++
	return f.detail, f.getErr
}

type fakeReviewer struct {
	calls  int
	result ReviewResult
	err    error
}

func (g *fakeReviewer) Review(MockReviewInput) (ReviewResult, error) {
	g.calls++
	return g.result, g.err
}

func reviewFixture() (*Application, *fakeReviewStore, *fakeReviewer, MockReviewCommand) {
	item, versionID := uuid.New(), uuid.New()
	base := &fakeContentStore{}
	f := &fakeReviewStore{fakeContentStore: base, version: ContentVersion{ID: versionID, ContentItemID: item, Status: "editable_draft", Version: 3}, detail: ReviewDetail{ContentVersion: ContentVersion{ID: versionID, Content: "fixed"}, WorkflowRun: WorkflowRun{ID: uuid.New()}, Review: ReviewReport{ID: uuid.New()}}}
	g := &fakeReviewer{result: ReviewResult{Conclusion: "pass", Score: 90, Summary: "ok", Findings: []ReviewFinding{{Category: "pacing", Severity: "low", Title: "second", Description: "d"}, {Category: "foreshadowing", Severity: "medium", Title: "first", Description: "d"}}, Recommendations: []ReviewRecommendation{{Priority: "low", Title: "second", Description: "d"}, {Priority: "high", Title: "first", Description: "d"}}}}
	return NewApplicationWithGenerators(f, nil, g), f, g, MockReviewCommand{ContentItemID: item, ContentVersionID: versionID, ExpectedVersion: 3, IdempotencyKey: "review-key"}
}

func TestMockReviewSuccessDeterminismAndStableOrdering(t *testing.T) {
	a, f, g, c := reviewFixture()
	out, err := a.MockReview(context.Background(), c)
	if err != nil || g.calls != 1 || f.getVersionCalls != 1 || f.persistCalls != 1 || f.failureCalls != 0 || out.Review.ID == uuid.Nil || out.WorkflowRun.ID == uuid.Nil {
		t.Fatalf("success calls/result: err=%v generator=%d persist=%d result=%+v", err, g.calls, f.persistCalls, out)
	}
	if f.persistReq.ContentVersionID != c.ContentVersionID || f.persistReq.Fingerprint != out.Fingerprint || f.persistReq.Result.Findings[0].SortOrder != 0 || f.persistReq.Result.Findings[1].SortOrder != 1 || f.persistReq.Result.Recommendations[1].SortOrder != 1 {
		t.Fatalf("persisted review request is not stable: %+v", f.persistReq)
	}
	first, _ := DeterministicReviewGenerator{}.Review(MockReviewInput{})
	second, _ := DeterministicReviewGenerator{}.Review(MockReviewInput{})
	if first.Summary != second.Summary || first.Score != second.Score || first.Findings[0].Title != second.Findings[0].Title || first.Recommendations[0].Title != second.Recommendations[0].Title {
		t.Fatal("deterministic reviewer changed output")
	}
}

func TestReviewFingerprintDeterministicAndCanonical(t *testing.T) {
	item, version := uuid.New(), uuid.New()
	a, rawA := ReviewFingerprint(item, version, 2)
	b, rawB := ReviewFingerprint(item, version, 2)
	if a != b || string(rawA) != string(rawB) || len(a) != 64 {
		t.Fatalf("non-deterministic fingerprint: %q %q", a, b)
	}
	// JSON object member order is immaterial: both representations normalize to the same typed command.
	var x, y struct {
		ContentItemID    string `json:"content_item_id"`
		ContentVersionID string `json:"content_version_id"`
		ExpectedVersion  int    `json:"expected_version"`
	}
	_ = json.Unmarshal([]byte(`{"expected_version":2,"content_version_id":"`+version.String()+`","content_item_id":"`+item.String()+`"}`), &x)
	_ = json.Unmarshal([]byte(`{"content_item_id":"`+item.String()+`","expected_version":2,"content_version_id":"`+version.String()+`"}`), &y)
	fx, _ := ReviewFingerprint(uuid.MustParse(x.ContentItemID), uuid.MustParse(x.ContentVersionID), x.ExpectedVersion)
	fy, _ := ReviewFingerprint(uuid.MustParse(y.ContentItemID), uuid.MustParse(y.ContentVersionID), y.ExpectedVersion)
	if fx != fy {
		t.Fatal("field order affected fingerprint")
	}
}

func TestMockReviewPreExecutionGuards(t *testing.T) {
	a, f, g, c := reviewFixture()
	c.IdempotencyKey = " "
	if _, err := a.MockReview(context.Background(), c); !errors.Is(err, ErrIdempotencyKeyRequired) || f.getVersionCalls != 0 || g.calls != 0 {
		t.Fatalf("missing key guard: %v", err)
	}
	c.IdempotencyKey, c.ExpectedVersion = "x", 0
	if _, err := a.MockReview(context.Background(), c); !errors.Is(err, ErrInvalidReviewParameters) || f.getVersionCalls != 0 || g.calls != 0 {
		t.Fatalf("invalid parameter guard: %v", err)
	}
	c.ExpectedVersion = 3
	f.getVersionErr = ErrContentItemNotFound
	if _, err := a.MockReview(context.Background(), c); !errors.Is(err, ErrContentItemNotFound) || g.calls != 0 {
		t.Fatalf("not found guard: %v", err)
	}
	f.getVersionErr = nil
	f.version.Version = 2
	if _, err := a.MockReview(context.Background(), c); !errors.Is(err, ErrVersionConflict) || g.calls != 0 {
		t.Fatalf("stale guard: %v", err)
	}
	f.version.Version = 3
	f.version.Status = "frozen"
	f.persistErr = ErrContentVersionReviewed
	if _, err := a.MockReview(context.Background(), c); !errors.Is(err, ErrContentVersionReviewed) || f.persistCalls != 1 {
		t.Fatalf("reviewed guard: %v", err)
	}
	f.persistErr = nil
	f.version.Status = "locked"
	if _, err := a.MockReview(context.Background(), c); !errors.Is(err, ErrContentVersionLocked) || f.persistCalls != 1 {
		t.Fatalf("locked guard: %v", err)
	}
}

func TestMockReviewIdempotencyAndSafeRepositoryMapping(t *testing.T) {
	a, f, _, c := reviewFixture()
	f.version.Status = "frozen"
	id, runID := uuid.New(), uuid.New()
	f.persisted = &ReviewOutcome{Review: ReviewReport{ID: id}, WorkflowRun: WorkflowRun{ID: runID}}
	first, err := a.MockReview(context.Background(), c)
	second, err2 := a.MockReview(context.Background(), c)
	if err != nil || err2 != nil || first.Review.ID != id || second.Review.ID != id || first.WorkflowRun.ID != runID || second.WorkflowRun.ID != runID || f.persistCalls != 2 || first.Fingerprint != second.Fingerprint {
		t.Fatalf("same-key idempotency result: first=%+v second=%+v errors=%v/%v", first, second, err, err2)
	}

	a, f, _, c = reviewFixture()
	f.persistErr = ErrIdempotencyConflict
	if _, err := a.MockReview(context.Background(), c); !errors.Is(err, ErrIdempotencyConflict) || f.persistCalls != 1 {
		t.Fatalf("different payload mapping: %v", err)
	}
	f.persistErr = errors.New("sql: secret")
	if _, err := a.MockReview(context.Background(), c); !errors.Is(err, ErrInternal) || err.Error() != ErrInternal.Error() {
		t.Fatalf("unsafe repository error: %v", err)
	}
}

func TestMockReviewGeneratorFailureRecordsSafeFailedRun(t *testing.T) {
	a, f, g, c := reviewFixture()
	g.err = errors.New("prompt secret\nstack")
	if _, err := a.MockReview(context.Background(), c); !errors.Is(err, ErrMockReviewFailed) || f.failureCalls != 1 || f.persistCalls != 0 {
		t.Fatalf("failure handling: %v calls=%d persist=%d", err, f.failureCalls, f.persistCalls)
	}
	if f.failureReq.ErrorCode != "mock_review_failed" || f.failureReq.ErrorSummary != "mock review failed" || string(f.failureReq.InputJSON) == "" {
		t.Fatalf("unsafe/missing failed run: %+v", f.failureReq)
	}
}

func TestListReviewsAndGetReview(t *testing.T) {
	a, f, _, c := reviewFixture()
	f.list = ReviewList{Items: []ReviewReport{{ID: uuid.New()}, {ID: uuid.New()}}, Total: 3, Limit: 2, Offset: 1}
	list, err := a.ListReviews(context.Background(), ListReviewsCommand{ContentItemID: c.ContentItemID, Limit: 2, Offset: 1})
	if err != nil || f.listCalls != 1 || list.Total != 3 || list.Items[0].ID == uuid.Nil {
		t.Fatalf("list mapping: %+v %v", list, err)
	}
	if _, err := a.ListReviews(context.Background(), ListReviewsCommand{ContentItemID: c.ContentItemID, Limit: 0}); !errors.Is(err, ErrInvalidPagination) || f.listCalls != 1 {
		t.Fatalf("pagination validation: %v", err)
	}
	f.getErr = nil
	d, err := a.GetReview(context.Background(), GetReviewCommand{ReviewID: f.detail.Review.ID})
	if err != nil || f.getCalls != 1 || d.ContentVersion.ID != f.detail.ContentVersion.ID || d.ContentVersion.Content != "fixed" {
		t.Fatalf("fixed review detail: %+v %v", d, err)
	}
	f.getErr = ErrReviewNotFound
	if _, err := a.GetReview(context.Background(), GetReviewCommand{ReviewID: uuid.New()}); !errors.Is(err, ErrReviewNotFound) {
		t.Fatalf("review not found: %v", err)
	}
}
