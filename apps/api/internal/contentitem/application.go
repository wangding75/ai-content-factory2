package contentitem

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// Application errors are safe to expose to a transport. The original cause is
// intentionally not returned so SQL diagnostics and implementation details stay private.
var (
	ErrIdempotencyKeyRequired      = errors.New("idempotency key required")
	ErrInvalidGenerationParameters = errors.New("invalid generation parameters")
	ErrMockGenerationFailed        = errors.New("mock generation failed")
	ErrValidation                  = errors.New("content item validation failed")
	ErrInternal                    = errors.New("content item internal error")
)

type contentStore interface {
	CreateOrGet(context.Context, uuid.UUID) (CreateResult, error)
	GetByID(context.Context, uuid.UUID) (Detail, error)
	SaveDraft(context.Context, uuid.UUID, int, DraftPatch) (Detail, error)
	PersistMockGeneration(context.Context, GenerationRequest) (GenerationOutcome, error)
	RecordMockGenerationFailure(context.Context, FailureRequest) (WorkflowRun, error)
}

// Generator is injectable so tests can exercise execution failure without a real model.
type Generator interface {
	Generate(MockGenerationInput) (MockGenerationOutput, error)
}

type MockGenerationInput struct {
	ContentItemID uuid.UUID
	Parameters    MockGenerationParameters
}
type MockGenerationOutput struct {
	Title, Content, Summary string
}

// OptionalUUIDs preserves JSON omission independently from an explicitly empty array.
type OptionalUUIDs struct {
	Set   bool
	Value []uuid.UUID
}

// MockGenerationParameters mirrors the frozen schema exactly. The Set bits are
// needed at this layer to preserve omitted, null, and empty-string distinctions.
type MockGenerationParameters struct {
	ChapterGoal, CreationNotes                     OptionalString
	StorylineRefs, MaterialRefs, ForeshadowingRefs OptionalUUIDs
}

type CreateOrGetCommand struct{ ChapterPlanID uuid.UUID }
type GetCommand struct{ ContentItemID uuid.UUID }
type SaveDraftCommand struct {
	ContentItemID           uuid.UUID
	ExpectedVersion         int
	Title, Content, Summary OptionalString
}
type MockGenerateCommand struct {
	ContentItemID   uuid.UUID
	ExpectedVersion int
	IdempotencyKey  string
	Parameters      MockGenerationParameters
}
type MockGenerateResult struct {
	Detail      Detail
	WorkflowRun WorkflowRun
	Fingerprint string
}

type Application struct {
	store           contentStore
	generator       Generator
	reviewGenerator ReviewGenerator
}

func NewApplication(store contentStore, generator Generator) *Application {
	return NewApplicationWithGenerators(store, generator, nil)
}

// NewApplicationWithGenerators permits deterministic review execution to be
// replaced in tests without coupling this package to a model or network client.
func NewApplicationWithGenerators(store contentStore, generator Generator, reviewGenerator ReviewGenerator) *Application {
	if generator == nil {
		generator = DeterministicGenerator{}
	}
	if reviewGenerator == nil {
		reviewGenerator = DeterministicReviewGenerator{}
	}
	return &Application{store: store, generator: generator, reviewGenerator: reviewGenerator}
}

func (a *Application) CreateOrGet(ctx context.Context, c CreateOrGetCommand) (CreateResult, error) {
	if c.ChapterPlanID == uuid.Nil {
		return CreateResult{}, ErrValidation
	}
	v, err := a.store.CreateOrGet(ctx, c.ChapterPlanID)
	if err != nil {
		return CreateResult{}, mapApplicationError(err)
	}
	return v, nil
}
func (a *Application) Get(ctx context.Context, c GetCommand) (Detail, error) {
	if c.ContentItemID == uuid.Nil {
		return Detail{}, ErrValidation
	}
	v, err := a.store.GetByID(ctx, c.ContentItemID)
	if err != nil {
		return Detail{}, mapApplicationError(err)
	}
	return v, nil
}
func (a *Application) SaveDraft(ctx context.Context, c SaveDraftCommand) (Detail, error) {
	if c.ContentItemID == uuid.Nil || c.ExpectedVersion < 1 || (!c.Title.Set && !c.Content.Set && !c.Summary.Set) {
		return Detail{}, ErrValidation
	}
	if err := validateDraft(c); err != nil {
		return Detail{}, err
	}
	current, err := a.Get(ctx, GetCommand{ContentItemID: c.ContentItemID})
	if err != nil {
		return Detail{}, err
	}
	if current.CurrentVersion.Status != "editable_draft" {
		return Detail{}, ErrContentVersionLocked
	}
	if current.CurrentVersion.Version != c.ExpectedVersion {
		return Detail{}, ErrVersionConflict
	}
	content := current.CurrentVersion.Content
	if c.Content.Set {
		content = *c.Content.Value
	}
	words := wordCount(content)
	patch := DraftPatch{Title: c.Title, Content: c.Content, Summary: c.Summary, WordCount: OptionalInt{Set: true, Value: &words}}
	v, err := a.store.SaveDraft(ctx, c.ContentItemID, c.ExpectedVersion, patch)
	if err != nil {
		return Detail{}, mapApplicationError(err)
	}
	return v, nil
}
func validateDraft(c SaveDraftCommand) error {
	if c.Title.Set && (c.Title.Value == nil || strings.TrimSpace(*c.Title.Value) == "" || len(*c.Title.Value) > 120) {
		return ErrValidation
	}
	if c.Content.Set && (c.Content.Value == nil || len(*c.Content.Value) > 200000) {
		return ErrValidation
	}
	if c.Summary.Set && c.Summary.Value != nil && len(*c.Summary.Value) > 5000 {
		return ErrValidation
	}
	return nil
}

func (a *Application) MockGenerate(ctx context.Context, c MockGenerateCommand) (MockGenerateResult, error) {
	if c.ContentItemID == uuid.Nil || c.ExpectedVersion < 1 {
		return MockGenerateResult{}, ErrInvalidGenerationParameters
	}
	if strings.TrimSpace(c.IdempotencyKey) == "" {
		return MockGenerateResult{}, ErrIdempotencyKeyRequired
	}
	if err := validateGenerationParameters(c.Parameters); err != nil {
		return MockGenerateResult{}, err
	}
	fingerprint, canonical := GenerationFingerprint(c.ContentItemID, c.ExpectedVersion, c.Parameters)
	generated, err := a.generator.Generate(MockGenerationInput{ContentItemID: c.ContentItemID, Parameters: c.Parameters})
	if err != nil {
		// Failure records contain only fixed safe diagnostics, never generator details.
		_, recordErr := a.store.RecordMockGenerationFailure(ctx, FailureRequest{ContentItemID: c.ContentItemID, IdempotencyKey: c.IdempotencyKey, Fingerprint: fingerprint, InputJSON: canonical, ErrorCode: "mock_generation_failed", ErrorSummary: "mock generation failed"})
		if recordErr != nil {
			return MockGenerateResult{}, mapApplicationError(recordErr)
		}
		return MockGenerateResult{}, ErrMockGenerationFailed
	}
	parametersJSON := frozenParametersJSON(c.Parameters)
	result := GenerationResult{Title: generated.Title, Content: generated.Content, Summary: generated.Summary, SummarySet: true, WordCount: wordCount(generated.Content), Parameters: parametersJSON, InputJSON: canonical, OutputJSON: generationOutputJSON(generated)}
	out, err := a.store.PersistMockGeneration(ctx, GenerationRequest{ContentItemID: c.ContentItemID, ExpectedVersion: c.ExpectedVersion, IdempotencyKey: c.IdempotencyKey, Fingerprint: fingerprint, Result: result})
	if err != nil {
		return MockGenerateResult{}, mapApplicationError(err)
	}
	return MockGenerateResult{Detail: out.Detail, WorkflowRun: out.WorkflowRun, Fingerprint: fingerprint}, nil
}

func validateGenerationParameters(p MockGenerationParameters) error {
	if !p.ChapterGoal.Set || !p.CreationNotes.Set || !p.StorylineRefs.Set || !p.MaterialRefs.Set || !p.ForeshadowingRefs.Set {
		return ErrInvalidGenerationParameters
	}
	if (p.ChapterGoal.Value != nil && len(*p.ChapterGoal.Value) > 2000) || (p.CreationNotes.Value != nil && len(*p.CreationNotes.Value) > 2000) || len(p.StorylineRefs.Value) == 0 {
		return ErrInvalidGenerationParameters
	}
	for _, group := range [][]uuid.UUID{p.StorylineRefs.Value, p.MaterialRefs.Value, p.ForeshadowingRefs.Value} {
		seen := map[uuid.UUID]bool{}
		for _, id := range group {
			if id == uuid.Nil || seen[id] {
				return ErrInvalidGenerationParameters
			}
			seen[id] = true
		}
	}
	return nil
}

// GenerationFingerprint uses SHA-256 over canonical JSON. It deliberately omits
// the idempotency key while representing absent fields differently from null/text.
func GenerationFingerprint(itemID uuid.UUID, expected int, p MockGenerationParameters) (string, []byte) {
	canonical, _ := json.Marshal(struct {
		ContentItemID   string              `json:"content_item_id"`
		ExpectedVersion int                 `json:"expected_version"`
		Parameters      canonicalParameters `json:"parameters"`
	}{itemID.String(), expected, canonicalizeParameters(p)})
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), canonical
}

type canonicalOptionalString struct {
	Set   bool    `json:"set"`
	Value *string `json:"value"`
}
type canonicalOptionalUUIDs struct {
	Set   bool     `json:"set"`
	Value []string `json:"value"`
}
type canonicalParameters struct {
	ChapterGoal       canonicalOptionalString `json:"chapter_goal"`
	StorylineRefs     canonicalOptionalUUIDs  `json:"storyline_refs_json"`
	MaterialRefs      canonicalOptionalUUIDs  `json:"material_refs_json"`
	ForeshadowingRefs canonicalOptionalUUIDs  `json:"foreshadowing_refs_json"`
	CreationNotes     canonicalOptionalString `json:"creation_notes"`
}

func canonicalizeParameters(p MockGenerationParameters) canonicalParameters {
	ids := func(x OptionalUUIDs) canonicalOptionalUUIDs {
		out := make([]string, len(x.Value))
		for i, id := range x.Value {
			out[i] = id.String()
		}
		return canonicalOptionalUUIDs{x.Set, out}
	}
	return canonicalParameters{canonicalOptionalString{p.ChapterGoal.Set, copyString(p.ChapterGoal.Value)}, ids(p.StorylineRefs), ids(p.MaterialRefs), ids(p.ForeshadowingRefs), canonicalOptionalString{p.CreationNotes.Set, copyString(p.CreationNotes.Value)}}
}

// frozenParametersJSON is the persisted Schema-shaped parameter document. Unlike
// the fingerprint document it has no implementation-only omission markers.
func frozenParametersJSON(p MockGenerationParameters) []byte {
	ids := func(x OptionalUUIDs) []string {
		out := make([]string, len(x.Value))
		for i, id := range x.Value {
			out[i] = id.String()
		}
		return out
	}
	b, _ := json.Marshal(struct {
		ChapterGoal       *string  `json:"chapter_goal"`
		StorylineRefs     []string `json:"storyline_refs_json"`
		MaterialRefs      []string `json:"material_refs_json"`
		ForeshadowingRefs []string `json:"foreshadowing_refs_json"`
		CreationNotes     *string  `json:"creation_notes"`
	}{copyString(p.ChapterGoal.Value), ids(p.StorylineRefs), ids(p.MaterialRefs), ids(p.ForeshadowingRefs), copyString(p.CreationNotes.Value)})
	return b
}

func copyString(v *string) *string {
	if v == nil {
		return nil
	}
	out := *v
	return &out
}
func generationOutputJSON(v MockGenerationOutput) []byte {
	b, _ := json.Marshal(struct {
		Title   string `json:"title"`
		Content string `json:"content"`
		Summary string `json:"summary"`
	}{v.Title, v.Content, v.Summary})
	return b
}
func wordCount(s string) int { return len(strings.Fields(s)) }

// DeterministicGenerator is intentionally local-only: no clock, random number,
// network, model identifier, prompt, or streaming concern influences its output.
type DeterministicGenerator struct{}

func (DeterministicGenerator) Generate(in MockGenerationInput) (MockGenerationOutput, error) {
	text := func(v *string) string {
		if v == nil {
			return "(none)"
		}
		if *v == "" {
			return "(empty)"
		}
		return *v
	}
	ids := func(v []uuid.UUID) string {
		out := make([]string, len(v))
		for i, id := range v {
			out[i] = id.String()
		}
		if len(out) == 0 {
			return "(none)"
		}
		return strings.Join(out, ", ")
	}
	p := in.Parameters
	content := fmt.Sprintf("Chapter goal: %s.\n\nStoryline references: %s.\n\nMaterial references: %s.\n\nForeshadowing references: %s.\n\nCreation notes: %s.", text(p.ChapterGoal.Value), ids(p.StorylineRefs.Value), ids(p.MaterialRefs.Value), ids(p.ForeshadowingRefs.Value), text(p.CreationNotes.Value))
	return MockGenerationOutput{Title: "Mock chapter", Content: content, Summary: "Deterministic mock chapter draft."}, nil
}

func mapApplicationError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrChapterPlanNotFound), errors.Is(err, ErrChapterPlanNotConfirmed), errors.Is(err, ErrContentItemNotFound), errors.Is(err, ErrContentVersionNotFound), errors.Is(err, ErrContentVersionLocked), errors.Is(err, ErrContentVersionReviewed), errors.Is(err, ErrVersionConflict), errors.Is(err, ErrIdempotencyConflict), errors.Is(err, ErrReviewNotFound):
		return err
	default:
		return ErrInternal
	}
}
