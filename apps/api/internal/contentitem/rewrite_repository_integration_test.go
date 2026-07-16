package contentitem

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func openRewriteRepositoryDB(t *testing.T) (*pgxpool.Pool, context.Context) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set; rewrite repository integration test skipped")
	}
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	expected := os.Getenv("ITERATION07_REWRITE_TEST_DATABASE")
	if expected == "" {
		t.Skip("ITERATION07_REWRITE_TEST_DATABASE is not set; fresh rewrite repository integration test skipped")
	}
	if config.ConnConfig.Database != expected || expected == "ai_content_factory_i07_migration_test" {
		t.Fatalf("TEST_DATABASE_URL database=%q does not name the required fresh Iteration 07 rewrite database", config.ConnConfig.Database)
	}
	prepareFreshRewriteRepositoryDB(t, expected, url)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	db, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	var version int
	if err = db.QueryRow(ctx, "SELECT COALESCE(MAX(version),0) FROM schema_migrations").Scan(&version); err != nil || version != 7 {
		t.Fatalf("migration version=%d err=%v, want 7", version, err)
	}
	return db, ctx
}

var rewriteTestDatabaseName = regexp.MustCompile(`^ai_content_factory_i07_rewrite_[a-z0-9_]+$`)

func prepareFreshRewriteRepositoryDB(t *testing.T, database, targetURL string) {
	t.Helper()
	if !rewriteTestDatabaseName.MatchString(database) {
		t.Fatalf("unsafe fresh test database name %q", database)
	}
	adminURL := os.Getenv("ITERATION07_REWRITE_TEST_ADMIN_URL")
	if adminURL == "" {
		t.Fatal("ITERATION07_REWRITE_TEST_ADMIN_URL is required for a fresh isolated database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(ctx)
	var exists bool
	if err = admin.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname=$1)", database).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatalf("fresh test database %q already exists", database)
	}
	if _, err = admin.Exec(ctx, "CREATE DATABASE "+database); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, e := pgx.Connect(context.Background(), adminURL)
		if e == nil {
			_, _ = cleanup.Exec(context.Background(), "DROP DATABASE IF EXISTS "+database+" WITH (FORCE)")
			cleanup.Close(context.Background())
		}
	})
	command := exec.Command("go", "run", "./cmd/migrate", "up")
	command.Dir = "../.."
	command.Env = append(os.Environ(), "DATABASE_URL="+targetURL)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("migrate fresh test database: %v: %s", err, fmt.Sprint(string(output)))
	}
}

type rewriteRepositoryFixture struct {
	projectID, otherProjectID                uuid.UUID
	itemID, otherItemID                      uuid.UUID
	v1ID, otherV1ID, reviewID, otherReviewID uuid.UUID
}

func insertRewriteRepositoryFixture(t *testing.T, ctx context.Context, db *pgxpool.Pool) rewriteRepositoryFixture {
	t.Helper()
	f := rewriteRepositoryFixture{uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()}
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	for _, projectID := range []uuid.UUID{f.projectID, f.otherProjectID} {
		if _, err = tx.Exec(ctx, "INSERT INTO projects(id,name,type,created_by) VALUES($1,$2,'novel','i07-repository')", projectID, "i07-repository-"+projectID.String()); err != nil {
			t.Fatal(err)
		}
	}
	for _, row := range []struct {
		projectID, itemID, versionID uuid.UUID
		chapterNo                    int
	}{{f.projectID, f.itemID, f.v1ID, 1}, {f.otherProjectID, f.otherItemID, f.otherV1ID, 2}} {
		chapterID := uuid.New()
		if _, err = tx.Exec(ctx, "INSERT INTO chapter_plans(id,project_id,chapter_no,title,status,source,confirmed_at,created_by) VALUES($1,$2,$3,'chapter','confirmed','mock_generated',NOW(),'i07-repository')", chapterID, row.projectID, row.chapterNo); err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(ctx, "INSERT INTO content_items(id,project_id,chapter_plan_id,title,current_version_id) VALUES($1,$2,$3,'item',$4)", row.itemID, row.projectID, chapterID, row.versionID); err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(ctx, "INSERT INTO content_versions(id,content_item_id,version_no,title,content,source,status,frozen_at) VALUES($1,$2,1,'v1','frozen','mock_generated','frozen',NOW())", row.versionID, row.itemID); err != nil {
			t.Fatal(err)
		}
	}
	for _, row := range []struct{ projectID, itemID, versionID, reviewID uuid.UUID }{{f.projectID, f.itemID, f.v1ID, f.reviewID}, {f.otherProjectID, f.otherItemID, f.otherV1ID, f.otherReviewID}} {
		runID := uuid.New()
		if _, err = tx.Exec(ctx, "INSERT INTO workflow_runs(id,project_id,content_item_id,content_version_id,provider_key,workflow_key,subject_type,subject_id,status,idempotency_key,request_fingerprint,input_json,output_json,started_at,finished_at) VALUES($1,$2,$3,$4,'mock','content_mock_review','content_item',$3,'succeeded',$5,'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','{}','{}',NOW(),NOW())", runID, row.projectID, row.itemID, row.versionID, "review-"+runID.String()); err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(ctx, "INSERT INTO review_reports(id,project_id,content_item_id,content_version_id,workflow_run_id,provider_key,status,conclusion,score,summary) VALUES($1,$2,$3,$4,$5,'mock','completed','pass',100,'review')", row.reviewID, row.projectID, row.itemID, row.versionID, runID); err != nil {
			t.Fatal(err)
		}
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec(context.Background(), "DELETE FROM projects WHERE id=$1 OR id=$2", f.projectID, f.otherProjectID)
	})
	return f
}

func TestRewriteRepositoriesPostgres(t *testing.T) {
	db, ctx := openRewriteRepositoryDB(t)
	f := insertRewriteRepositoryFixture(t, ctx, db)
	r := NewPostgresRepository(db)
	if got, err := r.GetContentVersion(ctx, f.v1ID); err != nil || got.ID != f.v1ID || got.Source != ContentVersionSourceMockGenerated {
		t.Fatalf("v1=%+v err=%v", got, err)
	}
	if _, err := r.GetContentVersion(ctx, uuid.New()); !errors.Is(err, ErrContentVersionNotFound) {
		t.Fatalf("missing version=%v", err)
	}

	v2ID := uuid.New()
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err = r.CreateContentVersion(ctx, tx, ContentVersion{ID: v2ID, ContentItemID: f.itemID, VersionNo: 2, Version: 1, Title: "v2", Content: "rewrite", WordCount: 1, Source: ContentVersionSourceMockRewrite, Status: ContentVersionStatusEditableDraft, GenerationParameters: []byte(`{"rewrite_focus":["pacing"]}`)})
	if err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if got, err := r.GetContentVersion(ctx, v2ID); err != nil || got.ID != v2ID || got.Source != ContentVersionSourceMockRewrite || got.FrozenAt != nil {
		t.Fatalf("v2=%+v err=%v", got, err)
	}
	var current uuid.UUID
	if err = db.QueryRow(ctx, "SELECT current_version_id FROM content_items WHERE id=$1", f.itemID).Scan(&current); err != nil || current != f.v1ID {
		t.Fatalf("current=%s err=%v", current, err)
	}
	if page, err := r.ListContentVersions(ctx, f.itemID, ContentVersionPageOptions{Limit: 1, Offset: 0}); err != nil || page.Total != 2 || len(page.Items) != 1 || page.Items[0].ID != v2ID || page.Limit != 1 {
		t.Fatalf("page=%+v err=%v", page, err)
	}
	if total, err := r.CountContentVersions(ctx, f.itemID); err != nil || total != 2 {
		t.Fatalf("total=%d err=%v", total, err)
	}
	if got, err := r.GetContentVersionByNumber(ctx, f.itemID, 2); err != nil || got.ID != v2ID {
		t.Fatalf("by number=%+v err=%v", got, err)
	}
	if page, err := r.ListContentVersions(ctx, f.otherItemID, ContentVersionPageOptions{Limit: 20}); err != nil || page.Total != 1 || page.Items[0].ID != f.otherV1ID {
		t.Fatalf("isolation=%+v err=%v", page, err)
	}
	tx, err = db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err = r.CreateContentVersion(ctx, tx, ContentVersion{ID: uuid.New(), ContentItemID: f.itemID, VersionNo: 2, Version: 1, Title: "duplicate", Source: ContentVersionSourceMockRewrite, Status: ContentVersionStatusEditableDraft})
	_ = tx.Rollback(ctx)
	if !errors.Is(err, ErrRewriteAlreadyExists) {
		t.Fatalf("duplicate=%v", err)
	}

	reviewID := f.reviewID
	firstRunID := uuid.New()
	tx, err = db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	running, err := r.CreateMockRewriteRunning(ctx, tx, WorkflowRun{ID: firstRunID, ProjectID: f.projectID, ContentItemID: f.itemID, ContentVersionID: f.v1ID, SourceReviewReportID: &reviewID, ProviderKey: WorkflowProviderMock, WorkflowKey: WorkflowKeyMockRewrite, SubjectType: "content_item", SubjectID: f.itemID, Status: WorkflowRunStatusRunning, IdempotencyKey: "rewrite-1", RequestFingerprint: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", InputJSON: []byte(`{"source_version_no":1}`)})
	if err != nil {
		t.Fatal(err)
	}
	if running.TargetContentVersionID != nil || running.SourceReviewReportID == nil {
		t.Fatalf("running=%+v", running)
	}
	if _, err = r.MarkMockRewriteSucceeded(ctx, tx, firstRunID, v2ID, []byte(`{"target_version_no":2}`), time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if got, err := r.GetWorkflowRun(ctx, firstRunID); err != nil || got.TargetContentVersionID == nil || *got.TargetContentVersionID != v2ID || got.FinishedAt == nil {
		t.Fatalf("succeeded=%+v err=%v", got, err)
	}
	if got, err := r.FindMockRewriteByIdempotencyKey(ctx, f.itemID, "rewrite-1"); err != nil || got.ID != firstRunID {
		t.Fatalf("key=%+v err=%v", got, err)
	}
	if _, err := db.Exec(ctx, "INSERT INTO workflow_runs(id,project_id,content_item_id,content_version_id,provider_key,workflow_key,subject_type,subject_id,status,idempotency_key,request_fingerprint,input_json,output_json,started_at,finished_at) VALUES($1,$2,$3,$4,'mock','content_mock_review','content_item',$3,'succeeded','rewrite-1','dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd','{}','{}',NOW(),NOW())", uuid.New(), f.projectID, f.itemID, f.v1ID); err != nil {
		t.Fatal(err)
	}
	if got, err := r.FindMockRewriteByIdempotencyKey(ctx, f.itemID, "rewrite-1"); err != nil || got.ID != firstRunID {
		t.Fatalf("workflow scope=%+v err=%v", got, err)
	}
	if got, err := r.FindMockRewriteByFingerprint(ctx, f.itemID, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"); err != nil || got.ID != firstRunID {
		t.Fatalf("fingerprint=%+v err=%v", got, err)
	}
	if _, err := r.FindMockRewriteByIdempotencyKey(ctx, f.otherItemID, "rewrite-1"); !errors.Is(err, ErrWorkflowRunNotFound) {
		t.Fatalf("cross item=%v", err)
	}

	secondRunID := uuid.New()
	tx, err = db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err = r.CreateMockRewriteRunning(ctx, tx, WorkflowRun{ID: secondRunID, ProjectID: f.projectID, ContentItemID: f.itemID, ContentVersionID: f.v1ID, SourceReviewReportID: &reviewID, ProviderKey: WorkflowProviderMock, WorkflowKey: WorkflowKeyMockRewrite, SubjectType: "content_item", SubjectID: f.itemID, Status: WorkflowRunStatusRunning, IdempotencyKey: "rewrite-2", RequestFingerprint: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc", StartedAt: time.Now().UTC().Add(time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	failed, err := r.MarkMockRewriteFailed(ctx, tx, secondRunID, "mock_rewrite_failed", "mock rewrite failed", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if failed.TargetContentVersionID != nil || string(failed.OutputJSON) != "{}" || failed.ErrorCode == nil {
		t.Fatalf("failed=%+v", failed)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if latest, err := r.LatestMockRewrite(ctx, f.itemID); err != nil || latest.ID != secondRunID {
		t.Fatalf("latest=%+v err=%v", latest, err)
	}
	if _, err := r.GetWorkflowRun(ctx, uuid.New()); !errors.Is(err, ErrWorkflowRunNotFound) {
		t.Fatalf("missing run=%v", err)
	}

	db.Close()
	if _, err := r.GetWorkflowRun(context.Background(), firstRunID); !errors.Is(err, ErrInternal) {
		t.Fatalf("database error leaked: %v", err)
	}
}
