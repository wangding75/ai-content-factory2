package ready

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/local/ai-content-factory/apps/api/internal/workflowrun"
)

func TestDiscoverMigrationHeadFromDisk(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "000001_a.up.sql"), []byte("select 1;"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "000001_a.down.sql"), []byte("select 1;"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "000003_c.up.sql"), []byte("select 1;"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "000003_c.down.sql"), []byte("select 1;"), 0o644); err != nil {
		t.Fatal(err)
	}
	head, err := DiscoverMigrationHead(dir)
	if err != nil || head != 3 {
		t.Fatalf("head=%d err=%v", head, err)
	}
}

func TestWorkerHealthHealthyAndFatal(t *testing.T) {
	h := workflowrun.NewWorkerHealth()
	now := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)
	if h.Healthy(now, time.Minute) {
		t.Fatal("not started should be unhealthy")
	}
	h.MarkStarted()
	h.MarkLoopStart(now)
	h.MarkLoopComplete(now.Add(time.Second), nil)
	if !h.Healthy(now.Add(30*time.Second), time.Minute) {
		t.Fatal("fresh loop should be healthy")
	}
	if h.Healthy(now.Add(5*time.Minute), time.Minute) {
		t.Fatal("stale loop should be unhealthy")
	}
	h.MarkFatal("loop panic")
	if h.Healthy(now.Add(time.Second), time.Minute) {
		t.Fatal("fatal should be unhealthy")
	}
}

func TestCheckerReadyAndDatabaseFailure(t *testing.T) {
	raw := os.Getenv("DATABASE_URL")
	if raw == "" {
		t.Fatal("DATABASE_URL is required")
	}
	pool, err := pgxpool.New(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	candidates := []string{
		filepath.Join("..", "..", "migrations"),
		"migrations",
		filepath.Join("apps", "api", "migrations"),
	}
	// Walk up from the package directory to locate the module migrations folder.
	if wd, wdErr := os.Getwd(); wdErr == nil {
		dir := wd
		for i := 0; i < 6; i++ {
			candidates = append(candidates, filepath.Join(dir, "migrations"))
			candidates = append(candidates, filepath.Join(dir, "apps", "api", "migrations"))
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	var head int
	var headErr error = fmt.Errorf("migrations not found")
	for _, candidate := range candidates {
		var discoverErr error
		head, discoverErr = DiscoverMigrationHead(candidate)
		if discoverErr == nil {
			headErr = nil
			break
		}
		headErr = discoverErr
	}
	if headErr != nil {
		t.Fatalf("open migrations: %v", headErr)
	}
	health := workflowrun.NewWorkerHealth()
	health.MarkStarted()
	now := time.Now().UTC()
	health.MarkLoopStart(now)
	health.MarkLoopComplete(now, nil)
	checker := &Checker{Pool: pool, ExpectedMigrationHead: head, Worker: health, Now: func() time.Time { return now }}
	result := checker.Check(context.Background())
	if !result.Ready || result.Checks["database"] != "ok" || result.Checks["migration"] != "ok" || result.Checks["worker"] != "ok" {
		t.Fatalf("result=%+v", result)
	}
	// Behind head
	behind := &Checker{Pool: pool, ExpectedMigrationHead: head + 10, Worker: health, Now: func() time.Time { return now }}
	if behind.Check(context.Background()).Ready {
		t.Fatal("behind head should not be ready")
	}
	// Worker stopped
	health.MarkStopped()
	if checker.Check(context.Background()).Ready {
		t.Fatal("stopped worker should not be ready")
	}
}
