package chapterplan

import (
	"context"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const integrationDatabase = "ai_content_factory_http_test"

func openIntegrationDB(t *testing.T) (*pgxpool.Pool, context.Context) {
	t.Helper()
	u := os.Getenv("TEST_DATABASE_URL")
	if u == "" {
		t.Skip("TEST_DATABASE_URL is not set; PostgreSQL integration test skipped")
	}
	cfg, err := pgxpool.ParseConfig(u)
	if err != nil {
		t.Fatalf("parse TEST_DATABASE_URL: %v", err)
	}
	if cfg.ConnConfig.Database != integrationDatabase {
		t.Skipf("TEST_DATABASE_URL targets database %q, not %q; PostgreSQL integration test skipped", cfg.ConnConfig.Database, integrationDatabase)
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

func TestPostgresRepositoryNullableTriState(t *testing.T) {
	db, ctx := openIntegrationDB(t)
	f := newFixture(t, ctx, db)
	r := NewPostgresRepository(db)
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
	r := NewPostgresRepository(db)
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
	r := NewPostgresRepository(db)
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
	r := NewPostgresRepository(db)
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
	r := NewPostgresRepository(db)
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
	got, err := NewPostgresRepository(reconnected).GetByID(ctx, p.ID)
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
