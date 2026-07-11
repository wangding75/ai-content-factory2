package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

var migrationFilePattern = regexp.MustCompile(`^(\d+)_.+\.(up|down)\.sql$`)

type migration struct {
	version int
	up      string
	down    string
}

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: migrate <up|down 1|version>")
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	migrations, err := loadMigrations(migrationsDir())
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	defer conn.Close(ctx)

	if err := ensureSchemaMigrations(ctx, conn); err != nil {
		log.Fatal(err)
	}

	switch os.Args[1] {
	case "up":
		if len(os.Args) != 2 {
			log.Fatal("usage: migrate up")
		}
		if err := migrateUp(ctx, conn, migrations); err != nil {
			log.Fatal(err)
		}
	case "down":
		if len(os.Args) != 3 || os.Args[2] != "1" {
			log.Fatal("usage: migrate down 1")
		}
		if err := migrateDownOne(ctx, conn, migrations); err != nil {
			log.Fatal(err)
		}
	case "version":
		if len(os.Args) != 2 {
			log.Fatal("usage: migrate version")
		}
		version, err := currentVersion(ctx, conn)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(version)
	default:
		log.Fatalf("unsupported command: %s", os.Args[1])
	}
}

func migrationsDir() string {
	if directory := os.Getenv("MIGRATIONS_DIR"); directory != "" {
		return directory
	}
	return "migrations"
}

func loadMigrations(directory string) ([]migration, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, fmt.Errorf("read migrations directory: %w", err)
	}

	byVersion := map[int]*migration{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		matches := migrationFilePattern.FindStringSubmatch(entry.Name())
		if matches == nil {
			continue
		}

		version, err := strconv.Atoi(matches[1])
		if err != nil {
			return nil, fmt.Errorf("parse migration version %q: %w", entry.Name(), err)
		}
		contents, err := os.ReadFile(filepath.Join(directory, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("read migration %q: %w", entry.Name(), err)
		}

		item := byVersion[version]
		if item == nil {
			item = &migration{version: version}
			byVersion[version] = item
		}
		if matches[2] == "up" {
			item.up = string(contents)
		} else {
			item.down = string(contents)
		}
	}

	versions := make([]int, 0, len(byVersion))
	for version, item := range byVersion {
		if strings.TrimSpace(item.up) == "" || strings.TrimSpace(item.down) == "" {
			return nil, fmt.Errorf("migration %06d must have both up and down files", version)
		}
		versions = append(versions, version)
	}
	sort.Ints(versions)

	result := make([]migration, 0, len(versions))
	for _, version := range versions {
		result = append(result, *byVersion[version])
	}
	return result, nil
}

func ensureSchemaMigrations(ctx context.Context, conn *pgx.Conn) error {
	_, err := conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version BIGINT PRIMARY KEY,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
	return err
}

func currentVersion(ctx context.Context, conn *pgx.Conn) (int, error) {
	var version int
	err := conn.QueryRow(ctx, "SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&version)
	return version, err
}

func migrateUp(ctx context.Context, conn *pgx.Conn, migrations []migration) error {
	current, err := currentVersion(ctx, conn)
	if err != nil {
		return err
	}
	for _, migration := range migrations {
		if migration.version <= current {
			continue
		}
		if err := runMigration(ctx, conn, migration.version, migration.up, true); err != nil {
			return fmt.Errorf("apply migration %06d: %w", migration.version, err)
		}
	}
	return nil
}

func migrateDownOne(ctx context.Context, conn *pgx.Conn, migrations []migration) error {
	current, err := currentVersion(ctx, conn)
	if err != nil {
		return err
	}
	if current == 0 {
		return errors.New("no migration is currently applied")
	}

	for _, migration := range migrations {
		if migration.version == current {
			if err := runMigration(ctx, conn, migration.version, migration.down, false); err != nil {
				return fmt.Errorf("revert migration %06d: %w", migration.version, err)
			}
			return nil
		}
	}
	return fmt.Errorf("migration %06d is applied but not available on disk", current)
}

func runMigration(ctx context.Context, conn *pgx.Conn, version int, sql string, up bool) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if _, err := tx.Exec(ctx, sql); err != nil {
		return err
	}
	if up {
		_, err = tx.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", version)
	} else {
		_, err = tx.Exec(ctx, "DELETE FROM schema_migrations WHERE version = $1", version)
	}
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
