package storyline

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/local/ai-content-factory/apps/api/internal/audit"
	"github.com/local/ai-content-factory/apps/api/internal/project"
)

type applicationProjects struct{ values map[uuid.UUID]bool }

func (p applicationProjects) Get(_ context.Context, id uuid.UUID) (project.Project, error) {
	if !p.values[id] {
		return project.Project{}, project.ErrNotFound
	}
	return project.Project{ID: id}, nil
}

type applicationRepo struct {
	values      map[uuid.UUID]PlotLine
	createErr   error
	updateErr   error
	listErr     error
	createCalls int
	updateCalls int
}

func (r *applicationRepo) ListByProject(_ context.Context, projectID uuid.UUID) ([]PlotLine, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	values := make([]PlotLine, 0)
	for _, value := range r.values {
		if value.ProjectID == projectID {
			values = append(values, value)
		}
	}
	return values, nil
}
func (r *applicationRepo) GetByID(_ context.Context, id uuid.UUID) (PlotLine, error) {
	value, ok := r.values[id]
	if !ok {
		return PlotLine{}, ErrNotFound
	}
	return value, nil
}
func (r *applicationRepo) Create(_ context.Context, value PlotLine) (PlotLine, error) {
	r.createCalls++
	if r.createErr != nil {
		return PlotLine{}, r.createErr
	}
	value.Version = 1
	value.CreatedAt = time.Now().UTC()
	value.UpdatedAt = value.CreatedAt
	r.values[value.ID] = value
	return value, nil
}
func (r *applicationRepo) UpdateWithVersion(_ context.Context, value PlotLine, expected int) (PlotLine, error) {
	r.updateCalls++
	if r.updateErr != nil {
		return PlotLine{}, r.updateErr
	}
	current, ok := r.values[value.ID]
	if !ok {
		return PlotLine{}, ErrNotFound
	}
	if current.Version != expected {
		return PlotLine{}, ErrVersionConflict
	}
	value.Version = current.Version + 1
	value.CreatedAt = current.CreatedAt
	value.UpdatedAt = time.Now().UTC()
	r.values[value.ID] = value
	return value, nil
}
func (r *applicationRepo) ParentInProject(_ context.Context, id, projectID uuid.UUID) (bool, error) {
	value, ok := r.values[id]
	return ok && value.ProjectID == projectID, nil
}

type applicationAudit struct {
	entries []audit.Entry
	fail    bool
}

func (a *applicationAudit) Insert(_ context.Context, entry audit.Entry) error {
	if a.fail {
		return errors.New("audit failed")
	}
	a.entries = append(a.entries, entry)
	return nil
}

type applicationTx struct {
	repo  *applicationRepo
	audit *applicationAudit
}

func (t applicationTx) Run(ctx context.Context, fn func(Repository, AuditWriter) error) error {
	snapshot := make(map[uuid.UUID]PlotLine, len(t.repo.values))
	for id, value := range t.repo.values {
		snapshot[id] = value
	}
	entries := append([]audit.Entry(nil), t.audit.entries...)
	createCalls, updateCalls := t.repo.createCalls, t.repo.updateCalls
	if err := fn(t.repo, t.audit); err != nil {
		t.repo.values, t.audit.entries = snapshot, entries
		t.repo.createCalls, t.repo.updateCalls = createCalls, updateCalls
		return err
	}
	return nil
}

func appLine(projectID uuid.UUID, name string, start, end, order int) PlotLine {
	return PlotLine{ID: uuid.New(), ProjectID: projectID, Type: "main", Relation: "root", Name: name, Summary: "summary", Status: "active", StartChapter: intPtr(start), EndChapter: intPtr(end), SortOrder: order, CreatedBy: "seed", Version: 1}
}
func intPtr(value int) *int { return &value }
func appService(projectID uuid.UUID, values ...PlotLine) (*Service, *applicationRepo, *applicationAudit) {
	repo := &applicationRepo{values: map[uuid.UUID]PlotLine{}}
	for _, value := range values {
		repo.values[value.ID] = value
	}
	audit := &applicationAudit{}
	return NewService(applicationProjects{values: map[uuid.UUID]bool{projectID: true}}, repo, applicationTx{repo, audit}), repo, audit
}
func createCommand() CreateRootCommand {
	return CreateRootCommand{Name: "story", Summary: "summary", Status: "active", StartChapter: intPtr(1), EndChapter: intPtr(10), SortOrder: 1, ActorID: "actor"}
}

func TestApplicationGetTreeEmptyNestedAndStable(t *testing.T) {
	projectID := uuid.New()
	rootA := appLine(projectID, "a", 1, 10, 2)
	rootB := appLine(projectID, "b", 1, 10, 1)
	childB := appLine(projectID, "child-b", 2, 3, 2)
	childB.Type, childB.Relation, childB.ParentID = "child", "child", &rootB.ID
	childA := appLine(projectID, "child-a", 2, 3, 1)
	childA.Type, childA.Relation, childA.ParentID = "child", "child", &rootB.ID
	grandchild := appLine(projectID, "grandchild", 2, 2, 1)
	grandchild.Type, grandchild.Relation, grandchild.ParentID = "child", "child", &childA.ID
	service, _, _ := appService(projectID, rootA, rootB, childB, childA, grandchild)
	result, err := service.GetTree(context.Background(), projectID)
	if err != nil || len(result.Items) != 2 || result.Items[0].ID != rootB.ID || result.Items[0].Children[0].ID != childA.ID || result.Items[0].Children[0].Children[0].ID != grandchild.ID {
		t.Fatalf("tree=%#v err=%v", result, err)
	}
	emptyID := uuid.New()
	emptyService, _, _ := appService(emptyID)
	if empty, err := emptyService.GetTree(context.Background(), emptyID); err != nil || len(empty.Items) != 0 {
		t.Fatalf("empty=%#v err=%v", empty, err)
	}
}

func TestApplicationGetTreeRejectsMalformedGraphs(t *testing.T) {
	projectID := uuid.New()
	missing := appLine(projectID, "missing", 1, 2, 1)
	missing.Type, missing.Relation, missing.ParentID = "child", "child", uuidPtr(uuid.New())
	service, _, _ := appService(projectID, missing)
	if _, err := service.GetTree(context.Background(), projectID); !errors.Is(err, ErrMissingParent) {
		t.Fatalf("missing parent=%v", err)
	}
	self := appLine(projectID, "self", 1, 2, 1)
	self.Type, self.Relation, self.ParentID = "child", "child", &self.ID
	service, _, _ = appService(projectID, self)
	if _, err := service.GetTree(context.Background(), projectID); !errors.Is(err, ErrCycle) {
		t.Fatalf("self=%v", err)
	}
	a, b := appLine(projectID, "a", 1, 2, 1), appLine(projectID, "b", 1, 2, 1)
	a.Type, a.Relation, a.ParentID = "child", "child", &b.ID
	b.Type, b.Relation, b.ParentID = "child", "child", &a.ID
	service, _, _ = appService(projectID, a, b)
	if _, err := service.GetTree(context.Background(), projectID); !errors.Is(err, ErrCycle) {
		t.Fatalf("cycle=%v", err)
	}
}

func TestApplicationCreateRootChildAndAudit(t *testing.T) {
	projectID := uuid.New()
	service, repo, audit := appService(projectID)
	root, err := service.CreateRoot(context.Background(), projectID, createCommand())
	if err != nil || root.Type != "main" || root.Relation != "root" || root.ParentID != nil || len(audit.entries) != 1 || audit.entries[0].Action != "storyline.created" {
		t.Fatalf("root=%#v audit=%#v err=%v", root, audit.entries, err)
	}
	child, err := service.CreateChild(context.Background(), projectID, root.ID, createCommand())
	if err != nil || child.ParentID == nil || *child.ParentID != root.ID || child.Type != "child" || child.Relation != "child" || len(audit.entries) != 2 {
		t.Fatalf("child=%#v err=%v", child, err)
	}
	bad := createCommand()
	bad.Status = "bad"
	if _, err = service.CreateRoot(context.Background(), projectID, bad); !errors.Is(err, ErrValidation) {
		t.Fatalf("invalid root=%v", err)
	}
	if _, err = service.CreateChild(context.Background(), projectID, uuid.New(), createCommand()); !errors.Is(err, ErrParentNotFound) {
		t.Fatalf("missing child parent=%v", err)
	}
	other := appLine(uuid.New(), "other", 1, 10, 1)
	repo.values[other.ID] = other
	if _, err = service.CreateChild(context.Background(), projectID, other.ID, createCommand()); !errors.Is(err, ErrProjectMismatch) {
		t.Fatalf("cross project=%v", err)
	}
	outside := createCommand()
	outside.EndChapter = intPtr(11)
	if _, err = service.CreateChild(context.Background(), projectID, root.ID, outside); !errors.Is(err, ErrChildOutOfRange) {
		t.Fatalf("outside=%v", err)
	}
}

func TestApplicationUpdateIdempotencyBoundsAuditAndConflict(t *testing.T) {
	projectID := uuid.New()
	root := appLine(projectID, "root", 1, 10, 1)
	child := appLine(projectID, "child", 3, 8, 1)
	child.Type, child.Relation, child.ParentID = "child", "child", &root.ID
	grandchild := appLine(projectID, "grand", 4, 7, 1)
	grandchild.Type, grandchild.Relation, grandchild.ParentID = "child", "child", &child.ID
	service, repo, audit := appService(projectID, root, child, grandchild)
	name := "changed"
	updated, err := service.Update(context.Background(), root.ID, UpdateCommand{ExpectedVersion: 1, Name: &name, ActorID: "actor"})
	if err != nil || updated.Version != 2 || repo.updateCalls != 1 || len(audit.entries) != 1 || audit.entries[0].Action != "storyline.updated" {
		t.Fatalf("update=%#v calls=%d audits=%#v err=%v", updated, repo.updateCalls, audit.entries, err)
	}
	unchanged, err := service.Update(context.Background(), root.ID, UpdateCommand{ExpectedVersion: 2, Name: &name, ActorID: "actor"})
	if err != nil || unchanged.Version != 2 || repo.updateCalls != 1 || len(audit.entries) != 1 {
		t.Fatalf("same=%#v calls=%d audit=%d err=%v", unchanged, repo.updateCalls, len(audit.entries), err)
	}
	if _, err = service.Update(context.Background(), root.ID, UpdateCommand{ExpectedVersion: 1, Name: &name}); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("conflict=%v", err)
	}
	tooLate := OptionalInt{Set: true, Value: intPtr(11)}
	if _, err = service.Update(context.Background(), child.ID, UpdateCommand{ExpectedVersion: 1, EndChapter: tooLate}); !errors.Is(err, ErrChildOutOfRange) {
		t.Fatalf("child bounds=%v", err)
	}
	shrink := OptionalInt{Set: true, Value: intPtr(5)}
	if _, err = service.Update(context.Background(), root.ID, UpdateCommand{ExpectedVersion: 2, EndChapter: shrink}); !errors.Is(err, ErrDescendantOutOfRange) {
		t.Fatalf("descendant bounds=%v", err)
	}
}

func TestApplicationTransactionsRollbackOnRepositoryAndAuditFailure(t *testing.T) {
	projectID := uuid.New()
	service, repo, audit := appService(projectID)
	repo.createErr = errors.New("create failed")
	if _, err := service.CreateRoot(context.Background(), projectID, createCommand()); err == nil || len(repo.values) != 0 || len(audit.entries) != 0 {
		t.Fatalf("repository rollback err=%v", err)
	}
	repo.createErr = nil
	audit.fail = true
	if _, err := service.CreateRoot(context.Background(), projectID, createCommand()); err == nil || len(repo.values) != 0 || len(audit.entries) != 0 {
		t.Fatalf("audit rollback err=%v", err)
	}
	root := appLine(projectID, "root", 1, 10, 1)
	repo.values[root.ID] = root
	audit.fail = false
	repo.updateErr = errors.New("update failed")
	name := "updated"
	if _, err := service.Update(context.Background(), root.ID, UpdateCommand{ExpectedVersion: 1, Name: &name}); err == nil || repo.values[root.ID].Name != "root" {
		t.Fatalf("update rollback err=%v value=%#v", err, repo.values[root.ID])
	}

}

func TestApplicationUpdateWritesBeforeAfterAuditPayload(t *testing.T) {
	projectID := uuid.New()
	root := appLine(projectID, "before", 1, 10, 1)
	service, _, audits := appService(projectID, root)
	name := "after"
	if _, err := service.Update(context.Background(), root.ID, UpdateCommand{ExpectedVersion: 1, Name: &name, ActorID: "actor"}); err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Before *PlotLine `json:"before"`
		After  PlotLine  `json:"after"`
	}
	if len(audits.entries) != 1 || json.Unmarshal(audits.entries[0].Payload, &payload) != nil || payload.Before == nil || payload.Before.Name != "before" || payload.After.Name != "after" {
		t.Fatalf("audit=%#v payload=%s", audits.entries, audits.entries[0].Payload)
	}
}

func TestApplicationUpdateRejectsIndirectLegacyDescendantOutsideNewRange(t *testing.T) {
	projectID := uuid.New()
	root := appLine(projectID, "root", 1, 10, 1)
	child := appLine(projectID, "child", 3, 5, 1)
	child.Type, child.Relation, child.ParentID = "child", "child", &root.ID
	grandchild := appLine(projectID, "legacy-grandchild", 4, 7, 1)
	grandchild.Type, grandchild.Relation, grandchild.ParentID = "child", "child", &child.ID
	service, _, _ := appService(projectID, root, child, grandchild)
	end := OptionalInt{Set: true, Value: intPtr(6)}
	if _, err := service.Update(context.Background(), root.ID, UpdateCommand{ExpectedVersion: 1, EndChapter: end}); !errors.Is(err, ErrDescendantOutOfRange) {
		t.Fatalf("indirect descendant bounds=%v", err)
	}
}

func TestApplicationCreateChildForParentDerivesProject(t *testing.T) {
	projectID := uuid.New()
	parent := appLine(projectID, "parent", 1, 10, 1)
	service, _, _ := appService(projectID, parent)
	created, err := service.CreateChildForParent(context.Background(), parent.ID, createCommand())
	if err != nil || created.ProjectID != projectID || created.ParentID == nil || *created.ParentID != parent.ID {
		t.Fatalf("created=%#v err=%v", created, err)
	}
	if _, err := service.CreateChildForParent(context.Background(), uuid.New(), createCommand()); !errors.Is(err, ErrParentNotFound) {
		t.Fatalf("missing parent=%v", err)
	}
}
