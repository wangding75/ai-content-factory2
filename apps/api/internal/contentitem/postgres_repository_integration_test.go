package contentitem

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const integrationDatabase = "ai_content_factory_i06_test"

func openDB(t *testing.T) (*pgxpool.Pool, context.Context) {
	t.Helper()
	u := os.Getenv("TEST_DATABASE_URL")
	if u == "" {
		t.Skip("TEST_DATABASE_URL is not set; PostgreSQL integration test skipped")
	}
	cfg, e := pgxpool.ParseConfig(u)
	if e != nil {
		t.Fatal(e)
	}
	if cfg.ConnConfig.Database != integrationDatabase {
		t.Skipf("TEST_DATABASE_URL targets %q, not %q", cfg.ConnConfig.Database, integrationDatabase)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	db, e := pgxpool.NewWithConfig(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(db.Close)
	if e = db.Ping(ctx); e != nil {
		t.Fatal(e)
	}
	return db, ctx
}

type fx struct{ project, other, confirmed, pending uuid.UUID }

func fixture(t *testing.T, ctx context.Context, db *pgxpool.Pool) fx {
	t.Helper()
	f := fx{uuid.New(), uuid.New(), uuid.New(), uuid.New()}
	tx, e := db.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(ctx)
	for _, id := range []uuid.UUID{f.project, f.other} {
		if _, e = tx.Exec(ctx, "INSERT INTO projects(id,name,type,created_by) VALUES($1,$2,'novel','i06')", id, "i06-"+id.String()); e != nil {
			t.Fatal(e)
		}
	}
	for _, p := range []struct {
		id     uuid.UUID
		status string
		n      int
	}{{f.confirmed, "confirmed", 1}, {f.pending, "pending_confirmation", 2}} {
		if _, e = tx.Exec(ctx, "INSERT INTO chapter_plans(id,project_id,chapter_no,title,summary,status,source,created_by,confirmed_at) VALUES($1,$2,$3,$4,'summary',$5,'mock_generated','i06',CASE WHEN $5='confirmed' THEN NOW() ELSE NULL END)", p.id, f.project, p.n, "chapter "+p.status, p.status); e != nil {
			t.Fatal(e)
		}
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		_, _ = db.Exec(context.Background(), "DELETE FROM projects WHERE id=$1 OR id=$2", f.project, f.other)
	})
	return f
}
func create(t *testing.T, ctx context.Context, r *PostgresRepository, f fx) CreateResult {
	t.Helper()
	x, e := r.CreateOrGet(ctx, f.confirmed)
	if e != nil {
		t.Fatal(e)
	}
	return x
}
func count(t *testing.T, ctx context.Context, db *pgxpool.Pool, q string, args ...any) int {
	t.Helper()
	var n int
	if e := db.QueryRow(ctx, q, args...).Scan(&n); e != nil {
		t.Fatal(e)
	}
	return n
}

func TestPostgresCreateGetAndIsolation(t *testing.T) {
	db, ctx := openDB(t)
	f := fixture(t, ctx, db)
	r := NewPostgresRepository(db)
	x := create(t, ctx, r, f)
	if !x.Created || x.Detail.Item.ProjectID != f.project || x.Detail.Item.Status != "draft" || x.Detail.CurrentVersion.VersionNo != 1 || x.Detail.CurrentVersion.Version != 1 || x.Detail.CurrentVersion.Source != "manual_created" || x.Detail.CurrentVersion.Status != "editable_draft" || x.Detail.Item.CurrentVersionID != x.Detail.CurrentVersion.ID {
		t.Fatalf("create=%+v", x)
	}
	if x.Detail.CurrentVersion.Content != "" || x.Detail.CurrentVersion.Summary != nil || x.Detail.CurrentVersion.WordCount != 0 {
		t.Fatal("v1 was not blank")
	}
	if count(t, ctx, db, "SELECT count(*) FROM content_items WHERE chapter_plan_id=$1", f.confirmed) != 1 || count(t, ctx, db, "SELECT count(*) FROM content_versions WHERE content_item_id=$1", x.Detail.Item.ID) != 1 {
		t.Fatal("unexpected creation rows")
	}
	again, e := r.CreateOrGet(ctx, f.confirmed)
	if e != nil || again.Created || again.Detail.Item.ID != x.Detail.Item.ID || again.Detail.CurrentVersion.ID != x.Detail.CurrentVersion.ID {
		t.Fatalf("retry=%+v err=%v", again, e)
	}
	got, e := r.GetByChapterPlanID(ctx, f.confirmed)
	if e != nil || got.Item.ID != x.Detail.Item.ID {
		t.Fatalf("get chapter=%+v %v", got, e)
	}
	if _, e = r.CreateOrGet(ctx, f.pending); !errors.Is(e, ErrChapterPlanNotConfirmed) {
		t.Fatalf("pending=%v", e)
	}
}

func TestPostgresSaveDraftTriStateAndLocks(t *testing.T) {
	db, ctx := openDB(t)
	f := fixture(t, ctx, db)
	r := NewPostgresRepository(db)
	x := create(t, ctx, r, f)
	empty := ""
	text := "text"
	words := 3
	d, e := r.SaveDraft(ctx, x.Detail.Item.ID, 1, DraftPatch{Summary: OptionalString{Set: true, Value: &text}, Content: OptionalString{Set: true, Value: &text}, WordCount: OptionalInt{Set: true, Value: &words}})
	if e != nil {
		t.Fatal(e)
	}
	if d.CurrentVersion.Version != 2 || *d.CurrentVersion.Summary != "text" {
		t.Fatalf("save=%+v", d)
	}
	d, e = r.SaveDraft(ctx, x.Detail.Item.ID, 2, DraftPatch{Summary: OptionalString{Set: true, Value: nil}})
	if e != nil || d.CurrentVersion.Summary != nil || d.CurrentVersion.Version != 3 {
		t.Fatalf("null=%+v %v", d, e)
	}
	d, e = r.SaveDraft(ctx, x.Detail.Item.ID, 3, DraftPatch{Summary: OptionalString{Set: true, Value: &empty}})
	if e != nil || d.CurrentVersion.Summary == nil || *d.CurrentVersion.Summary != "" || d.CurrentVersion.Version != 4 {
		t.Fatalf("empty=%+v %v", d, e)
	}
	d, e = r.SaveDraft(ctx, x.Detail.Item.ID, 4, DraftPatch{})
	if e != nil || d.CurrentVersion.Content != "text" || d.CurrentVersion.WordCount != 3 {
		t.Fatalf("omitted=%+v %v", d, e)
	}
	if _, e = r.SaveDraft(ctx, x.Detail.Item.ID, 4, DraftPatch{}); !errors.Is(e, ErrVersionConflict) {
		t.Fatalf("conflict=%v", e)
	}
	if _, e = db.Exec(ctx, "UPDATE content_versions SET status='frozen',frozen_at=NOW() WHERE id=$1", d.CurrentVersion.ID); e != nil {
		t.Fatal(e)
	}
	if _, e = r.SaveDraft(ctx, x.Detail.Item.ID, 5, DraftPatch{}); !errors.Is(e, ErrContentVersionLocked) {
		t.Fatalf("locked=%v", e)
	}
}

func generation(f fx, item uuid.UUID, version int, key, fingerprint string) GenerationRequest {
	return GenerationRequest{ContentItemID: item, ExpectedVersion: version, IdempotencyKey: key, Fingerprint: fingerprint, Result: GenerationResult{Content: "generated", WordCount: 1, Parameters: []byte(`{"chapter_goal":"goal"}`), InputJSON: []byte(`{"parameters":{}}`), OutputJSON: []byte(`{"ok":true}`)}}
}
func TestPostgresMockGenerationIdempotencyAndRollback(t *testing.T) {
	db, ctx := openDB(t)
	f := fixture(t, ctx, db)
	r := NewPostgresRepository(db)
	x := create(t, ctx, r, f)
	fp := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	req := generation(f, x.Detail.Item.ID, 1, "key-1", fp)
	o, e := r.PersistMockGeneration(ctx, req)
	if e != nil {
		t.Fatal(e)
	}
	if o.Detail.Item.ProjectID != f.project || o.WorkflowRun.ProjectID != f.project || o.Detail.CurrentVersion.Source != "mock_generated" || o.Detail.CurrentVersion.Version != 2 || o.Detail.CurrentVersion.VersionNo != 1 || o.WorkflowRun.Status != "succeeded" || o.WorkflowRun.RequestFingerprint != fp {
		t.Fatalf("out=%+v", o)
	}
	if count(t, ctx, db, "SELECT count(*) FROM workflow_runs WHERE content_item_id=$1", x.Detail.Item.ID) != 1 || count(t, ctx, db, "SELECT count(*) FROM content_versions WHERE content_item_id=$1", x.Detail.Item.ID) != 1 {
		t.Fatal("unexpected workflow/version rows")
	}
	again, e := r.PersistMockGeneration(ctx, req)
	if e != nil || again.Detail.CurrentVersion.Version != 2 || again.WorkflowRun.ID != o.WorkflowRun.ID {
		t.Fatalf("retry=%+v %v", again, e)
	}
	if _, e = r.PersistMockGeneration(ctx, generation(f, x.Detail.Item.ID, 2, "key-1", "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff")); !errors.Is(e, ErrIdempotencyConflict) {
		t.Fatalf("fingerprint=%v", e)
	}
	bad := generation(f, x.Detail.Item.ID, 1, "key-2", fp)
	if _, e = r.PersistMockGeneration(ctx, bad); !errors.Is(e, ErrVersionConflict) {
		t.Fatalf("rollback err=%v", e)
	}
	if count(t, ctx, db, "SELECT count(*) FROM workflow_runs WHERE content_item_id=$1", x.Detail.Item.ID) != 1 {
		t.Fatal("failed generation left a run")
	}
}

func TestPostgresFailureRunAndReconnect(t *testing.T) {
	db, ctx := openDB(t)
	f := fixture(t, ctx, db)
	r := NewPostgresRepository(db)
	x := create(t, ctx, r, f)
	fp := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	run, e := r.RecordMockGenerationFailure(ctx, FailureRequest{ContentItemID: x.Detail.Item.ID, IdempotencyKey: "failure-key", Fingerprint: fp, InputJSON: []byte(`{}`), ErrorCode: "MOCK_GENERATION_FAILED", ErrorSummary: "safe failure"})
	if e != nil {
		t.Fatal(e)
	}
	if run.Status != "failed" || run.FinishedAt == nil || run.ErrorCode == nil || *run.ErrorCode != "MOCK_GENERATION_FAILED" {
		t.Fatalf("failure=%+v", run)
	}
	if count(t, ctx, db, "SELECT count(*) FROM workflow_runs WHERE content_item_id=$1 AND status='running'", x.Detail.Item.ID) != 0 {
		t.Fatal("orphan running workflow")
	}
	if _, e = r.RecordMockGenerationFailure(ctx, FailureRequest{ContentItemID: x.Detail.Item.ID, IdempotencyKey: "failure-key", Fingerprint: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", ErrorCode: "X", ErrorSummary: "safe"}); !errors.Is(e, ErrIdempotencyConflict) {
		t.Fatalf("failure fingerprint=%v", e)
	}
	cfg := db.Config().Copy()
	db.Close()
	re, e := pgxpool.NewWithConfig(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(re.Close)
	// The fixture cleanup uses the original pool, which this test deliberately
	// closes to exercise reconnection. Clean through the replacement pool first.
	t.Cleanup(func() {
		_, _ = re.Exec(context.Background(), "DELETE FROM projects WHERE id=$1 OR id=$2", f.project, f.other)
	})
	got, e := NewPostgresRepository(re).GetByID(ctx, x.Detail.Item.ID)
	if e != nil || got.CurrentVersion.Version != 1 {
		t.Fatalf("reconnect=%+v %v", got, e)
	}
}

var _ = pgx.ErrNoRows
