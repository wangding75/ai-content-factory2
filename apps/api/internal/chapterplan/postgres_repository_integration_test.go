package chapterplan

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const integrationDatabase = "ai_content_factory"

func openIntegrationDB(t *testing.T) (*pgxpool.Pool, context.Context) {
	t.Helper()
	u := os.Getenv("DATABASE_URL")
	if u == "" {
		t.Fatal("DATABASE_URL is not set; PostgreSQL integration test is required")
	}
	cfg, err := pgxpool.ParseConfig(u)
	if err != nil {
		t.Fatalf("parse DATABASE_URL: %v", err)
	}
	if cfg.ConnConfig.Database != integrationDatabase {
		t.Fatalf("DATABASE_URL targets database %q, not %q; PostgreSQL integration test is required", cfg.ConnConfig.Database, integrationDatabase)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	db, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("connect PostgreSQL: %v", err)
	}
	t.Cleanup(db.Close)
	if err = db.Ping(ctx); err != nil {
		t.Fatalf("ping PostgreSQL: %v", err)
	}
	return db, ctx
}

type fixture struct {
	project, otherProject uuid.UUID
	storylines            []uuid.UUID
	materials             []uuid.UUID
	foreshadowings        []uuid.UUID
}

func newFixture(t *testing.T, ctx context.Context, db *pgxpool.Pool) fixture {
	t.Helper()
	f := fixture{project: uuid.New(), otherProject: uuid.New()}
	for i := 0; i < 3; i++ {
		f.storylines = append(f.storylines, uuid.New())
		f.materials = append(f.materials, uuid.New())
		f.foreshadowings = append(f.foreshadowings, uuid.New())
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	for _, project := range []uuid.UUID{f.project, f.otherProject} {
		if _, err = tx.Exec(ctx, "INSERT INTO projects(id,name,type,created_by) VALUES($1,$2,'novel','i05')", project, "i05-"+project.String()); err != nil {
			t.Fatal(err)
		}
	}
	for i := range f.storylines {
		if _, err = tx.Exec(ctx, "INSERT INTO storylines(id,project_id,type,relation,name,status,sort_order,created_by) VALUES($1,$2,'main','root',$3,'active',$4,'i05')", f.storylines[i], f.project, "storyline", i); err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(ctx, "INSERT INTO materials(id,type,name,created_by) VALUES($1,'reference',$2,'i05')", f.materials[i], "material"); err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(ctx, "INSERT INTO project_material_usages(id,project_id,material_id,usage_type,created_by) VALUES($1,$2,$3,'reference','i05')", uuid.New(), f.project, f.materials[i]); err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(ctx, "INSERT INTO foreshadowings(id,project_id,title,priority,status,created_by) VALUES($1,$2,$3,'medium','planned','i05')", f.foreshadowings[i], f.project, "foreshadowing"); err != nil {
			t.Fatal(err)
		}
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec(context.Background(), "DELETE FROM projects WHERE id=$1 OR id=$2", f.project, f.otherProject)
	})
	return f
}

func plan(f fixture, chapter int) Plan {
	return Plan{ID: uuid.New(), ProjectID: f.project, ChapterNo: chapter, Title: "chapter", Summary: "summary", CreatedBy: "i05"}
}

func save(t *testing.T, ctx context.Context, r *Repository, p Plan) {
	t.Helper()
	if err := r.SaveMock(ctx, Run{ID: uuid.New(), ProjectID: p.ProjectID}, []Plan{p}); err != nil {
		t.Fatalf("SaveMock: %v", err)
	}
}

func rawNullable(t *testing.T, ctx context.Context, db *pgxpool.Pool, id uuid.UUID) (goal, notes *string) {
	t.Helper()
	if err := db.QueryRow(ctx, "SELECT chapter_goal,creation_notes FROM chapter_plans WHERE id=$1", id).Scan(&goal, &notes); err != nil {
		t.Fatal(err)
	}
	return goal, notes
}

func requireStringPtr(t *testing.T, label string, got *string, want *string) {
	t.Helper()
	if (got == nil) != (want == nil) {
		t.Fatalf("%s nil=%t, want nil=%t", label, got == nil, want == nil)
	}
	if got != nil && *got != *want {
		t.Fatalf("%s=%q, want %q", label, *got, *want)
	}
}

func mustNewRepo(t *testing.T, db *pgxpool.Pool) *Repository {
	t.Helper()
	repo, err := NewPostgresRepository(db, "test-hmac-secret-1234567890")
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}
	return repo
}

func TestPostgresRepositoryNullableTriState(t *testing.T) {
	db, ctx := openIntegrationDB(t)
	f := newFixture(t, ctx, db)
	r := mustNewRepo(t, db)
	text := "ordinary text"
	empty := ""
	cases := []struct {
		name        string
		goal, notes *string
	}{
		{"null", nil, nil},
		{"empty", &empty, &empty},
		{"text", &text, &text},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := plan(f, i+1)
			p.Goal, p.Notes = tc.goal, tc.notes
			save(t, ctx, r, p)
			got, err := r.GetByID(ctx, p.ID)
			if err != nil {
				t.Fatal(err)
			}
			rawGoal, rawNotes := rawNullable(t, ctx, db, p.ID)
			requireStringPtr(t, "repository goal", got.Goal, tc.goal)
			requireStringPtr(t, "repository notes", got.Notes, tc.notes)
			requireStringPtr(t, "database goal", rawGoal, tc.goal)
			requireStringPtr(t, "database notes", rawNotes, tc.notes)
		})
	}
	p := plan(f, 4)
	p.Goal, p.Notes = &text, &text
	save(t, ctx, r, p)
	p.Goal, p.Notes = &empty, &empty
	if _, err := r.Update(ctx, p, 1); err != nil {
		t.Fatal(err)
	}
	got, err := r.GetByID(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	rawGoal, rawNotes := rawNullable(t, ctx, db, p.ID)
	requireStringPtr(t, "updated repository goal", got.Goal, &empty)
	requireStringPtr(t, "updated repository notes", got.Notes, &empty)
	requireStringPtr(t, "updated database goal", rawGoal, &empty)
	requireStringPtr(t, "updated database notes", rawNotes, &empty)
	p.Goal, p.Notes = nil, nil
	if _, err = r.Update(ctx, p, 2); err != nil {
		t.Fatal(err)
	}
	got, err = r.GetByID(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	rawGoal, rawNotes = rawNullable(t, ctx, db, p.ID)
	requireStringPtr(t, "null repository goal", got.Goal, nil)
	requireStringPtr(t, "null repository notes", got.Notes, nil)
	requireStringPtr(t, "null database goal", rawGoal, nil)
	requireStringPtr(t, "null database notes", rawNotes, nil)
}

func TestRepositorySafelyMapsLegacyManualSource(t *testing.T) {
	db, ctx := openIntegrationDB(t)
	f := newFixture(t, ctx, db)
	id := uuid.New()
	if _, err := db.Exec(ctx, "INSERT INTO chapter_plans(id,project_id,chapter_no,title,summary,status,source,created_by) VALUES($1,$2,1,'legacy chapter','legacy summary','pending_confirmation','manual','legacy-importer')", id, f.project); err != nil {
		t.Fatal(err)
	}
	repo := mustNewRepo(t, db)
	got, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("read legacy chapter plan: %v", err)
	}
	if got.ID != id || got.ProjectID != f.project || got.Source != "manual" || got.Status != "pending_confirmation" {
		t.Fatalf("legacy plan=%+v", got)
	}
	items, err := repo.ListByProject(ctx, f.project)
	if err != nil || len(items) != 1 || items[0].ID != id || items[0].Source != "manual" {
		t.Fatalf("legacy list=%+v err=%v", items, err)
	}
}

func associationIDs(t *testing.T, ctx context.Context, db *pgxpool.Pool, table, column string, planID uuid.UUID) []uuid.UUID {
	t.Helper()
	rows, err := db.Query(ctx, "SELECT "+column+" FROM "+table+" WHERE chapter_plan_id=$1 ORDER BY position", planID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err = rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	return ids
}

func TestPostgresRepositoryAssociationsReplaceClearAndRejectCrossProject(t *testing.T) {
	db, ctx := openIntegrationDB(t)
	f := newFixture(t, ctx, db)
	r := mustNewRepo(t, db)
	p := plan(f, 1)
	p.Storylines = []StorylineRef{{ID: f.storylines[2], Relation: "secondary"}, {ID: f.storylines[0], Relation: "primary"}}
	p.Materials = []uuid.UUID{f.materials[2], f.materials[0]}
	p.Foreshadowings = []uuid.UUID{f.foreshadowings[2], f.foreshadowings[0]}
	save(t, ctx, r, p)
	got, err := r.GetByID(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal([]uuid.UUID{got.Storylines[0].ID, got.Storylines[1].ID}, []uuid.UUID{f.storylines[2], f.storylines[0]}) || got.Storylines[0].Relation != "secondary" || got.Storylines[1].Relation != "primary" {
		t.Fatalf("storylines=%+v", got.Storylines)
	}
	if !slices.Equal(got.Materials, p.Materials) || !slices.Equal(got.Foreshadowings, p.Foreshadowings) {
		t.Fatalf("repository associations=%+v", got)
	}
	if !slices.Equal(associationIDs(t, ctx, db, "chapter_plan_storylines", "storyline_id", p.ID), []uuid.UUID{f.storylines[2], f.storylines[0]}) || !slices.Equal(associationIDs(t, ctx, db, "chapter_plan_materials", "material_id", p.ID), p.Materials) || !slices.Equal(associationIDs(t, ctx, db, "chapter_plan_foreshadowings", "foreshadowing_id", p.ID), p.Foreshadowings) {
		t.Fatal("database association positions do not match input order")
	}
	p.Storylines = []StorylineRef{{ID: f.storylines[1], Relation: "primary"}, {ID: f.storylines[0], Relation: "secondary"}}
	p.Materials = []uuid.UUID{f.materials[1], f.materials[0]}
	p.Foreshadowings = []uuid.UUID{f.foreshadowings[1], f.foreshadowings[0]}
	if _, err = r.Update(ctx, p, 1); err != nil {
		t.Fatal(err)
	}
	got, err = r.GetByID(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal([]uuid.UUID{got.Storylines[0].ID, got.Storylines[1].ID}, []uuid.UUID{f.storylines[1], f.storylines[0]}) || !slices.Equal(got.Materials, p.Materials) || !slices.Equal(got.Foreshadowings, p.Foreshadowings) {
		t.Fatalf("replacement associations=%+v", got)
	}
	if !slices.Equal(associationIDs(t, ctx, db, "chapter_plan_storylines", "storyline_id", p.ID), []uuid.UUID{f.storylines[1], f.storylines[0]}) || !slices.Equal(associationIDs(t, ctx, db, "chapter_plan_materials", "material_id", p.ID), p.Materials) || !slices.Equal(associationIDs(t, ctx, db, "chapter_plan_foreshadowings", "foreshadowing_id", p.ID), p.Foreshadowings) {
		t.Fatal("replacement database positions do not match input order")
	}
	p.Storylines = []StorylineRef{}
	p.Materials = []uuid.UUID{}
	p.Foreshadowings = []uuid.UUID{}
	if _, err = r.Update(ctx, p, 2); err != nil {
		t.Fatal(err)
	}
	got, err = r.GetByID(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Storylines) != 0 || len(got.Materials) != 0 || len(got.Foreshadowings) != 0 {
		t.Fatalf("empty replacement did not clear: %+v", got)
	}
	otherStoryline, otherMaterial, otherForeshadowing := uuid.New(), uuid.New(), uuid.New()
	if _, err = db.Exec(ctx, "INSERT INTO storylines(id,project_id,type,relation,name,status,sort_order,created_by) VALUES($1,$2,'main','root','other','active',0,'i05')", otherStoryline, f.otherProject); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, "INSERT INTO materials(id,type,name,created_by) VALUES($1,'reference','other','i05')", otherMaterial); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, "INSERT INTO project_material_usages(id,project_id,material_id,usage_type,created_by) VALUES($1,$2,$3,'reference','i05')", uuid.New(), f.otherProject, otherMaterial); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, "INSERT INTO foreshadowings(id,project_id,title,priority,status,created_by) VALUES($1,$2,'other','medium','planned','i05')", otherForeshadowing, f.otherProject); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		set  func(*Plan)
	}{
		{"storyline", func(p *Plan) { p.Storylines = []StorylineRef{{ID: otherStoryline, Relation: "primary"}} }},
		{"material", func(p *Plan) { p.Materials = []uuid.UUID{otherMaterial} }},
		{"foreshadowing", func(p *Plan) { p.Foreshadowings = []uuid.UUID{otherForeshadowing} }},
	} {
		t.Run("cross-project-"+tc.name, func(t *testing.T) {
			p.Storylines, p.Materials, p.Foreshadowings = nil, nil, nil
			tc.set(&p)
			if _, err = r.Update(ctx, p, 3); !errors.Is(err, ErrInvalidReference) {
				t.Fatalf("cross-project association error=%v", err)
			}
			got, err = r.GetByID(ctx, p.ID)
			if err != nil {
				t.Fatal(err)
			}
			if len(got.Storylines) != 0 || len(got.Materials) != 0 || len(got.Foreshadowings) != 0 || got.Version != 3 {
				t.Fatalf("failed replacement partially modified plan: %+v", got)
			}
		})
	}
}

func TestPostgresRepositoryRejectsDuplicateAssociationsAtomically(t *testing.T) {
	db, ctx := openIntegrationDB(t)
	f := newFixture(t, ctx, db)
	r := mustNewRepo(t, db)
	cases := []struct {
		name string
		set  func(*Plan)
	}{
		{"storyline", func(p *Plan) {
			p.Storylines = []StorylineRef{{ID: f.storylines[0], Relation: "primary"}, {ID: f.storylines[0], Relation: "secondary"}}
		}},
		{"material", func(p *Plan) { p.Materials = []uuid.UUID{f.materials[0], f.materials[0]} }},
		{"foreshadowing", func(p *Plan) { p.Foreshadowings = []uuid.UUID{f.foreshadowings[0], f.foreshadowings[0]} }},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := plan(f, i+1)
			tc.set(&p)
			err := r.SaveMock(ctx, Run{ID: uuid.New(), ProjectID: f.project}, []Plan{p})
			if !errors.Is(err, ErrInvalidReference) || strings.Contains(strings.ToLower(err.Error()), "sql") {
				t.Fatalf("duplicate error=%v", err)
			}
			if _, err = r.GetByID(ctx, p.ID); !errors.Is(err, ErrNotFound) {
				t.Fatalf("duplicate write was not rolled back: %v", err)
			}
		})
	}
}

func TestPostgresRepositoryConfirmBatchRollsBackOnFailure(t *testing.T) {
	db, ctx := openIntegrationDB(t)
	f := newFixture(t, ctx, db)
	r := mustNewRepo(t, db)
	for _, tc := range []struct {
		name      string
		selection func(Plan, Plan) []Selection
	}{
		{"wrong-version", func(a, b Plan) []Selection {
			return []Selection{{ID: a.ID, ExpectedVersion: 1}, {ID: b.ID, ExpectedVersion: 99}}
		}},
		{"missing", func(a, b Plan) []Selection {
			return []Selection{{ID: a.ID, ExpectedVersion: 1}, {ID: uuid.New(), ExpectedVersion: 1}}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, b := plan(f, int(time.Now().UnixNano()%100000)+1), plan(f, int(time.Now().UnixNano()%100000)+2)
			save(t, ctx, r, a)
			save(t, ctx, r, b)
			if _, err := r.Confirm(ctx, tc.selection(a, b)); !errors.Is(err, ErrVersionConflict) {
				t.Fatalf("Confirm error=%v", err)
			}
			for _, id := range []uuid.UUID{a.ID, b.ID} {
				got, err := r.GetByID(ctx, id)
				if err != nil {
					t.Fatal(err)
				}
				if got.Status != "pending_confirmation" || got.Version != 1 || got.ConfirmedAt != nil {
					t.Fatalf("batch failure changed plan: %+v", got)
				}
			}
		})
	}
}

func TestPostgresRepositoryPersistsAcrossReconnect(t *testing.T) {
	db, ctx := openIntegrationDB(t)
	f := newFixture(t, ctx, db)
	r := mustNewRepo(t, db)
	goal, notes := "goal", "notes"
	p := plan(f, 1)
	p.Goal, p.Notes = &goal, &notes
	p.Storylines = []StorylineRef{{ID: f.storylines[1], Relation: "primary"}, {ID: f.storylines[0], Relation: "secondary"}}
	p.Materials = []uuid.UUID{f.materials[1], f.materials[0]}
	p.Foreshadowings = []uuid.UUID{f.foreshadowings[1], f.foreshadowings[0]}
	run := Run{ID: uuid.New(), ProjectID: f.project}
	if err := r.SaveMock(ctx, run, []Plan{p}); err != nil {
		t.Fatal(err)
	}
	cfg := db.Config().Copy()
	db.Close()
	reconnected, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(reconnected.Close)
	got, err := mustNewRepo(t, reconnected).GetByID(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.RunID != run.ID || got.Status != "pending_confirmation" || got.Version != 1 {
		t.Fatalf("persisted plan=%+v", got)
	}
	requireStringPtr(t, "reconnected goal", got.Goal, &goal)
	requireStringPtr(t, "reconnected notes", got.Notes, &notes)
	if !slices.Equal([]uuid.UUID{got.Storylines[0].ID, got.Storylines[1].ID}, []uuid.UUID{f.storylines[1], f.storylines[0]}) || !slices.Equal(got.Materials, p.Materials) || !slices.Equal(got.Foreshadowings, p.Foreshadowings) {
		t.Fatalf("reconnected associations=%+v", got)
	}
}

// --- SaveMock revision and snapshot tests ---

func TestSaveMockCreatesRevisionAndSetsCurrentRevisionID(t *testing.T) {
	db, ctx := openIntegrationDB(t)
	f := newFixture(t, ctx, db)
	r := mustNewRepo(t, db)

	p := plan(f, 1)
	goal, notes := "mock goal", "mock notes"
	p.Goal, p.Notes = &goal, &notes
	p.Storylines = []StorylineRef{{ID: f.storylines[0], Relation: "primary"}, {ID: f.storylines[1], Relation: "secondary"}}
	p.Materials = []uuid.UUID{f.materials[0], f.materials[1]}
	p.Foreshadowings = []uuid.UUID{f.foreshadowings[0]}

	run := Run{ID: uuid.New(), ProjectID: f.project}
	if err := r.SaveMock(ctx, run, []Plan{p}); err != nil {
		t.Fatalf("SaveMock: %v", err)
	}

	// Verify plan has current_revision_id set.
	got, err := r.GetByID(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.CurrentRevisionID == nil {
		t.Fatal("current_revision_id is nil")
	}

	// Verify revision exists in database.
	var revID uuid.UUID
	var revPlanID, revProjectID uuid.UUID
	var revNo int
	var revChangeType string
	var revCreatedBy string
	var revSnapshot []byte
	if err := db.QueryRow(ctx, `
		SELECT id, chapter_plan_id, project_id, revision_no, change_type, created_by, snapshot
		FROM chapter_plan_revisions WHERE chapter_plan_id = $1
	`, p.ID).Scan(&revID, &revPlanID, &revProjectID, &revNo, &revChangeType, &revCreatedBy, &revSnapshot); err != nil {
		t.Fatalf("query revision: %v", err)
	}

	if revID != *got.CurrentRevisionID {
		t.Fatalf("current_revision_id=%s, revision id=%s", got.CurrentRevisionID, revID)
	}
	if revPlanID != p.ID {
		t.Fatalf("revision chapter_plan_id=%s, want=%s", revPlanID, p.ID)
	}
	if revProjectID != f.project {
		t.Fatalf("revision project_id=%s, want=%s", revProjectID, f.project)
	}
	if revNo != 1 {
		t.Fatalf("revision_no=%d, want=1", revNo)
	}
	if revChangeType != "manual_create" {
		t.Fatalf("change_type=%s, want=manual_create", revChangeType)
	}
	if revCreatedBy != p.CreatedBy {
		t.Fatalf("created_by=%s, want=%s", revCreatedBy, p.CreatedBy)
	}
}

func TestSaveMockRevisionSnapshotCanonicalFormat(t *testing.T) {
	db, ctx := openIntegrationDB(t)
	f := newFixture(t, ctx, db)
	r := mustNewRepo(t, db)

	p := plan(f, 1)
	p.Title = "Mock Title"
	p.Summary = "Mock Summary"
	p.Storylines = []StorylineRef{{ID: f.storylines[2], Relation: "secondary"}, {ID: f.storylines[0], Relation: "primary"}}
	p.Materials = []uuid.UUID{f.materials[2], f.materials[0]}
	p.Foreshadowings = []uuid.UUID{f.foreshadowings[1]}

	if err := r.SaveMock(ctx, Run{ID: uuid.New(), ProjectID: f.project}, []Plan{p}); err != nil {
		t.Fatalf("SaveMock: %v", err)
	}

	var rawSnap []byte
	if err := db.QueryRow(ctx, "SELECT snapshot FROM chapter_plan_revisions WHERE chapter_plan_id = $1", p.ID).Scan(&rawSnap); err != nil {
		t.Fatal(err)
	}

	var snap candidateSnapshotStruct
	if err := json.Unmarshal(rawSnap, &snap); err != nil {
		t.Fatalf("unmarshal snapshot: %v", err)
	}

	if snap.ChapterNo != 1 {
		t.Fatalf("chapterNo=%d, want=1", snap.ChapterNo)
	}
	if snap.Title != "Mock Title" {
		t.Fatalf("title=%s", snap.Title)
	}
	if snap.Summary != "Mock Summary" {
		t.Fatalf("summary=%s", snap.Summary)
	}
	if snap.ChapterPurpose != "other" {
		t.Fatalf("chapterPurpose=%s", snap.ChapterPurpose)
	}

	// Storyline order and relation.
	if len(snap.StorylineRefs) != 2 {
		t.Fatalf("storylineRefs len=%d, want=2", len(snap.StorylineRefs))
	}
	if snap.StorylineRefs[0].ID != f.storylines[2] || snap.StorylineRefs[0].Relation != "secondary" || snap.StorylineRefs[0].Position != 0 {
		t.Fatalf("storylineRefs[0]=%+v", snap.StorylineRefs[0])
	}
	if snap.StorylineRefs[1].ID != f.storylines[0] || snap.StorylineRefs[1].Relation != "primary" || snap.StorylineRefs[1].Position != 1 {
		t.Fatalf("storylineRefs[1]=%+v", snap.StorylineRefs[1])
	}

	// Material order.
	if len(snap.MaterialRefs) != 2 {
		t.Fatalf("materialRefs len=%d, want=2", len(snap.MaterialRefs))
	}
	if snap.MaterialRefs[0].ID != f.materials[2] || snap.MaterialRefs[0].Position != 0 {
		t.Fatalf("materialRefs[0]=%+v", snap.MaterialRefs[0])
	}
	if snap.MaterialRefs[1].ID != f.materials[0] || snap.MaterialRefs[1].Position != 1 {
		t.Fatalf("materialRefs[1]=%+v", snap.MaterialRefs[1])
	}

	// Foreshadowing order.
	if len(snap.ForeshadowingRefs) != 1 {
		t.Fatalf("foreshadowingRefs len=%d, want=1", len(snap.ForeshadowingRefs))
	}
	if snap.ForeshadowingRefs[0].ID != f.foreshadowings[1] || snap.ForeshadowingRefs[0].Position != 0 {
		t.Fatalf("foreshadowingRefs[0]=%+v", snap.ForeshadowingRefs[0])
	}
}

func TestSaveMockMultiplePlansEachHasOwnRevision(t *testing.T) {
	db, ctx := openIntegrationDB(t)
	f := newFixture(t, ctx, db)
	r := mustNewRepo(t, db)

	p1 := plan(f, 1)
	p1.Storylines = []StorylineRef{{ID: f.storylines[0], Relation: "primary"}}
	p2 := plan(f, 2)
	p2.Storylines = []StorylineRef{{ID: f.storylines[1], Relation: "primary"}}
	p3 := plan(f, 3)
	p3.Storylines = []StorylineRef{{ID: f.storylines[2], Relation: "primary"}}

	run := Run{ID: uuid.New(), ProjectID: f.project}
	if err := r.SaveMock(ctx, run, []Plan{p1, p2, p3}); err != nil {
		t.Fatalf("SaveMock: %v", err)
	}

	for i, p := range []Plan{p1, p2, p3} {
		got, err := r.GetByID(ctx, p.ID)
		if err != nil {
			t.Fatalf("plan %d GetByID: %v", i, err)
		}
		if got.CurrentRevisionID == nil {
			t.Fatalf("plan %d current_revision_id is nil", i)
		}

		var count int
		if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM chapter_plan_revisions WHERE chapter_plan_id = $1", p.ID).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("plan %d has %d revisions, want 1", i, count)
		}

		var revID uuid.UUID
		if err := db.QueryRow(ctx, "SELECT id FROM chapter_plan_revisions WHERE chapter_plan_id = $1", p.ID).Scan(&revID); err != nil {
			t.Fatal(err)
		}
		if revID != *got.CurrentRevisionID {
			t.Fatalf("plan %d current_revision_id=%s, revision.id=%s", i, got.CurrentRevisionID, revID)
		}
	}

	// No cross-plan references.
	for i, p := range []Plan{p1, p2, p3} {
		var revChapterPlanID uuid.UUID
		if err := db.QueryRow(ctx, "SELECT chapter_plan_id FROM chapter_plan_revisions WHERE chapter_plan_id = $1", p.ID).Scan(&revChapterPlanID); err != nil {
			t.Fatal(err)
		}
		if revChapterPlanID != p.ID {
			t.Fatalf("plan %d revision chapter_plan_id=%s, want=%s", i, revChapterPlanID, p.ID)
		}
	}
}

func TestSaveMockAtomicRollbackOnInvalidAssociation(t *testing.T) {
	db, ctx := openIntegrationDB(t)
	f := newFixture(t, ctx, db)
	r := mustNewRepo(t, db)

	// Create a storyline in otherProject (cross-project reference will fail).
	otherStoryline := uuid.New()
	if _, err := db.Exec(ctx, "INSERT INTO storylines(id,project_id,type,relation,name,status,sort_order,created_by) VALUES($1,$2,'main','root','foreign','active',0,'i05')", otherStoryline, f.otherProject); err != nil {
		t.Fatal(err)
	}

	p := plan(f, 1)
	p.Storylines = []StorylineRef{{ID: otherStoryline, Relation: "primary"}} // cross-project

	run := Run{ID: uuid.New(), ProjectID: f.project}
	err := r.SaveMock(ctx, run, []Plan{p})
	if err == nil {
		t.Fatal("expected error for cross-project association")
	}

	// Verify no residual records.
	var runCount int
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM mock_generation_runs WHERE id = $1", run.ID).Scan(&runCount); err != nil {
		t.Fatal(err)
	}
	if runCount != 0 {
		t.Fatalf("mock_generation_runs residual count=%d, want=0", runCount)
	}

	var planCount int
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM chapter_plans WHERE id = $1", p.ID).Scan(&planCount); err != nil {
		t.Fatal(err)
	}
	if planCount != 0 {
		t.Fatalf("chapter_plans residual count=%d, want=0", planCount)
	}

	var revCount int
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM chapter_plan_revisions WHERE chapter_plan_id = $1", p.ID).Scan(&revCount); err != nil {
		t.Fatal(err)
	}
	if revCount != 0 {
		t.Fatalf("chapter_plan_revisions residual count=%d, want=0", revCount)
	}

	var storyCount int
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM chapter_plan_storylines WHERE chapter_plan_id = $1", p.ID).Scan(&storyCount); err != nil {
		t.Fatal(err)
	}
	if storyCount != 0 {
		t.Fatalf("chapter_plan_storylines residual count=%d, want=0", storyCount)
	}
}
