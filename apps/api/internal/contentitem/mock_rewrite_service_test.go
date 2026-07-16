package contentitem

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type fakeRewriteStore struct {
	detail           Detail
	source           ContentVersion
	review           ReviewDetail
	target           ContentVersion
	prior            *WorkflowRun
	fingerprintPrior *WorkflowRun
	err              map[string]error
	calls            []string
}

func (f *fakeRewriteStore) fail(name string) error { return f.err[name] }
func (f *fakeRewriteStore) GetByID(context.Context, uuid.UUID) (Detail, error) {
	f.calls = append(f.calls, "item")
	return f.detail, f.fail("item")
}
func (f *fakeRewriteStore) GetReview(context.Context, uuid.UUID) (ReviewDetail, error) {
	f.calls = append(f.calls, "review")
	return f.review, f.fail("review")
}
func (f *fakeRewriteStore) GetContentVersion(_ context.Context, id uuid.UUID) (ContentVersion, error) {
	f.calls = append(f.calls, "version")
	if e := f.fail("version"); e != nil {
		return ContentVersion{}, e
	}
	if id == f.source.ID {
		return f.source, nil
	}
	if id == f.target.ID {
		return f.target, nil
	}
	return ContentVersion{}, ErrContentVersionNotFound
}
func (f *fakeRewriteStore) ListContentVersions(context.Context, uuid.UUID, ContentVersionPageOptions) (ContentVersionPage, error) {
	return ContentVersionPage{}, nil
}
func (f *fakeRewriteStore) CountContentVersions(context.Context, uuid.UUID) (int, error) {
	return 0, nil
}
func (f *fakeRewriteStore) GetContentVersionByNumber(context.Context, uuid.UUID, int) (ContentVersion, error) {
	return ContentVersion{}, ErrContentVersionNotFound
}
func (f *fakeRewriteStore) CreateContentVersion(_ context.Context, _ pgx.Tx, v ContentVersion) (ContentVersion, error) {
	f.calls = append(f.calls, "create-v2")
	if e := f.fail("create-v2"); e != nil {
		return ContentVersion{}, e
	}
	f.target = v
	return v, nil
}
func (f *fakeRewriteStore) GetWorkflowRun(context.Context, uuid.UUID) (WorkflowRun, error) {
	return WorkflowRun{}, ErrWorkflowRunNotFound
}
func (f *fakeRewriteStore) CreateMockRewriteRunning(_ context.Context, _ pgx.Tx, r WorkflowRun) (WorkflowRun, error) {
	f.calls = append(f.calls, "running")
	if e := f.fail("running"); e != nil {
		return WorkflowRun{}, e
	}
	return r, nil
}
func (f *fakeRewriteStore) MarkMockRewriteSucceeded(_ context.Context, _ pgx.Tx, id, target uuid.UUID, output []byte, at time.Time) (WorkflowRun, error) {
	f.calls = append(f.calls, "succeeded")
	if e := f.fail("succeeded"); e != nil {
		return WorkflowRun{}, e
	}
	return WorkflowRun{ID: id, Status: WorkflowRunStatusSucceeded, TargetContentVersionID: &target, OutputJSON: output, FinishedAt: &at}, nil
}
func (f *fakeRewriteStore) MarkMockRewriteFailed(_ context.Context, _ pgx.Tx, id uuid.UUID, code, summary string, at time.Time) (WorkflowRun, error) {
	f.calls = append(f.calls, "failed")
	if e := f.fail("failed"); e != nil {
		return WorkflowRun{}, e
	}
	return WorkflowRun{ID: id, Status: WorkflowRunStatusFailed, ErrorCode: &code, ErrorSummary: &summary, FinishedAt: &at}, nil
}
func (f *fakeRewriteStore) FindMockRewriteByIdempotencyKey(context.Context, uuid.UUID, string) (WorkflowRun, error) {
	f.calls = append(f.calls, "idempotency")
	if f.prior == nil {
		return WorkflowRun{}, ErrWorkflowRunNotFound
	}
	return *f.prior, nil
}
func (f *fakeRewriteStore) FindMockRewriteByFingerprint(context.Context, uuid.UUID, string) (WorkflowRun, error) {
	if f.fingerprintPrior != nil {
		return *f.fingerprintPrior, nil
	}
	return WorkflowRun{}, ErrWorkflowRunNotFound
}
func (f *fakeRewriteStore) LatestMockRewrite(context.Context, uuid.UUID) (WorkflowRun, error) {
	return WorkflowRun{}, ErrWorkflowRunNotFound
}

type fakeRewriteProvider struct {
	calls  int
	output MockRewriteOutput
	err    error
}

func (p *fakeRewriteProvider) Rewrite(context.Context, MockRewriteInput) (MockRewriteOutput, error) {
	p.calls++
	return p.output, p.err
}

type fakeRewriteTransactions struct {
	calls     []string
	commitErr error
}

func (t *fakeRewriteTransactions) InTransaction(_ context.Context, fn func(pgx.Tx) error) error {
	t.calls = append(t.calls, "begin")
	if err := fn(nil); err != nil {
		t.calls = append(t.calls, "rollback")
		return err
	}
	t.calls = append(t.calls, "commit")
	return t.commitErr
}

func rewriteFixture() (*fakeRewriteStore, *fakeRewriteProvider, *fakeRewriteTransactions, MockRewriteCommand) {
	project, item, sourceID, reviewID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	source := ContentVersion{ID: sourceID, ContentItemID: item, VersionNo: 1, Version: 4, Title: "Original", Content: "one two three", WordCount: 3, Source: ContentVersionSourceMockGenerated, Status: ContentVersionStatusFrozen}
	store := &fakeRewriteStore{source: source, detail: Detail{Item: ContentItem{ID: item, ProjectID: project, CurrentVersionID: sourceID}, CurrentVersion: source}, review: ReviewDetail{Review: ReviewReport{ID: reviewID, ProjectID: project, ContentItemID: item, ContentVersionID: sourceID, Status: "completed"}}, err: map[string]error{}}
	provider := &fakeRewriteProvider{output: MockRewriteOutput{Title: "Rewritten", Content: "one two three refined", Summary: "safe summary", WordCount: 4, ProviderKey: WorkflowProviderMock, OutputSummary: []byte(`{"provider":"mock"}`)}}
	tx := &fakeRewriteTransactions{}
	return store, provider, tx, MockRewriteCommand{ContentItemID: item, SourceContentVersionID: sourceID, ReviewReportID: reviewID, ExpectedVersion: 4, IdempotencyKey: "rewrite-key", BusinessInputID: "business-input", Parameters: MockRewriteParameters{RewriteFocus: []string{"pacing"}}}
}

func TestMockRewriteServiceSuccessCreatesV2AndSucceedsRun(t *testing.T) {
	store, provider, tx, command := rewriteFixture()
	before := store.detail.Item.CurrentVersionID
	out, err := NewMockRewriteService(store, provider, tx).Rewrite(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	if out.Reused || out.TargetContentVersion == nil || out.TargetContentVersion.VersionNo != 2 || out.TargetContentVersion.Source != ContentVersionSourceMockRewrite || out.TargetContentVersion.Status != ContentVersionStatusEditableDraft {
		t.Fatalf("unexpected target: %#v", out)
	}
	if out.WorkflowRun.Status != WorkflowRunStatusSucceeded || out.WorkflowRun.TargetContentVersionID == nil {
		t.Fatalf("unexpected run: %#v", out.WorkflowRun)
	}
	if store.detail.Item.CurrentVersionID != before || provider.calls != 1 {
		t.Fatalf("current version/provider changed: %s %d", store.detail.Item.CurrentVersionID, provider.calls)
	}
	if !reflect.DeepEqual(tx.calls, []string{"begin", "commit"}) || !reflect.DeepEqual(store.calls, []string{"item", "version", "review", "idempotency", "running", "create-v2", "succeeded"}) {
		t.Fatalf("unexpected sequence: tx=%v store=%v", tx.calls, store.calls)
	}
}

func TestMockRewriteServiceReusesMatchingIdempotencyRun(t *testing.T) {
	store, provider, tx, command := rewriteFixture()
	target := ContentVersion{ID: uuid.New(), ContentItemID: store.source.ContentItemID, VersionNo: 2}
	store.target = target
	prior := WorkflowRun{ID: uuid.New(), Status: WorkflowRunStatusSucceeded, RequestFingerprint: MockRewriteFingerprint(command), TargetContentVersionID: &target.ID}
	store.prior = &prior
	out, err := NewMockRewriteService(store, provider, tx).Rewrite(context.Background(), command)
	if err != nil || !out.Reused || out.TargetContentVersion == nil || provider.calls != 0 || len(tx.calls) != 0 {
		t.Fatalf("unexpected replay: %#v %v calls=%d tx=%v", out, err, provider.calls, tx.calls)
	}
	command.Parameters.RewriteFocus = []string{"foreshadowing"}
	_, err = NewMockRewriteService(store, provider, tx).Rewrite(context.Background(), command)
	if !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("wanted idempotency conflict, got %v", err)
	}
}

func TestMockRewriteServiceReusesMatchingFingerprintRun(t *testing.T) {
	store, provider, tx, command := rewriteFixture()
	target := ContentVersion{ID: uuid.New(), ContentItemID: store.source.ContentItemID, VersionNo: 2}
	store.target = target
	prior := WorkflowRun{ID: uuid.New(), Status: WorkflowRunStatusSucceeded, RequestFingerprint: MockRewriteFingerprint(command), TargetContentVersionID: &target.ID}
	store.fingerprintPrior = &prior
	out, err := NewMockRewriteService(store, provider, tx).Rewrite(context.Background(), command)
	if err != nil || !out.Reused || provider.calls != 0 || len(tx.calls) != 0 {
		t.Fatalf("unexpected fingerprint replay: %#v %v", out, err)
	}
}

func TestMockRewriteServiceValidationRejectsInvalidSourceAndRelations(t *testing.T) {
	cases := []struct {
		name   string
		change func(*fakeRewriteStore, *MockRewriteCommand)
		want   error
	}{
		{"stale", func(s *fakeRewriteStore, c *MockRewriteCommand) { c.ExpectedVersion++ }, ErrVersionConflict},
		{"non-frozen", func(s *fakeRewriteStore, c *MockRewriteCommand) { s.source.Status = ContentVersionStatusEditableDraft }, ErrContentVersionNotFrozen},
		{"v2-rewrite", func(s *fakeRewriteStore, c *MockRewriteCommand) { s.source.VersionNo = 2 }, ErrSourceVersionMismatch},
		{"incomplete-review", func(s *fakeRewriteStore, c *MockRewriteCommand) { s.review.Review.Status = "running" }, ErrReviewNotCompleted},
		{"mismatched-review", func(s *fakeRewriteStore, c *MockRewriteCommand) { s.review.Review.ContentVersionID = uuid.New() }, ErrSourceVersionMismatch},
		{"cross-project", func(s *fakeRewriteStore, c *MockRewriteCommand) { s.review.Review.ProjectID = uuid.New() }, ErrCrossProjectRelation},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store, provider, tx, command := rewriteFixture()
			tc.change(store, &command)
			_, err := NewMockRewriteService(store, provider, tx).Rewrite(context.Background(), command)
			if !errors.Is(err, tc.want) || provider.calls != 0 || len(tx.calls) != 0 {
				t.Fatalf("got %v, provider=%d tx=%v", err, provider.calls, tx.calls)
			}
		})
	}
}

func TestMockRewriteServiceProviderFailurePersistsOnlyFailedRun(t *testing.T) {
	store, provider, tx, command := rewriteFixture()
	provider.err = errors.New("provider unavailable")
	out, err := NewMockRewriteService(store, provider, tx).Rewrite(context.Background(), command)
	if !errors.Is(err, ErrMockRewriteFailed) || out.WorkflowRun.Status != WorkflowRunStatusFailed || contains(store.calls, "create-v2") || !contains(store.calls, "failed") || !reflect.DeepEqual(tx.calls, []string{"begin", "commit"}) {
		t.Fatalf("unexpected failure: %#v %v calls=%v tx=%v", out, err, store.calls, tx.calls)
	}
}

func TestMockRewriteServiceWriteAndCommitFailuresRollBack(t *testing.T) {
	for _, name := range []string{"create-v2", "succeeded"} {
		t.Run(name, func(t *testing.T) {
			store, provider, tx, command := rewriteFixture()
			store.err[name] = errors.New("write failed")
			_, err := NewMockRewriteService(store, provider, tx).Rewrite(context.Background(), command)
			if !errors.Is(err, ErrInternal) || !reflect.DeepEqual(tx.calls, []string{"begin", "rollback"}) || provider.calls != 1 {
				t.Fatalf("got %v tx=%v", err, tx.calls)
			}
		})
	}
	store, provider, tx, command := rewriteFixture()
	tx.commitErr = errors.New("commit failed")
	_, err := NewMockRewriteService(store, provider, tx).Rewrite(context.Background(), command)
	if !errors.Is(err, ErrInternal) || !reflect.DeepEqual(tx.calls, []string{"begin", "commit"}) || provider.calls != 1 {
		t.Fatalf("commit failure got %v tx=%v", err, tx.calls)
	}
}

func TestMockRewriteServiceCancellationAndFingerprint(t *testing.T) {
	store, provider, tx, command := rewriteFixture()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := NewMockRewriteService(store, provider, tx).Rewrite(ctx, command)
	if !errors.Is(err, context.Canceled) || provider.calls != 0 || len(tx.calls) != 0 {
		t.Fatalf("cancellation got %v", err)
	}
	first, second := MockRewriteFingerprint(command), MockRewriteFingerprint(command)
	if first != second {
		t.Fatal("fingerprint is not stable")
	}
	command.Parameters.PreserveEnding = true
	if first == MockRewriteFingerprint(command) {
		t.Fatal("fingerprint ignored formal parameters")
	}
}

func contains(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}
