package contentitem

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
)

type fakeContentStore struct {
	detail                                   Detail
	create                                   CreateResult
	err                                      error
	creates, gets, saves, persists, failures int
	patch                                    DraftPatch
	generation                               GenerationRequest
	failure                                  FailureRequest
	persisted                                GenerationOutcome
	run                                      WorkflowRun
}

func (f *fakeContentStore) CreateOrGet(context.Context, uuid.UUID) (CreateResult, error) {
	f.creates++
	return f.create, f.err
}
func (f *fakeContentStore) GetByID(context.Context, uuid.UUID) (Detail, error) {
	f.gets++
	return f.detail, f.err
}
func (f *fakeContentStore) SaveDraft(_ context.Context, _ uuid.UUID, _ int, p DraftPatch) (Detail, error) {
	f.saves++
	f.patch = p
	return f.detail, f.err
}
func (f *fakeContentStore) PersistMockGeneration(_ context.Context, r GenerationRequest) (GenerationOutcome, error) {
	f.persists++
	f.generation = r
	if f.err != nil {
		return GenerationOutcome{}, f.err
	}
	if f.persisted.Detail.Item.ID == uuid.Nil {
		f.persisted = GenerationOutcome{Detail: f.detail, WorkflowRun: WorkflowRun{ID: uuid.New(), Status: "succeeded", RequestFingerprint: r.Fingerprint}}
	}
	return f.persisted, nil
}
func (f *fakeContentStore) RecordMockGenerationFailure(_ context.Context, r FailureRequest) (WorkflowRun, error) {
	f.failures++
	f.failure = r
	if f.err != nil {
		return WorkflowRun{}, f.err
	}
	f.run = WorkflowRun{ID: uuid.New(), Status: "failed", ErrorCode: &r.ErrorCode, ErrorSummary: &r.ErrorSummary}
	return f.run, nil
}

type failingGenerator struct{ calls int }

func (g *failingGenerator) Generate(MockGenerationInput) (MockGenerationOutput, error) {
	g.calls++
	return MockGenerationOutput{}, errors.New("postgres stack prompt secret")
}
func appFixture() (*Application, *fakeContentStore, uuid.UUID, uuid.UUID) {
	p, id := uuid.New(), uuid.New()
	d := Detail{Item: ContentItem{ID: id, ProjectID: p}, CurrentVersion: ContentVersion{ID: uuid.New(), ContentItemID: id, Title: "Chapter", Content: "old text", Version: 1, VersionNo: 1, Status: "editable_draft"}}
	f := &fakeContentStore{detail: d, create: CreateResult{Detail: d, Created: true}}
	return NewApplication(f, nil), f, p, id
}
func validParameters() MockGenerationParameters {
	line := uuid.New()
	return MockGenerationParameters{ChapterGoal: OptionalString{Set: true, Value: ptr("goal")}, CreationNotes: OptionalString{Set: true, Value: nil}, StorylineRefs: OptionalUUIDs{Set: true, Value: []uuid.UUID{line}}, MaterialRefs: OptionalUUIDs{Set: true, Value: []uuid.UUID{}}, ForeshadowingRefs: OptionalUUIDs{Set: true, Value: []uuid.UUID{}}}
}
func ptr(s string) *string { return &s }
func TestApplicationCreateOrGetFirstAndExisting(t *testing.T) {
	a, f, _, _ := appFixture()
	if x, e := a.CreateOrGet(context.Background(), CreateOrGetCommand{ChapterPlanID: uuid.New()}); e != nil || !x.Created || f.creates != 1 {
		t.Fatalf("%+v %v", x, e)
	}
	f.create.Created = false
	if x, e := a.CreateOrGet(context.Background(), CreateOrGetCommand{ChapterPlanID: uuid.New()}); e != nil || x.Created || f.creates != 2 {
		t.Fatalf("%+v %v", x, e)
	}
}
func TestApplicationGetAndNotFound(t *testing.T) {
	a, f, _, id := appFixture()
	if _, e := a.Get(context.Background(), GetCommand{ContentItemID: id}); e != nil || f.gets != 1 {
		t.Fatal(e)
	}
	f.err = ErrContentItemNotFound
	if _, e := a.Get(context.Background(), GetCommand{ContentItemID: id}); !errors.Is(e, ErrContentItemNotFound) {
		t.Fatal(e)
	}
}
func TestApplicationSaveDraftTriStateAndGuards(t *testing.T) {
	a, f, _, id := appFixture()
	empty := ""
	if _, e := a.SaveDraft(context.Background(), SaveDraftCommand{ContentItemID: id, ExpectedVersion: 1, Content: OptionalString{Set: true, Value: ptr("new words")}, Summary: OptionalString{Set: true, Value: &empty}}); e != nil || f.saves != 1 || f.patch.WordCount.Value == nil || *f.patch.WordCount.Value != 2 {
		t.Fatalf("%v %#v", e, f.patch)
	}
	if !f.patch.Summary.Set || *f.patch.Summary.Value != "" {
		t.Fatal("empty lost")
	}
	if _, e := a.SaveDraft(context.Background(), SaveDraftCommand{ContentItemID: id, ExpectedVersion: 1, Summary: OptionalString{Set: true, Value: nil}}); e != nil || f.patch.Summary.Value != nil {
		t.Fatal(e)
	}
	if _, e := a.SaveDraft(context.Background(), SaveDraftCommand{ContentItemID: id, ExpectedVersion: 1, Title: OptionalString{Set: true, Value: ptr("")}}); !errors.Is(e, ErrValidation) {
		t.Fatal(e)
	}
	f.detail.CurrentVersion.Version = 2
	if _, e := a.SaveDraft(context.Background(), SaveDraftCommand{ContentItemID: id, ExpectedVersion: 1, Content: OptionalString{Set: true, Value: ptr("x")}}); !errors.Is(e, ErrVersionConflict) {
		t.Fatal(e)
	}
	f.detail.CurrentVersion.Version = 1
	f.detail.CurrentVersion.Status = "frozen"
	if _, e := a.SaveDraft(context.Background(), SaveDraftCommand{ContentItemID: id, ExpectedVersion: 1, Content: OptionalString{Set: true, Value: ptr("x")}}); !errors.Is(e, ErrContentVersionLocked) {
		t.Fatal(e)
	}
}
func TestFingerprintDeterministicAndTriState(t *testing.T) {
	_, _, _, id := appFixture()
	p := validParameters()
	a, _ := GenerationFingerprint(id, 1, p)
	b, _ := GenerationFingerprint(id, 1, p)
	if a != b {
		t.Fatal("unstable")
	}
	q := p
	q.ChapterGoal = OptionalString{Set: true, Value: nil}
	c, _ := GenerationFingerprint(id, 1, q)
	q.ChapterGoal = OptionalString{Set: false}
	d, _ := GenerationFingerprint(id, 1, q)
	q.ChapterGoal = OptionalString{Set: true, Value: ptr("")}
	e, _ := GenerationFingerprint(id, 1, q)
	if a == c || c == d || d == e {
		t.Fatal("tri-state collapsed")
	}
}
func TestApplicationMockGenerateSuccessIdempotencyAndValidation(t *testing.T) {
	a, f, _, id := appFixture()
	c := MockGenerateCommand{ContentItemID: id, ExpectedVersion: 1, IdempotencyKey: "key", Parameters: validParameters()}
	one, e := a.MockGenerate(context.Background(), c)
	if e != nil || f.persists != 1 || f.generation.Result.WordCount != wordCount(f.generation.Result.Content) || strings.Contains(f.generation.Result.Content, "prompt") || strings.Contains(string(f.generation.Result.Parameters), "\"set\"") {
		t.Fatalf("%+v %v", one, e)
	}
	two, e := a.MockGenerate(context.Background(), c)
	if e != nil || one.Fingerprint != two.Fingerprint || f.persists != 2 {
		t.Fatalf("%+v %v", two, e)
	}
	if _, e = a.MockGenerate(context.Background(), MockGenerateCommand{ContentItemID: id, ExpectedVersion: 1, Parameters: validParameters()}); !errors.Is(e, ErrIdempotencyKeyRequired) {
		t.Fatal(e)
	}
	bad := c
	bad.Parameters.StorylineRefs = OptionalUUIDs{Set: true}
	if _, e = a.MockGenerate(context.Background(), bad); !errors.Is(e, ErrInvalidGenerationParameters) || f.persists != 2 {
		t.Fatal(e)
	}
	f.err = ErrIdempotencyConflict
	if _, e = a.MockGenerate(context.Background(), c); !errors.Is(e, ErrIdempotencyConflict) {
		t.Fatal(e)
	}
}
func TestApplicationMockGeneratorFailureSafeRunAndInternalMapping(t *testing.T) {
	_, f, _, id := appFixture()
	g := &failingGenerator{}
	a := NewApplication(f, g)
	c := MockGenerateCommand{ContentItemID: id, ExpectedVersion: 1, IdempotencyKey: "fail", Parameters: validParameters()}
	if _, e := a.MockGenerate(context.Background(), c); !errors.Is(e, ErrMockGenerationFailed) || f.failures != 1 || f.persists != 0 || g.calls != 1 {
		t.Fatalf("%v %+v", e, f)
	}
	if *f.run.ErrorSummary != "mock generation failed" || strings.Contains(*f.run.ErrorSummary, "postgres") || strings.Contains(string(f.failure.InputJSON), "prompt") {
		t.Fatal("unsafe failure")
	}
	f.err = errors.New("sql password")
	if _, e := a.Get(context.Background(), GetCommand{ContentItemID: id}); !errors.Is(e, ErrInternal) {
		t.Fatal(e)
	}
}
