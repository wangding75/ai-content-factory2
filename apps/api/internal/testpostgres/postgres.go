package testpostgres

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const DatabaseName = "ai_content_factory_http_test"

func Open(t *testing.T) (*pgxpool.Pool, context.Context) {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set; PostgreSQL integration test skipped")
	}
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("parse TEST_DATABASE_URL for database %q: %v", DatabaseName, err)
	}
	if config.ConnConfig.Database != DatabaseName {
		t.Skipf("TEST_DATABASE_URL targets database %q, not repository integration database %q; PostgreSQL integration test skipped", config.ConnConfig.Database, DatabaseName)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatalf("connect PostgreSQL database %q: %v", config.ConnConfig.Database, err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping PostgreSQL database %q: %v", config.ConnConfig.Database, err)
	}
	return pool, ctx
}
