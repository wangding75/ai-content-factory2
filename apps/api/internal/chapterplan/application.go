package chapterplan

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/local/ai-content-factory/apps/api/internal/foreshadowing"
	"github.com/local/ai-content-factory/apps/api/internal/material"
	"github.com/local/ai-content-factory/apps/api/internal/project"
	"github.com/local/ai-content-factory/apps/api/internal/storyline"
)

// Application errors are deliberately stable: callers never receive driver or SQL details.
var (
	ErrProjectNotFound               = errors.New("chapter plan project not found")
	ErrChapterPlanNotFound           = errors.New("chapter plan not found")
	ErrStorylineReferenceInvalid     = errors.New("chapter plan storyline reference invalid")
	ErrMaterialReferenceInvalid      = errors.New("chapter plan material reference invalid")
	ErrForeshadowingReferenceInvalid = errors.New("chapter plan foreshadowing reference invalid")
	ErrInvalidState                  = errors.New("chapter plan invalid state")
	ErrValidation                    = errors.New("chapter plan validation failed")
	ErrInternal                      = errors.New("chapter plan internal error")
)

type store interface {
	ListByProject(context.Context, uuid.UUID) ([]Plan, error)
	GetByID(context.Context, uuid.UUID) (Plan, error)
	SaveMock(context.Context, Run, []Plan) error
	Update(context.Context, Plan, int) (Plan, error)
	Delete(context.Context, uuid.UUID, int) error
	Confirm(context.Context, []Selection) ([]Plan, error)
}
type projectReader interface {
	Get(context.Context, uuid.UUID) (project.Project, error)
}
type storylineReader interface {
	GetByID(context.Context, uuid.UUID) (storyline.PlotLine, error)
	ListByProject(context.Context, uuid.UUID) ([]storyline.PlotLine, error)
}
type materialReader interface {
	GetByProjectAndMaterial(context.Context, uuid.UUID, uuid.UUID) (material.ProjectMaterialUsage, error)
	ListByProject(context.Context, uuid.UUID) ([]material.ProjectMaterialUsage, error)
}
type foreshadowingReader interface {
	GetByID(context.Context, uuid.UUID) (foreshadowing.Foreshadowing, error)
	ListByProject(context.Context, uuid.UUID) ([]foreshadowing.Foreshadowing, error)
}

type OptionalString struct {
	Set   bool
	Value *string
}
type OptionalStorylines struct {
	Set   bool
	Value []StorylineRef
}
type OptionalUUIDs struct {
	Set   bool
	Value []uuid.UUID
}

type UpdateCommand struct {
	ExpectedVersion           int
	ChapterNo                 *int
	Title, Summary            *string
	Storylines                OptionalStorylines
	Materials, Foreshadowings OptionalUUIDs
	Goal, Notes               OptionalString
}
type MockGenerateCommand struct {
	TargetStorylineID                                                                                                                uuid.UUID
	StartChapterNo, EndChapterNo, ChapterCount                                                                                       int
	IncludeMainStoryline, IncludeChildStorylines, IncludeProjectMaterials, IncludeUnpaidForeshadowings, IncludePriorChapterSummaries bool
	SummaryLength, ChapterPace                                                                                                       string
	GenerationNotes                                                                                                                  *string
	ActorID                                                                                                                          string
}
type MockGenerateResult struct {
	Run   Run
	Items []Plan
}

type Service struct {
	projects       projectReader
	plans          store
	storylines     storylineReader
	materials      materialReader
	foreshadowings foreshadowingReader
	now            func() time.Time
}

func NewService(projects projectReader, plans store, storylines storylineReader, materials materialReader, foreshadowings foreshadowingReader) *Service {
	return &Service{projects: projects, plans: plans, storylines: storylines, materials: materials, foreshadowings: foreshadowings, now: time.Now}
}

// NewPostgresService keeps infrastructure construction at the composition edge while the
// application itself depends only on the narrow reader/store interfaces above.
func NewPostgresService(projects projectReader, pool *pgxpool.Pool) *Service {
	return NewService(projects, NewPostgresRepository(pool), storyline.NewPostgresRepository(pool), material.NewPostgresRepository(pool), foreshadowing.NewPostgresRepository(pool))
}

func (s *Service) List(ctx context.Context, projectID uuid.UUID) ([]Plan, error) {
	if err := s.projectExists(ctx, projectID); err != nil {
		return nil, err
	}
	items, err := s.plans.ListByProject(ctx, projectID)
	if err != nil {
		return nil, mapError(err)
	}
	if items == nil {
		items = []Plan{}
	}
	return items, nil
}
func (s *Service) Get(ctx context.Context, id uuid.UUID) (Plan, error) {
	p, err := s.plans.GetByID(ctx, id)
	if err != nil {
		return Plan{}, mapError(err)
	}
	return p, nil
}

func (s *Service) GenerateMock(ctx context.Context, projectID uuid.UUID, c MockGenerateCommand) (MockGenerateResult, error) {
	if err := validateMock(c); err != nil {
		return MockGenerateResult{}, err
	}
	if err := s.projectExists(ctx, projectID); err != nil {
		return MockGenerateResult{}, err
	}
	lines, err := s.storylines.ListByProject(ctx, projectID)
	if err != nil {
		return MockGenerateResult{}, mapReferenceError(err, ErrStorylineReferenceInvalid)
	}
	target, ok := findStoryline(lines, c.TargetStorylineID)
	if !ok || target.ProjectID != projectID {
		return MockGenerateResult{}, ErrStorylineReferenceInvalid
	}
	existing, err := s.plans.ListByProject(ctx, projectID)
	if err != nil {
		return MockGenerateResult{}, mapError(err)
	}
	used := make(map[int]struct{}, len(existing))
	for _, p := range existing {
		used[p.ChapterNo] = struct{}{}
	}
	for n := c.StartChapterNo; n <= c.EndChapterNo; n++ {
		if _, exists := used[n]; exists {
			return MockGenerateResult{}, ErrChapterNoConflict
		}
	}

	refs := mockStorylines(lines, target, c)
	materials := []uuid.UUID{}
	if c.IncludeProjectMaterials {
		usages, err := s.materials.ListByProject(ctx, projectID)
		if err != nil {
			return MockGenerateResult{}, mapReferenceError(err, ErrMaterialReferenceInvalid)
		}
		for _, usage := range usages {
			materials = append(materials, usage.MaterialID)
		}
	}
	foreshadowings := []foreshadowing.Foreshadowing{}
	if c.IncludeUnpaidForeshadowings {
		values, err := s.foreshadowings.ListByProject(ctx, projectID)
		if err != nil {
			return MockGenerateResult{}, mapReferenceError(err, ErrForeshadowingReferenceInvalid)
		}
		for _, value := range values {
			if value.Status != "paid_off" {
				foreshadowings = append(foreshadowings, value)
			}
		}
	}
	// The formal command's inclusion switches are represented in deterministic output; no workflow or LLM is called.
	items := make([]Plan, 0, c.ChapterCount)
	for n := c.StartChapterNo; n <= c.EndChapterNo; n++ {
		p := Plan{ID: deterministicID(projectID, c, n), ProjectID: projectID, ChapterNo: n,
			Title: fmt.Sprintf("Chapter %d", n), Summary: mockSummary(n, c), Goal: stringPtr(fmt.Sprintf("Advance storyline %s in chapter %d.", target.Name, n)), Notes: cloneString(c.GenerationNotes),
			Status: "pending_confirmation", Source: "mock_generated", CreatedBy: c.ActorID,
			Storylines: slices.Clone(refs), Materials: slices.Clone(materials), Foreshadowings: applicableForeshadowings(foreshadowings, n)}
		items = append(items, p)
	}
	run := Run{ID: deterministicRunID(projectID, c), ProjectID: projectID, CreatedAt: s.now().UTC(), UpdatedAt: s.now().UTC()}
	if err := s.plans.SaveMock(ctx, run, items); err != nil {
		return MockGenerateResult{}, mapError(err)
	}
	persisted := make([]Plan, 0, len(items))
	for _, item := range items {
		stored, err := s.plans.GetByID(ctx, item.ID)
		if err != nil {
			return MockGenerateResult{}, mapError(err)
		}
		persisted = append(persisted, stored)
	}
	return MockGenerateResult{Run: run, Items: persisted}, nil
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, c UpdateCommand) (Plan, error) {
	if c.ExpectedVersion < 1 || !hasUpdate(c) {
		return Plan{}, ErrValidation
	}
	current, err := s.plans.GetByID(ctx, id)
	if err != nil {
		return Plan{}, mapError(err)
	}
	if current.Status != "pending_confirmation" {
		return Plan{}, ErrInvalidState
	}
	if current.Version != c.ExpectedVersion {
		return Plan{}, ErrVersionConflict
	}
	next := applyUpdate(current, c)
	if err := s.validatePlan(ctx, next); err != nil {
		return Plan{}, err
	}
	if samePlan(current, next) {
		return current, nil
	}
	updated, err := s.plans.Update(ctx, next, c.ExpectedVersion)
	if err != nil {
		return Plan{}, mapError(err)
	}
	return updated, nil
}
func (s *Service) Delete(ctx context.Context, id uuid.UUID, expectedVersion int) error {
	if expectedVersion < 1 {
		return ErrValidation
	}
	p, err := s.plans.GetByID(ctx, id)
	if err != nil {
		return mapError(err)
	}
	if p.Status != "pending_confirmation" {
		return ErrInvalidState
	}
	if p.Version != expectedVersion {
		return ErrVersionConflict
	}
	return mapError(s.plans.Delete(ctx, id, expectedVersion))
}

func (s *Service) Confirm(ctx context.Context, projectID uuid.UUID, selections []Selection) ([]Plan, error) {
	if len(selections) == 0 {
		return nil, ErrValidation
	}
	if err := s.projectExists(ctx, projectID); err != nil {
		return nil, err
	}
	seen := map[uuid.UUID]struct{}{}
	pending := make([]Selection, 0, len(selections))
	confirmed := make([]Plan, 0, len(selections))
	chapters := map[int]struct{}{}
	for _, selection := range selections {
		if selection.ID == uuid.Nil || selection.ExpectedVersion < 1 {
			return nil, ErrValidation
		}
		if _, duplicate := seen[selection.ID]; duplicate {
			return nil, ErrValidation
		}
		seen[selection.ID] = struct{}{}
		p, err := s.plans.GetByID(ctx, selection.ID)
		if err != nil {
			return nil, mapError(err)
		}
		if p.ProjectID != projectID {
			return nil, ErrProjectMismatch
		}
		if _, duplicate := chapters[p.ChapterNo]; duplicate {
			return nil, ErrChapterNoConflict
		}
		chapters[p.ChapterNo] = struct{}{}
		switch p.Status {
		case "pending_confirmation":
			if p.Version != selection.ExpectedVersion {
				return nil, ErrVersionConflict
			}
			if err := s.validatePlan(ctx, p); err != nil {
				return nil, err
			}
			pending = append(pending, selection)
		case "confirmed":
			if selection.ExpectedVersion != p.Version && selection.ExpectedVersion != p.Version-1 {
				return nil, ErrVersionConflict
			}
			confirmed = append(confirmed, p)
		default:
			return nil, ErrInvalidState
		}
	}
	if len(pending) == 0 {
		return orderSelected(selections, confirmed), nil
	}
	updated, err := s.plans.Confirm(ctx, pending)
	if err != nil {
		return nil, mapError(err)
	}
	return orderSelected(selections, append(confirmed, updated...)), nil
}

func (s *Service) projectExists(ctx context.Context, id uuid.UUID) error {
	_, err := s.projects.Get(ctx, id)
	if err != nil {
		return mapProjectError(err)
	}
	return nil
}
func (s *Service) validatePlan(ctx context.Context, p Plan) error {
	if p.ChapterNo < 1 || strings.TrimSpace(p.Title) == "" || len(p.Title) > 120 || len(p.Summary) > 5000 || (p.Goal != nil && len(*p.Goal) > 2000) || (p.Notes != nil && len(*p.Notes) > 2000) {
		return ErrValidation
	}
	primary := 0
	seenLines := map[uuid.UUID]struct{}{}
	for _, ref := range p.Storylines {
		if ref.ID == uuid.Nil || (ref.Relation != "primary" && ref.Relation != "secondary") {
			return ErrValidation
		}
		if _, ok := seenLines[ref.ID]; ok {
			return ErrValidation
		}
		seenLines[ref.ID] = struct{}{}
		if ref.Relation == "primary" {
			primary++
		}
		line, err := s.storylines.GetByID(ctx, ref.ID)
		if err != nil {
			return mapReferenceError(err, ErrStorylineReferenceInvalid)
		}
		if line.ProjectID != p.ProjectID {
			return ErrStorylineReferenceInvalid
		}
	}
	if primary != 1 {
		return ErrValidation
	}
	seenMaterials := map[uuid.UUID]struct{}{}
	for _, id := range p.Materials {
		if id == uuid.Nil {
			return ErrValidation
		}
		if _, ok := seenMaterials[id]; ok {
			return ErrValidation
		}
		seenMaterials[id] = struct{}{}
		if _, err := s.materials.GetByProjectAndMaterial(ctx, p.ProjectID, id); err != nil {
			return mapReferenceError(err, ErrMaterialReferenceInvalid)
		}
	}
	seenForeshadowings := map[uuid.UUID]struct{}{}
	for _, id := range p.Foreshadowings {
		if id == uuid.Nil {
			return ErrValidation
		}
		if _, ok := seenForeshadowings[id]; ok {
			return ErrValidation
		}
		seenForeshadowings[id] = struct{}{}
		f, err := s.foreshadowings.GetByID(ctx, id)
		if err != nil {
			return mapReferenceError(err, ErrForeshadowingReferenceInvalid)
		}
		if f.ProjectID != p.ProjectID || !foreshadowingApplies(f, p.ChapterNo) {
			return ErrForeshadowingReferenceInvalid
		}
	}
	return nil
}

func validateMock(c MockGenerateCommand) error {
	if c.TargetStorylineID == uuid.Nil || c.StartChapterNo < 1 || c.EndChapterNo < c.StartChapterNo || c.ChapterCount != c.EndChapterNo-c.StartChapterNo+1 || c.ChapterCount > 20 || (c.SummaryLength != "short" && c.SummaryLength != "medium" && c.SummaryLength != "long") || (c.ChapterPace != "slow" && c.ChapterPace != "balanced" && c.ChapterPace != "fast") || (c.GenerationNotes != nil && len(*c.GenerationNotes) > 2000) {
		return ErrValidation
	}
	return nil
}
func hasUpdate(c UpdateCommand) bool {
	return c.ChapterNo != nil || c.Title != nil || c.Summary != nil || c.Storylines.Set || c.Materials.Set || c.Foreshadowings.Set || c.Goal.Set || c.Notes.Set
}
func applyUpdate(p Plan, c UpdateCommand) Plan {
	n := p
	if c.ChapterNo != nil {
		n.ChapterNo = *c.ChapterNo
	}
	if c.Title != nil {
		n.Title = *c.Title
	}
	if c.Summary != nil {
		n.Summary = *c.Summary
	}
	if c.Storylines.Set {
		n.Storylines = slices.Clone(c.Storylines.Value)
	}
	if c.Materials.Set {
		n.Materials = slices.Clone(c.Materials.Value)
	}
	if c.Foreshadowings.Set {
		n.Foreshadowings = slices.Clone(c.Foreshadowings.Value)
	}
	if c.Goal.Set {
		n.Goal = cloneString(c.Goal.Value)
	}
	if c.Notes.Set {
		n.Notes = cloneString(c.Notes.Value)
	}
	return n
}
func samePlan(a, b Plan) bool {
	return a.ChapterNo == b.ChapterNo && a.Title == b.Title && a.Summary == b.Summary && sameString(a.Goal, b.Goal) && sameString(a.Notes, b.Notes) && slices.EqualFunc(a.Storylines, b.Storylines, func(x, y StorylineRef) bool { return x == y }) && slices.Equal(a.Materials, b.Materials) && slices.Equal(a.Foreshadowings, b.Foreshadowings)
}
func sameString(a, b *string) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}
func cloneString(v *string) *string {
	if v == nil {
		return nil
	}
	out := *v
	return &out
}
func stringPtr(v string) *string { return &v }
func foreshadowingApplies(f foreshadowing.Foreshadowing, chapter int) bool {
	return (f.PlannedPlantChapter == nil || *f.PlannedPlantChapter <= chapter) && (f.PlannedPayoffChapter == nil || chapter <= *f.PlannedPayoffChapter)
}
func findStoryline(lines []storyline.PlotLine, id uuid.UUID) (storyline.PlotLine, bool) {
	for _, l := range lines {
		if l.ID == id {
			return l, true
		}
	}
	return storyline.PlotLine{}, false
}
func mockStorylines(lines []storyline.PlotLine, target storyline.PlotLine, c MockGenerateCommand) []StorylineRef {
	refs := []StorylineRef{{ID: target.ID, Relation: "primary"}}
	seen := map[uuid.UUID]struct{}{target.ID: {}}
	add := func(l storyline.PlotLine) {
		if _, ok := seen[l.ID]; !ok {
			refs = append(refs, StorylineRef{ID: l.ID, Relation: "secondary"})
			seen[l.ID] = struct{}{}
		}
	}
	if c.IncludeMainStoryline {
		for _, l := range lines {
			if l.ParentID == nil {
				add(l)
				break
			}
		}
	}
	if c.IncludeChildStorylines {
		for _, l := range lines {
			if l.ParentID != nil && *l.ParentID == target.ID {
				add(l)
			}
		}
	}
	return refs
}
func mockSummary(n int, c MockGenerateCommand) string {
	return fmt.Sprintf("%s pace %s chapter %d; main=%t children=%t materials=%t unpaid_foreshadowings=%t prior_summaries=%t.", c.SummaryLength, c.ChapterPace, n, c.IncludeMainStoryline, c.IncludeChildStorylines, c.IncludeProjectMaterials, c.IncludeUnpaidForeshadowings, c.IncludePriorChapterSummaries)
}
func applicableForeshadowings(values []foreshadowing.Foreshadowing, chapter int) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(values))
	for _, value := range values {
		if foreshadowingApplies(value, chapter) {
			out = append(out, value.ID)
		}
	}
	return out
}
func deterministicID(projectID uuid.UUID, c MockGenerateCommand, n int) uuid.UUID {
	return uuid.NewSHA1(projectID, []byte(fmt.Sprintf("chapter-plan:%s:%d:%d:%d:%s:%s:%t:%t:%t:%t:%t:%v", c.TargetStorylineID, n, c.StartChapterNo, c.EndChapterNo, c.SummaryLength, c.ChapterPace, c.IncludeMainStoryline, c.IncludeChildStorylines, c.IncludeProjectMaterials, c.IncludeUnpaidForeshadowings, c.IncludePriorChapterSummaries, c.GenerationNotes)))
}
func deterministicRunID(projectID uuid.UUID, c MockGenerateCommand) uuid.UUID {
	return uuid.NewSHA1(projectID, []byte(fmt.Sprintf("mock-run:%s:%d:%d:%d:%s:%s:%t:%t:%t:%t:%t:%v", c.TargetStorylineID, c.StartChapterNo, c.EndChapterNo, c.ChapterCount, c.SummaryLength, c.ChapterPace, c.IncludeMainStoryline, c.IncludeChildStorylines, c.IncludeProjectMaterials, c.IncludeUnpaidForeshadowings, c.IncludePriorChapterSummaries, c.GenerationNotes)))
}
func orderSelected(s []Selection, plans []Plan) []Plan {
	byID := map[uuid.UUID]Plan{}
	for _, p := range plans {
		byID[p.ID] = p
	}
	out := make([]Plan, 0, len(s))
	for _, x := range s {
		out = append(out, byID[x.ID])
	}
	return out
}
func mapProjectError(err error) error {
	if errors.Is(err, project.ErrNotFound) {
		return ErrProjectNotFound
	}
	return ErrInternal
}
func mapReferenceError(err, errorKind error) error {
	if errors.Is(err, storyline.ErrNotFound) || errors.Is(err, material.ErrNotFound) || errors.Is(err, material.ErrUsageNotFound) || errors.Is(err, foreshadowing.ErrNotFound) || errors.Is(err, storyline.ErrProjectMismatch) || errors.Is(err, foreshadowing.ErrProjectMismatch) {
		return errorKind
	}
	return ErrInternal
}
func mapError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, ErrNotFound):
		return ErrChapterPlanNotFound
	case errors.Is(err, ErrVersionConflict):
		return ErrVersionConflict
	case errors.Is(err, ErrChapterNoConflict):
		return ErrChapterNoConflict
	case errors.Is(err, ErrInvalidReference):
		return ErrValidation
	case errors.Is(err, ErrProjectMismatch):
		return ErrProjectMismatch
	default:
		return ErrInternal
	}
}
