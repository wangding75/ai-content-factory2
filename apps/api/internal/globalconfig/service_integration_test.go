package globalconfig

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCreateProviderConcurrentIdempotencyUsesPostgresLock(t *testing.T) {
	pool, ctx := globalConfigIntegrationDatabase(t)
	service, err := NewService(pool, "iteration-12-concurrency-key")
	if err != nil {
		t.Fatal(err)
	}
	const requests = 8
	ready := make(chan struct{}, requests)
	release := make(chan struct{})
	service.beforeIdempotencyLock = func() {
		ready <- struct{}{}
		<-release
	}

	results := make(chan Provider, requests)
	errs := make(chan error, requests)
	secret := "concurrent-provider-secret"
	request := ProviderCreate{Name: "concurrent-provider", ProviderType: "openai_compatible", BaseURL: "https://api.example.test/v1", DefaultModel: "gpt-4.1-mini", TimeoutSeconds: 30, Secret: &secret}
	var workers sync.WaitGroup
	for range requests {
		workers.Add(1)
		go func() {
			defer workers.Done()
			provider, createErr := service.CreateProvider(ctx, request, "concurrent-provider-key")
			results <- provider
			errs <- createErr
		}()
	}
	for range requests {
		<-ready
	}
	close(release)
	workers.Wait()
	close(results)
	close(errs)

	var id string
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent create returned %v", err)
		}
	}
	for provider := range results {
		if id == "" {
			id = provider.ID.String()
		} else if provider.ID.String() != id {
			t.Fatalf("concurrent create returned different IDs: %s and %s", id, provider.ID)
		}
	}
	for table, query := range map[string]string{
		"provider":    "SELECT COUNT(*) FROM llm_provider_configurations WHERE name='concurrent-provider'",
		"audit":       "SELECT COUNT(*) FROM audit_logs WHERE subject_id=$1 AND action='llm_provider.create'",
		"idempotency": "SELECT COUNT(*) FROM idempotency_records WHERE scope='llm-provider:create' AND idempotency_key='concurrent-provider-key'",
	} {
		var count int
		var queryErr error
		if table == "provider" || table == "idempotency" {
			queryErr = pool.QueryRow(ctx, query).Scan(&count)
		} else {
			queryErr = pool.QueryRow(ctx, query, id).Scan(&count)
		}
		if queryErr != nil || count != 1 {
			t.Fatalf("%s count=%d err=%v", table, count, queryErr)
		}
	}
	conflicting := request
	conflicting.Name = "different-payload"
	if _, err = service.CreateProvider(ctx, conflicting, "concurrent-provider-key"); !errors.Is(err, ErrIdempotency) {
		t.Fatalf("different payload error=%v, want ErrIdempotency", err)
	}
}

func globalConfigIntegrationDatabase(t *testing.T) (*pgxpool.Pool, context.Context) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	database := fmt.Sprintf("ai_content_factory_i12_globalconfig_%d", time.Now().UTC().UnixNano())
	admin, err := pgx.Connect(ctx, "postgres://postgres:postgres@127.0.0.1:15433/postgres?sslmode=disable")
	if err != nil {
		if os.Getenv("REQUIRE_POSTGRES_INTEGRATION") == "1" {
			t.Fatalf("PostgreSQL integration is required: %v", err)
		}
		t.Skipf("PostgreSQL integration test skipped: %v", err)
	}
	if _, err = admin.Exec(ctx, "CREATE DATABASE "+database); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), "DROP DATABASE IF EXISTS "+database+" WITH (FORCE)")
		_ = admin.Close(context.Background())
	})
	pool, err := pgxpool.New(ctx, "postgres://postgres:postgres@127.0.0.1:15433/"+database+"?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	files, err := filepath.Glob(filepath.Join("..", "..", "migrations", "*.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	for _, file := range files {
		sql, readErr := os.ReadFile(file)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if _, execErr := pool.Exec(ctx, string(sql)); execErr != nil {
			t.Fatalf("apply %s: %v", file, execErr)
		}
	}
	return pool, ctx
}
