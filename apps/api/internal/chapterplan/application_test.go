package chapterplan

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/local/ai-content-factory/apps/api/internal/foreshadowing"
	"github.com/local/ai-content-factory/apps/api/internal/material"
	"github.com/local/ai-content-factory/apps/api/internal/project"
	"github.com/local/ai-content-factory/apps/api/internal/storyline"
)

type fakeStore struct {
	plans                                           map[uuid.UUID]Plan
	saveRun                                         Run
	saved                                           []Plan
	saveErr, errorUpdate, errorDelete, errorConfirm error
	saves, updates, deletes, confirms               int
	update                                          Plan
	updateExpected                                  int
	deletedID                                       uuid.UUID
	deletedExpected                                 int
	confirmed                                       []Selection
}

func (f *fakeStore) ListByProject(_ context.Context, id uuid.UUID) ([]Plan, error) {
	var r []Plan
	for _, p := range f.plans {
		if p.ProjectID == id {
			r = append(r, p)
		}
	}
	return r, nil
}
func (f *fakeStore) GetByID(_ context.Context, id uuid.UUID) (Plan, error) {
	p, ok := f.plans[id]
	if !ok {
		return Plan{}, ErrNotFound
	}
	return p, nil
}
func (f *fakeStore) SaveMock(_ context.Context, r Run, p []Plan) error {
	f.saves++
	f.saveRun = r
	f.saved = slices.Clone(p)
	if f.saveErr != nil {
		return f.saveErr
	}
	for _, v := range p {
		v.Version = 1
		f.plans[v.ID] = v
	}
	return nil
}
func (f *fakeStore) Update(_ context.Context, p Plan, e int) (Plan, error) {
	f.updates++
	f.update = p
	f.updateExpected = e
	if f.errorUpdate != nil {
		return Plan{}, f.errorUpdate
	}
	p.Version++
	f.plans[p.ID] = p
	return p, nil
}
func (f *fakeStore) Delete(_ context.Context, id uuid.UUID, e int) error {
	f.deletes++
	f.deletedID = id
	f.deletedExpected = e
	if f.errorDelete != nil {
		return f.errorDelete
	}
	delete(f.plans, id)
	return nil
}
func (f *fakeStore) Confirm(_ context.Context, s []Selection) ([]Plan, error) {
	f.confirms++
	f.confirmed = slices.Clone(s)
	if f.errorConfirm != nil {
		return nil, f.errorConfirm
	}
	out := make([]Plan, 0, len(s))
	for _, x := range s {
		p := f.plans[x.ID]
		p.Status = "confirmed"
		p.Version++
		f.plans[p.ID] = p
		out = append(out, p)
	}
	return out, nil
}

type fakeProjects struct {
	known map[uuid.UUID]bool
	err   error
}

func (f fakeProjects) Get(_ context.Context, id uuid.UUID) (project.Project, error) {
	if f.err != nil {
		return project.Project{}, f.err
	}
	if !f.known[id] {
		return project.Project{}, project.ErrNotFound
	}
	return project.Project{ID: id}, nil
}

type fakeStorylines struct {
	lines map[uuid.UUID]storyline.PlotLine
	err   error
}

func (f fakeStorylines) GetByID(_ context.Context, id uuid.UUID) (storyline.PlotLine, error) {
	if f.err != nil {
		return storyline.PlotLine{}, f.err
	}
	p, ok := f.lines[id]
	if !ok {
		return storyline.PlotLine{}, storyline.ErrNotFound
	}
	return p, nil
}
func (f fakeStorylines) ListByProject(_ context.Context, id uuid.UUID) ([]storyline.PlotLine, error) {
	if f.err != nil {
		return nil, f.err
	}
	var r []storyline.PlotLine
	for _, p := range f.lines {
		if p.ProjectID == id {
			r = append(r, p)
		}
	}
	return r, nil
}

type fakeMaterials struct {
	owned map[uuid.UUID]uuid.UUID
	err   error
	calls int
}

func (f *fakeMaterials) GetByProjectAndMaterial(_ context.Context, p, id uuid.UUID) (material.ProjectMaterialUsage, error) {
	f.calls++
	if f.err != nil {
		return material.ProjectMaterialUsage{}, f.err
	}
	if f.owned[id] != p {
		return material.ProjectMaterialUsage{}, material.ErrUsageNotFound
	}
	return material.ProjectMaterialUsage{ProjectID: p, MaterialID: id}, nil
}
func (f *fakeMaterials) ListByProject(_ context.Context, p uuid.UUID) ([]material.ProjectMaterialUsage, error) {
	if f.err != nil {
		return nil, f.err
	}
	out := []material.ProjectMaterialUsage{}
	for id, projectID := range f.owned {
		if projectID == p {
			out = append(out, material.ProjectMaterialUsage{ProjectID: p, MaterialID: id})
		}
	}
	return out, nil
}

type fakeForeshadowings struct {
	values map[uuid.UUID]foreshadowing.Foreshadowing
	err    error
}

func (f fakeForeshadowings) GetByID(_ context.Context, id uuid.UUID) (foreshadowing.Foreshadowing, error) {
	if f.err != nil {
		return foreshadowing.Foreshadowing{}, f.err
	}
	v, ok := f.values[id]
	if !ok {
		return foreshadowing.Foreshadowing{}, foreshadowing.ErrNotFound
	}
	return v, nil
}
func (f fakeForeshadowings) ListByProject(_ context.Context, p uuid.UUID) ([]foreshadowing.Foreshadowing, error) {
	if f.err != nil {
		return nil, f.err
	}
	out := []foreshadowing.Foreshadowing{}
	for _, v := range f.values {
		if v.ProjectID == p {
			out = append(out, v)
		}
	}
	return out, nil
}
func fixtureService() (*Service, *fakeStore, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID) {
	p, line, mat, fo := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	store := &fakeStore{plans: map[uuid.UUID]Plan{}}
	materials := &fakeMaterials{owned: map[uuid.UUID]uuid.UUID{mat: p}}
	s := NewService(fakeProjects{known: map[uuid.UUID]bool{p: true}}, store, fakeStorylines{lines: map[uuid.UUID]storyline.PlotLine{line: {ID: line, ProjectID: p, Name: "Arc"}}}, materials, fakeForeshadowings{values: map[uuid.UUID]foreshadowing.Foreshadowing{fo: {ID: fo, ProjectID: p}}})
	return s, store, p, line, mat, fo
}
func pending(id, p, line, mat, fo uuid.UUID) Plan {
	return Plan{ID: id, ProjectID: p, ChapterNo: 1, Title: "one", Summary: "summary", Status: "pending_confirmation", Source: "mock_generated", Version: 1, Storylines: []StorylineRef{{ID: line, Relation: "primary"}}, Materials: []uuid.UUID{mat}, Foreshadowings: []uuid.UUID{fo}}
}

func TestApplicationListGetAndNotFound(t *testing.T) {
	s, st, p, line, mat, fo := fixtureService()
	a := pending(uuid.New(), p, line, mat, fo)
	st.plans[a.ID] = a
	got, e := s.List(context.Background(), p)
	if e != nil || len(got) != 1 {
		t.Fatalf("list=%v %v", got, e)
	}
	if _, e = s.Get(context.Background(), a.ID); e != nil {
		t.Fatal(e)
	}
	if _, e = s.List(context.Background(), uuid.New()); !errors.Is(e, ErrProjectNotFound) {
		t.Fatalf("%v", e)
	}
	if _, e = s.Get(context.Background(), uuid.New()); !errors.Is(e, ErrChapterPlanNotFound) {
		t.Fatalf("%v", e)
	}
}
func TestApplicationMockGenerationDeterministicAndAtomicFailure(t *testing.T) {
	s, st, p, line, _, _ := fixtureService()
	c := MockGenerateCommand{TargetStorylineID: line, StartChapterNo: 2, EndChapterNo: 3, ChapterCount: 2, IncludeProjectMaterials: true, IncludeUnpaidForeshadowings: true, SummaryLength: "short", ChapterPace: "balanced", ActorID: "a"}
	a, e := s.GenerateMock(context.Background(), p, c)
	if e != nil || len(a.Items) != 2 || st.saves != 1 {
		t.Fatalf("%+v %v", a, e)
	}
	bID := deterministicID(p, c, 2)
	if a.Items[0].ID != bID || a.Items[0].Status != "pending_confirmation" || a.Items[0].Source != "mock_generated" || len(st.saved[0].Materials) != 1 || len(st.saved[0].Foreshadowings) != 1 {
		t.Fatalf("not deterministic %+v", a.Items[0])
	}
	st.plans[uuid.New()] = Plan{ProjectID: p, ChapterNo: 4}
	c.StartChapterNo, c.EndChapterNo, c.ChapterCount = 4, 4, 1
	if _, e = s.GenerateMock(context.Background(), p, c); !errors.Is(e, ErrChapterNoConflict) || st.saves != 1 {
		t.Fatalf("%v saves=%d", e, st.saves)
	}
	c.StartChapterNo, c.EndChapterNo, c.ChapterCount = 5, 5, 1
	st.saveErr = errors.New("database secret")
	if _, e = s.GenerateMock(context.Background(), p, c); !errors.Is(e, ErrInternal) || len(st.plans) != 3 {
		t.Fatalf("%v plans=%d", e, len(st.plans))
	}
}
func TestApplicationUpdateTriStateReplacementAndGuards(t *testing.T) {
	s, st, p, line, mat, fo := fixtureService()
	id := uuid.New()
	a := pending(id, p, line, mat, fo)
	text := "text"
	a.Goal = &text
	st.plans[id] = a
	empty := ""
	got, e := s.Update(context.Background(), id, UpdateCommand{ExpectedVersion: 1, Goal: OptionalString{Set: true, Value: &empty}, Notes: OptionalString{Set: true, Value: nil}, Materials: OptionalUUIDs{Set: true, Value: []uuid.UUID{}}, Foreshadowings: OptionalUUIDs{Set: true, Value: []uuid.UUID{}}, Storylines: OptionalStorylines{Set: true, Value: []StorylineRef{{ID: line, Relation: "primary"}}}})
	if e != nil || got.Goal == nil || *got.Goal != "" || got.Notes != nil || st.updates != 1 || len(st.update.Materials) != 0 {
		t.Fatalf("%+v %#v %v", got, st.update, e)
	}
	a = got
	a.Status = "confirmed"
	st.plans[id] = a
	if _, e = s.Update(context.Background(), id, UpdateCommand{ExpectedVersion: a.Version, Title: stringPtr("x")}); !errors.Is(e, ErrInvalidState) {
		t.Fatal(e)
	}
	a.Status = "pending_confirmation"
	st.plans[id] = a
	if _, e = s.Update(context.Background(), id, UpdateCommand{ExpectedVersion: 1, Title: stringPtr("x")}); !errors.Is(e, ErrVersionConflict) {
		t.Fatal(e)
	}
}
func TestApplicationDeleteAndConfirmAtomicGuards(t *testing.T) {
	s, st, p, line, mat, fo := fixtureService()
	a, b := pending(uuid.New(), p, line, mat, fo), pending(uuid.New(), p, line, mat, fo)
	b.ChapterNo = 2
	st.plans[a.ID], st.plans[b.ID] = a, b
	if e := s.Delete(context.Background(), a.ID, 1); e != nil || st.deletes != 1 {
		t.Fatalf("%v", e)
	}
	a.Status = "confirmed"
	st.plans[a.ID] = a
	if e := s.Delete(context.Background(), a.ID, 1); !errors.Is(e, ErrInvalidState) {
		t.Fatal(e)
	}
	a.Status = "pending_confirmation"
	a.Version = 2
	st.plans[a.ID] = a
	if e := s.Delete(context.Background(), a.ID, 1); !errors.Is(e, ErrVersionConflict) {
		t.Fatal(e)
	}
	if _, e := s.Confirm(context.Background(), p, []Selection{{ID: a.ID, ExpectedVersion: 1}, {ID: b.ID, ExpectedVersion: 1}}); !errors.Is(e, ErrVersionConflict) || st.confirms != 0 {
		t.Fatalf("stale batch %v confirms=%d", e, st.confirms)
	}
	got, e := s.Confirm(context.Background(), p, []Selection{{ID: a.ID, ExpectedVersion: 2}, {ID: b.ID, ExpectedVersion: 1}})
	if e != nil || len(got) != 2 || st.confirms != 1 {
		t.Fatalf("%v %+v", e, got)
	}
	if _, e = s.Confirm(context.Background(), p, []Selection{{ID: a.ID, ExpectedVersion: 2}, {ID: b.ID, ExpectedVersion: 1}}); e != nil || st.confirms != 1 {
		t.Fatalf("retry %v confirms=%d", e, st.confirms)
	}
	other := pending(uuid.New(), uuid.New(), line, mat, fo)
	st.plans[other.ID] = other
	if _, e = s.Confirm(context.Background(), p, []Selection{{ID: other.ID, ExpectedVersion: 1}}); !errors.Is(e, ErrProjectMismatch) {
		t.Fatal(e)
	}
}
func TestApplicationConfirmRejectsBadReferenceAndRepositoryErrors(t *testing.T) {
	s, st, p, line, mat, fo := fixtureService()
	a := pending(uuid.New(), p, line, mat, fo)
	st.plans[a.ID] = a
	s.foreshadowings = fakeForeshadowings{values: map[uuid.UUID]foreshadowing.Foreshadowing{}}
	if _, e := s.Confirm(context.Background(), p, []Selection{{ID: a.ID, ExpectedVersion: 1}}); !errors.Is(e, ErrForeshadowingReferenceInvalid) || st.confirms != 0 {
		t.Fatalf("%v", e)
	}
	s.foreshadowings = fakeForeshadowings{values: map[uuid.UUID]foreshadowing.Foreshadowing{fo: {ID: fo, ProjectID: p}}}
	st.errorConfirm = errors.New("postgres password")
	if _, e := s.Confirm(context.Background(), p, []Selection{{ID: a.ID, ExpectedVersion: 1}}); !errors.Is(e, ErrInternal) {
		t.Fatal(e)
	}
}
