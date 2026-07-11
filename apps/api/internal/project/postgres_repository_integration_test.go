package project

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresRepositoryIntegrationCreateWritesAuditLog(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect PostgreSQL: %v", err)
	}
	defer pool.Close()
	repository := NewPostgresRepository(pool)
	p, err := New("repository integration", TypeNovel, "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM audit_logs WHERE subject_id = $1", p.ID.String())
		_, _ = pool.Exec(context.Background(), "DELETE FROM projects WHERE id = $1", p.ID)
	}()
	created, err := repository.Create(ctx, p, "integration-test")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	var auditCount int
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM audit_logs WHERE subject_id = $1 AND action = $2", created.ID.String(), "project.created").Scan(&auditCount); err != nil {
		t.Fatalf("query audit log: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("audit logs = %d, want 1", auditCount)
	}
}

func TestPostgresRepositoryIntegrationRollsBackWhenAuditWriteFails(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect PostgreSQL: %v", err)
	}
	defer pool.Close()
	_, err = pool.Exec(ctx, `CREATE OR REPLACE FUNCTION test_project_audit_failure() RETURNS trigger AS $$ BEGIN RAISE EXCEPTION 'forced audit failure'; END; $$ LANGUAGE plpgsql; CREATE TRIGGER test_project_audit_failure BEFORE INSERT ON audit_logs FOR EACH ROW EXECUTE FUNCTION test_project_audit_failure();`)
	if err != nil {
		t.Fatalf("install failure trigger: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), "DROP TRIGGER IF EXISTS test_project_audit_failure ON audit_logs; DROP FUNCTION IF EXISTS test_project_audit_failure()")
	}()
	p, err := New("rollback integration", TypeNovel, "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewPostgresRepository(pool).Create(ctx, p, "integration-test")
	if err == nil {
		t.Fatal("expected audit failure")
	}
	var projectCount, auditCount int
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM projects WHERE id = $1", p.ID).Scan(&projectCount); err != nil {
		t.Fatalf("count projects: %v", err)
	}
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM audit_logs WHERE subject_id = $1", p.ID.String()).Scan(&auditCount); err != nil {
		t.Fatalf("count audit logs: %v", err)
	}
	if projectCount != 0 || auditCount != 0 {
		t.Fatalf("rollback left project=%d audit_log=%d", projectCount, auditCount)
	}
}
