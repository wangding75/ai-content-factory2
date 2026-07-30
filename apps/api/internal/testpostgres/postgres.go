package testpostgres

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const DatabaseName = "ai_content_factory"

func Open(t *testing.T) (*pgxpool.Pool, context.Context) {
	t.Helper()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("DATABASE_URL is not set; PostgreSQL integration tests require the existing ai_content_factory database")
	}

	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("parse DATABASE_URL for database %q: %v", DatabaseName, err)
	}
	if config.ConnConfig.Database != DatabaseName {
		t.Fatalf("DATABASE_URL targets database %q; PostgreSQL integration tests must use database %q", config.ConnConfig.Database, DatabaseName)
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
