package foreshadowing

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/local/ai-content-factory/apps/api/internal/audit"
	"github.com/local/ai-content-factory/apps/api/internal/project"
	"github.com/local/ai-content-factory/apps/api/internal/storyline"
)

type applicationProjects struct{ values map[uuid.UUID]bool }

func (p applicationProjects) Get(_ context.Context, id uuid.UUID) (project.Project, error) {
	if !p.values[id] {
		return project.Project{}, project.ErrNotFound
	}
	return project.Project{ID: id}, nil
}

type applicationStorylines struct {
	values map[uuid.UUID]storyline.PlotLine
	err    error
}

func (s applicationStorylines) GetByID(_ context.Context, id uuid.UUID) (storyline.PlotLine, error) {
	if s.err != nil {
		return storyline.PlotLine{}, s.err
	}
	value, ok := s.values[id]
	if !ok {
		return storyline.PlotLine{}, storyline.ErrNotFound
	}
	return value, nil
}

type applicationRepo struct {
	values                   map[uuid.UUID]Foreshadowing
	listed                   []Foreshadowing
	createErr, updateErr     error
	createCalls, updateCalls int
}

func (r *applicationRepo) ListByProject(_ context.Context, projectID uuid.UUID) ([]Foreshadowing, error) {
	if r.listed != nil {
		return append([]Foreshadowing(nil), r.listed...), nil
	}
	result := make([]Foreshadowing, 0)
	for _, value := range r.values {
		if value.ProjectID == projectID {
			result = append(result, value)
		}
	}
	return result, nil
}
func (r *applicationRepo) GetByID(_ context.Context, id uuid.UUID) (Foreshadowing, error) {
	value, ok := r.values[id]
	if !ok {
		return Foreshadowing{}, ErrNotFound
	}
	return value, nil
}
func (r *applicationRepo) Create(_ context.Context, value Foreshadowing) (Foreshadowing, error) {
	r.createCalls++
	if r.createErr != nil {
		return Foreshadowing{}, r.createErr
	}
	value.Version = 1
	value.CreatedAt = time.Now().UTC()
	value.UpdatedAt = value.CreatedAt
	r.values[value.ID] = value
	return value, nil
}
func (r *applicationRepo) UpdateWithVersion(_ context.Context, value Foreshadowing, expected int) (Foreshadowing, error) {
	r.updateCalls++
	if r.updateErr != nil {
		return Foreshadowing{}, r.updateErr
	}
	current, ok := r.values[value.ID]
	if !ok {
		return Foreshadowing{}, ErrNotFound
	}
	if current.Version != expected {
		return Foreshadowing{}, ErrVersionConflict
	}
	value.Version = current.Version + 1
	value.CreatedAt = current.CreatedAt
	value.UpdatedAt = time.Now().UTC()
	r.values[value.ID] = value
	return value, nil
}
func (r *applicationRepo) ValidateReferences(context.Context, uuid.UUID, *uuid.UUID, *uuid.UUID) error {
	return nil
}

type applicationAudit struct {
	entries []audit.Entry
	fail    bool
}

func (a *applicationAudit) Insert(_ context.Context, value audit.Entry) error {
	if a.fail {
		return errors.New("audit failed")
	}
	a.entries = append(a.entries, value)
	return nil
}

type applicationTx struct {
	repo  *applicationRepo
	audit *applicationAudit
}

func (t applicationTx) Run(ctx context.Context, fn func(Repository, AuditWriter) error) error {
	snapshot := map[uuid.UUID]Foreshadowing{}
	for id, value := range t.repo.values {
		snapshot[id] = value
	}
	entries := append([]audit.Entry(nil), t.audit.entries...)
	creates, updates := t.repo.createCalls, t.repo.updateCalls
	if err := fn(t.repo, t.audit); err != nil {
		t.repo.values, t.audit.entries = snapshot, entries
		t.repo.createCalls, t.repo.updateCalls = creates, updates
		return err
	}
	return nil
}

func intPointer(value int) *int { return &value }
func line(projectID uuid.UUID) storyline.PlotLine {
	return storyline.PlotLine{ID: uuid.New(), ProjectID: projectID}
}
func command() CreateCommand {
	return CreateCommand{Title: "hint", Description: "description", Priority: "medium", Status: "planned", PlannedPlantChapter: intPointer(2), PlannedPayoffChapter: intPointer(5), ActorID: "actor"}
}
func app(projectID uuid.UUID, lines []storyline.PlotLine, values ...Foreshadowing) (*Service, *applicationRepo, *applicationAudit) {
	repo := &applicationRepo{values: map[uuid.UUID]Foreshadowing{}}
	for _, value := range values {
		repo.values[value.ID] = value
	}
	reader := applicationStorylines{values: map[uuid.UUID]storyline.PlotLine{}}
	for _, value := range lines {
		reader.values[value.ID] = value
	}
	audit := &applicationAudit{}
	return NewService(applicationProjects{values: map[uuid.UUID]bool{projectID: true}}, reader, repo, applicationTx{repo, audit}), repo, audit
}

func TestApplicationListValidatesProjectPreservesOrderAndNilReferences(t *testing.T) {
	projectID := uuid.New()
	first := Foreshadowing{ID: uuid.New(), ProjectID: projectID, Title: "first"}
	second := Foreshadowing{ID: uuid.New(), ProjectID: projectID, Title: "second"}
	service, repo, _ := app(projectID, nil)
	repo.listed = []Foreshadowing{first, second}
	result, err := service.List(context.Background(), projectID)
	if err != nil || len(result.Items) != 2 || result.Items[0].ID != first.ID || result.Items[0].PlantedPlotLineID != nil || result.Items[0].PayoffPlotLineID != nil {
		t.Fatalf("list=%#v err=%v", result, err)
	}
	emptyID := uuid.New()
	missing, _, _ := app(emptyID, nil)
	if _, err := missing.List(context.Background(), uuid.New()); !errors.Is(err, project.ErrNotFound) {
		t.Fatalf("project error=%v", err)
	}
}

func TestApplicationCreateReferencesValidationAndAudit(t *testing.T) {
	projectID := uuid.New()
	planted, payoff := line(projectID), line(projectID)
	service, _, audits := app(projectID, []storyline.PlotLine{planted, payoff})
	for _, ids := range []struct {
		name            string
		planted, payoff *uuid.UUID
	}{{"nil", nil, nil}, {"plant", &planted.ID, nil}, {"payoff", nil, &payoff.ID}, {"different", &planted.ID, &payoff.ID}} {
		c := command()
		c.PlantedPlotLineID, c.PayoffPlotLineID = ids.planted, ids.payoff
		created, err := service.Create(context.Background(), projectID, c)
		if err != nil || created.Version != 1 {
			t.Fatalf("create %s=%#v err=%v", ids.name, created, err)
		}
	}
	if len(audits.entries) != 4 || audits.entries[0].Action != "foreshadowing.created" {
		t.Fatalf("audits=%#v", audits.entries)
	}
	missing := command()
	missing.PlantedPlotLineID = pointer(uuid.New())
	if _, err := service.Create(context.Background(), projectID, missing); !errors.Is(err, ErrStorylineNotFound) {
		t.Fatalf("missing=%v", err)
	}
	cross := line(uuid.New())
	service, _, _ = app(projectID, []storyline.PlotLine{cross})
	bad := command()
	bad.PlantedPlotLineID = &cross.ID
	if _, err := service.Create(context.Background(), projectID, bad); !errors.Is(err, ErrProjectMismatch) {
		t.Fatalf("cross=%v", err)
	}
	service, _, _ = app(projectID, nil)
	bad = command()
	bad.Priority = "urgent"
	if _, err := service.Create(context.Background(), projectID, bad); !errors.Is(err, ErrInvalidPriority) {
		t.Fatalf("priority=%v", err)
	}
	bad = command()
	bad.Status = "unknown"
	if _, err := service.Create(context.Background(), projectID, bad); !errors.Is(err, ErrInvalidStatus) {
		t.Fatalf("status=%v", err)
	}
	bad = command()
	bad.PlannedPlantChapter = intPointer(6)
	bad.PlannedPayoffChapter = intPointer(5)
	if _, err := service.Create(context.Background(), projectID, bad); !errors.Is(err, ErrChapterRange) {
		t.Fatalf("range=%v", err)
	}
}

func TestApplicationCreateRollsBackRepositoryAndAudit(t *testing.T) {
	projectID := uuid.New()
	service, repo, audits := app(projectID, nil)
	repo.createErr = errors.New("create failed")
	if _, err := service.Create(context.Background(), projectID, command()); err == nil || len(repo.values) != 0 || len(audits.entries) != 0 {
		t.Fatalf("repo rollback=%v", err)
	}
	repo.createErr = nil
	audits.fail = true
	if _, err := service.Create(context.Background(), projectID, command()); err == nil || len(repo.values) != 0 || len(audits.entries) != 0 {
		t.Fatalf("audit rollback=%v", err)
	}
}

func TestApplicationUpdateNullableReferencesTransitionsAndAudit(t *testing.T) {
	projectID := uuid.New()
	one, two := line(projectID), line(projectID)
	value := Foreshadowing{ID: uuid.New(), ProjectID: projectID, Title: "before", Description: "d", Priority: "low", Status: "planned", PlantedPlotLineID: &one.ID, PlannedPlantChapter: intPointer(1), PlannedPayoffChapter: intPointer(2), Version: 1}
	service, repo, audits := app(projectID, []storyline.PlotLine{one, two}, value)
	title := "after"
	updated, err := service.Update(context.Background(), value.ID, UpdateCommand{ExpectedVersion: 1, Title: &title, ActorID: "actor"})
	if err != nil || updated.Version != 2 || repo.updateCalls != 1 {
		t.Fatalf("update=%#v err=%v", updated, err)
	}
	same, err := service.Update(context.Background(), value.ID, UpdateCommand{ExpectedVersion: 2, Title: &title})
	if err != nil || same.Version != 2 || repo.updateCalls != 1 || len(audits.entries) != 1 {
		t.Fatalf("same=%#v err=%v calls=%d audits=%d", same, err, repo.updateCalls, len(audits.entries))
	}
	updated, err = service.Update(context.Background(), value.ID, UpdateCommand{ExpectedVersion: 2, PayoffPlotLineID: OptionalUUID{Set: true, Value: &two.ID}})
	if err != nil || updated.PayoffPlotLineID == nil || *updated.PayoffPlotLineID != two.ID {
		t.Fatalf("set ref=%#v err=%v", updated, err)
	}
	updated, err = service.Update(context.Background(), value.ID, UpdateCommand{ExpectedVersion: 3, PlantedPlotLineID: OptionalUUID{Set: true, Value: nil}, PayoffPlotLineID: OptionalUUID{Set: true, Value: nil}})
	if err != nil || updated.PlantedPlotLineID != nil || updated.PayoffPlotLineID != nil {
		t.Fatalf("clear refs=%#v err=%v", updated, err)
	}
	status := "planted"
	updated, err = service.Update(context.Background(), value.ID, UpdateCommand{ExpectedVersion: 4, Status: &status})
	if err != nil || updated.Status != "planted" {
		t.Fatalf("planned->planted=%#v err=%v", updated, err)
	}
	status = "paid_off"
	updated, err = service.Update(context.Background(), value.ID, UpdateCommand{ExpectedVersion: 5, Status: &status})
	if err != nil || updated.Status != "paid_off" {
		t.Fatalf("planted->paid_off=%#v err=%v", updated, err)
	}
	var payload struct {
		Before *Foreshadowing `json:"before"`
		After  Foreshadowing  `json:"after"`
	}
	if len(audits.entries) != 5 || json.Unmarshal(audits.entries[0].Payload, &payload) != nil || payload.Before == nil || payload.Before.Title != "before" || payload.After.Title != "after" {
		t.Fatalf("audit=%#v payload=%s", audits.entries, audits.entries[0].Payload)
	}
}

func TestApplicationUpdateRejectsInvalidInputReferencesTransitionsAndRollsBack(t *testing.T) {
	projectID := uuid.New()
	value := Foreshadowing{ID: uuid.New(), ProjectID: projectID, Title: "hint", Priority: "low", Status: "planned", Version: 1}
	service, repo, audits := app(projectID, nil, value)
	missing := uuid.New()
	if _, err := service.Update(context.Background(), value.ID, UpdateCommand{ExpectedVersion: 1, PlantedPlotLineID: OptionalUUID{Set: true, Value: &missing}}); !errors.Is(err, ErrStorylineNotFound) {
		t.Fatalf("missing=%v", err)
	}
	cross := line(uuid.New())
	service, repo, audits = app(projectID, []storyline.PlotLine{cross}, value)
	if _, err := service.Update(context.Background(), value.ID, UpdateCommand{ExpectedVersion: 1, PlantedPlotLineID: OptionalUUID{Set: true, Value: &cross.ID}}); !errors.Is(err, ErrProjectMismatch) {
		t.Fatalf("cross=%v", err)
	}
	status := "paid_off"
	if _, err := service.Update(context.Background(), value.ID, UpdateCommand{ExpectedVersion: 1, Status: &status}); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("planned->paid_off=%v", err)
	}
	value.Status = "planted"
	value.Version = 2
	repo.values[value.ID] = value
	status = "planned"
	if _, err := service.Update(context.Background(), value.ID, UpdateCommand{ExpectedVersion: 2, Status: &status}); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("planted->planned=%v", err)
	}
	value.Status = "paid_off"
	value.Version = 3
	repo.values[value.ID] = value
	for _, target := range []string{"planned", "planted"} {
		status = target
		if _, err := service.Update(context.Background(), value.ID, UpdateCommand{ExpectedVersion: 3, Status: &status}); !errors.Is(err, ErrInvalidTransition) {
			t.Fatalf("paid_off->%s=%v", target, err)
		}
	}
	late := OptionalInt{Set: true, Value: intPointer(3)}
	early := OptionalInt{Set: true, Value: intPointer(2)}
	if _, err := service.Update(context.Background(), value.ID, UpdateCommand{ExpectedVersion: 3, PlannedPlantChapter: late, PlannedPayoffChapter: early}); !errors.Is(err, ErrChapterRange) {
		t.Fatalf("range=%v", err)
	}
	name := "new"
	if _, err := service.Update(context.Background(), value.ID, UpdateCommand{ExpectedVersion: 1, Title: &name}); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("conflict=%v", err)
	}
	repo.updateErr = errors.New("update failed")
	if _, err := service.Update(context.Background(), value.ID, UpdateCommand{ExpectedVersion: 3, Title: &name}); err == nil || repo.values[value.ID].Title != "hint" {
		t.Fatalf("repo rollback=%v value=%#v", err, repo.values[value.ID])
	}
	repo.updateErr = nil
	audits.fail = true
	if _, err := service.Update(context.Background(), value.ID, UpdateCommand{ExpectedVersion: 3, Title: &name}); err == nil || repo.values[value.ID].Title != "hint" || len(audits.entries) != 0 {
		t.Fatalf("audit rollback=%v value=%#v", err, repo.values[value.ID])
	}
}

func pointer(value uuid.UUID) *uuid.UUID { return &value }

func TestApplicationCreateWritesAuditPayload(t *testing.T) {
	projectID := uuid.New()
	service, _, audits := app(projectID, nil)
	created, err := service.Create(context.Background(), projectID, command())
	if err != nil || len(audits.entries) != 1 {
		t.Fatalf("create=%#v audits=%#v err=%v", created, audits.entries, err)
	}
	var payload struct {
		Before *Foreshadowing `json:"before"`
		After  Foreshadowing  `json:"after"`
	}
	if err := json.Unmarshal(audits.entries[0].Payload, &payload); err != nil || payload.Before != nil || payload.After.ID != created.ID || audits.entries[0].SubjectID != created.ID.String() {
		t.Fatalf("audit payload=%s err=%v", audits.entries[0].Payload, err)
	}
}
