package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const iteration07MigrationTestDatabase = "ai_content_factory_i07_migration_test"

func openIteration07MigrationDB(t *testing.T) (*pgxpool.Pool, context.Context) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set; Iteration 07 migration PostgreSQL integration test skipped")
	}
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	if config.ConnConfig.Database != iteration07MigrationTestDatabase {
		t.Skipf("TEST_DATABASE_URL targets %q, not %q", config.ConnConfig.Database, iteration07MigrationTestDatabase)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	db, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	var version int
	if err = db.QueryRow(ctx, "SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&version); err != nil || version != 6 {
		t.Fatalf("migration version=%d err=%v, want 6 before Migration 000007", version, err)
	}
	return db, ctx
}

func iteration07Migrations(t *testing.T) []migration {
	t.Helper()
	migrations, err := loadMigrations(filepath.Join("..", "..", "migrations"))
	if err != nil {
		t.Fatal(err)
	}
	return migrations
}

type iteration07Fixture struct {
	projectID, otherProjectID                uuid.UUID
	itemID, otherItemID                      uuid.UUID
	v1ID, otherV1ID, reviewID, otherReviewID uuid.UUID
}

func insertIteration07Fixture(t *testing.T, ctx context.Context, db *pgxpool.Pool) iteration07Fixture {
	t.Helper()
	f := iteration07Fixture{uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()}
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	for _, projectID := range []uuid.UUID{f.projectID, f.otherProjectID} {
		if _, err = tx.Exec(ctx, "INSERT INTO projects(id,name,type,created_by) VALUES($1,$2,'novel','i07-migration')", projectID, "i07-"+projectID.String()); err != nil {
			t.Fatal(err)
		}
	}
	for _, row := range []struct {
		projectID, itemID, versionID uuid.UUID
		chapterNo                    int
	}{{f.projectID, f.itemID, f.v1ID, 1}, {f.otherProjectID, f.otherItemID, f.otherV1ID, 2}} {
		chapterID := uuid.New()
		if _, err = tx.Exec(ctx, "INSERT INTO chapter_plans(id,project_id,chapter_no,title,status,source,confirmed_at,created_by) VALUES($1,$2,$3,'chapter','confirmed','mock_generated',NOW(),'i07-migration')", chapterID, row.projectID, row.chapterNo); err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(ctx, "INSERT INTO content_items(id,project_id,chapter_plan_id,title,current_version_id) VALUES($1,$2,$3,'item',$4)", row.itemID, row.projectID, chapterID, row.versionID); err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(ctx, "INSERT INTO content_versions(id,content_item_id,version_no,title,content,source,status,frozen_at) VALUES($1,$2,1,'v1','frozen v1','mock_generated','frozen',NOW())", row.versionID, row.itemID); err != nil {
			t.Fatal(err)
		}
	}
	for _, row := range []struct {
		projectID, itemID, versionID, reviewID uuid.UUID
	}{{f.projectID, f.itemID, f.v1ID, f.reviewID}, {f.otherProjectID, f.otherItemID, f.otherV1ID, f.otherReviewID}} {
		runID := uuid.New()
		if _, err = tx.Exec(ctx, "INSERT INTO workflow_runs(id,project_id,content_item_id,content_version_id,provider_key,workflow_key,subject_type,subject_id,status,idempotency_key,request_fingerprint,input_json,output_json,started_at,finished_at) VALUES($1,$2,$3,$4,'mock','content_mock_review','content_item',$3,'succeeded',$5,'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','{}','{}',NOW(),NOW())", runID, row.projectID, row.itemID, row.versionID, "review-"+runID.String()); err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(ctx, "INSERT INTO review_reports(id,project_id,content_item_id,content_version_id,workflow_run_id,provider_key,status,conclusion,score,summary) VALUES($1,$2,$3,$4,$5,'mock','completed','pass',100,'completed review')", row.reviewID, row.projectID, row.itemID, row.versionID, runID); err != nil {
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

func mustFail(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected database constraint failure")
	}
}

func TestIteration07Migration000007UpgradeConstraintsAndRollback(t *testing.T) {
	db, ctx := openIteration07MigrationDB(t)
	f := insertIteration07Fixture(t, ctx, db)
	migrations := iteration07Migrations(t)
	conn, err := pgx.Connect(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close(context.Background()) })

	if err = migrateUp(ctx, conn, migrations); err != nil {
		t.Fatalf("upgrade 000006 to 000007: %v", err)
	}
	var version int
	if err = db.QueryRow(ctx, "SELECT COALESCE(MAX(version),0) FROM schema_migrations").Scan(&version); err != nil || version != 7 {
		t.Fatalf("version after upgrade=%d err=%v, want 7", version, err)
	}
	for _, name := range []string{
		"content_versions_mock_rewrite_shape",
		"review_reports_project_item_version_id_id_unique",
		"workflow_runs_target_version_same_item",
		"workflow_runs_source_review_same_scope",
		"workflow_runs_mock_rewrite_shape",
	} {
		var found bool
		if err = db.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_constraint WHERE conname=$1)", name).Scan(&found); err != nil || !found {
			t.Fatalf("constraint %s found=%t err=%v", name, found, err)
		}
	}
	var indexFound bool
	if err = db.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_indexes WHERE schemaname=current_schema() AND indexname='workflow_runs_content_item_workflow_started_at_id_idx')").Scan(&indexFound); err != nil || !indexFound {
		t.Fatalf("rewrite index found=%t err=%v", indexFound, err)
	}
	var projectWorkTables int
	if err = db.QueryRow(ctx, "SELECT count(*) FROM information_schema.tables WHERE table_schema=current_schema() AND table_name ILIKE '%project_work%'").Scan(&projectWorkTables); err != nil || projectWorkTables != 0 {
		t.Fatalf("project work tables=%d err=%v", projectWorkTables, err)
	}

	v2ID, rewriteRunID := uuid.New(), uuid.New()
	if _, err = db.Exec(ctx, "INSERT INTO content_versions(id,content_item_id,version_no,title,content,source,status) VALUES($1,$2,2,'v2','rewrite','mock_rewrite','editable_draft')", v2ID, f.itemID); err != nil {
		t.Fatalf("insert legal mock rewrite v2: %v", err)
	}
	if _, err = db.Exec(ctx, "INSERT INTO workflow_runs(id,project_id,content_item_id,content_version_id,target_content_version_id,source_review_report_id,provider_key,workflow_key,subject_type,subject_id,status,idempotency_key,request_fingerprint,input_json,output_json,started_at,finished_at) VALUES($1,$2,$3,$4,$5,$6,'mock','content_mock_rewrite','content_item',$3,'succeeded','rewrite-success','bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb','{}','{\"target_version_no\":2}',NOW(),NOW())", rewriteRunID, f.projectID, f.itemID, f.v1ID, v2ID, f.reviewID); err != nil {
		t.Fatalf("insert legal rewrite workflow relation: %v", err)
	}

	_, err = db.Exec(ctx, "INSERT INTO workflow_runs(id,project_id,content_item_id,content_version_id,target_content_version_id,source_review_report_id,provider_key,workflow_key,subject_type,subject_id,status,idempotency_key,request_fingerprint,input_json,output_json,started_at,finished_at) VALUES($1,$2,$3,$4,$5,$6,'mock','content_mock_rewrite','content_item',$3,'succeeded','missing-source-version','cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc','{}','{}',NOW(),NOW())", uuid.New(), f.projectID, f.itemID, uuid.New(), v2ID, f.reviewID)
	mustFail(t, err)
	_, err = db.Exec(ctx, "INSERT INTO workflow_runs(id,project_id,content_item_id,content_version_id,target_content_version_id,source_review_report_id,provider_key,workflow_key,subject_type,subject_id,status,idempotency_key,request_fingerprint,input_json,output_json,started_at,finished_at) VALUES($1,$2,$3,$4,$5,$6,'mock','content_mock_rewrite','content_item',$3,'succeeded','missing-source-review','dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd','{}','{}',NOW(),NOW())", uuid.New(), f.projectID, f.itemID, f.v1ID, v2ID, uuid.New())
	mustFail(t, err)
	_, err = db.Exec(ctx, "INSERT INTO content_versions(id,content_item_id,version_no,title,source,status) VALUES($1,$2,3,'bad','invalid_source','editable_draft')", uuid.New(), f.itemID)
	mustFail(t, err)
	_, err = db.Exec(ctx, "INSERT INTO workflow_runs(id,project_id,content_item_id,content_version_id,target_content_version_id,source_review_report_id,provider_key,workflow_key,subject_type,subject_id,status,idempotency_key,request_fingerprint,input_json,output_json,started_at,finished_at) VALUES($1,$2,$3,$4,$5,$6,'mock','content_mock_rewrite','content_item',$3,'succeeded','cross-project','eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee','{}','{}',NOW(),NOW())", uuid.New(), f.projectID, f.itemID, f.v1ID, v2ID, f.otherReviewID)
	mustFail(t, err)
	_, err = db.Exec(ctx, "INSERT INTO content_versions(id,content_item_id,version_no,title,source,status) VALUES($1,$2,2,'duplicate','mock_rewrite','editable_draft')", uuid.New(), f.itemID)
	mustFail(t, err)

	if _, err = db.Exec(ctx, "DELETE FROM workflow_runs WHERE id=$1", rewriteRunID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, "DELETE FROM content_versions WHERE id=$1", v2ID); err != nil {
		t.Fatal(err)
	}
	if err = migrateDownOne(ctx, conn, migrations); err != nil {
		t.Fatalf("downgrade 000007 to 000006: %v", err)
	}
	if err = db.QueryRow(ctx, "SELECT COALESCE(MAX(version),0) FROM schema_migrations").Scan(&version); err != nil || version != 6 {
		t.Fatalf("version after downgrade=%d err=%v, want 6", version, err)
	}
	var columnFound bool
	if err = db.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='workflow_runs' AND column_name='target_content_version_id')").Scan(&columnFound); err != nil || columnFound {
		t.Fatalf("target column after downgrade found=%t err=%v", columnFound, err)
	}
	_, err = db.Exec(ctx, "INSERT INTO content_versions(id,content_item_id,version_no,title,source,status) VALUES($1,$2,2,'v2','mock_rewrite','editable_draft')", uuid.New(), f.itemID)
	mustFail(t, err)

	if err = migrateUp(ctx, conn, migrations); err != nil {
		t.Fatalf("upgrade 000006 to 000007 after down: %v", err)
	}
	if err = db.QueryRow(ctx, "SELECT COALESCE(MAX(version),0) FROM schema_migrations").Scan(&version); err != nil || version != 7 {
		t.Fatalf("version after second upgrade=%d err=%v, want 7", version, err)
	}
}
