package main

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var iteration13MigrationDBName = regexp.MustCompile(`^ai_content_factory_i13_migration_[a-z0-9_]+$`)

func openIteration13MigrationDB(t *testing.T) (*pgxpool.Pool, context.Context, string) {
	t.Helper()
	raw := "postgres://postgres:postgres@127.0.0.1:15433/postgres?sslmode=disable"
	database := fmt.Sprintf("ai_content_factory_i13_migration_%d", time.Now().UTC().UnixNano())
	if !iteration13MigrationDBName.MatchString(database) {
		t.Fatal("unsafe generated database name")
	}
	adminURL := raw
	targetURL := fmt.Sprintf("postgres://postgres:postgres@127.0.0.1:15433/%s?sslmode=disable", database)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	admin, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(ctx)
	if _, err = admin.Exec(ctx, "CREATE DATABASE "+database); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		c, e := pgx.Connect(context.Background(), adminURL)
		if e == nil {
			_, _ = c.Exec(context.Background(), "DROP DATABASE IF EXISTS "+database+" WITH (FORCE)")
			_ = c.Close(context.Background())
		}
	})
	conn, err := pgx.Connect(ctx, targetURL)
	if err != nil {
		t.Fatal(err)
	}
	migrations := iteration13Migrations(t)
	if err = ensureSchemaMigrations(ctx, conn); err != nil {
		t.Fatal(err)
	}
	if err = migrateUp(ctx, conn, migrations); err != nil {
		t.Fatalf("migrate fresh database to latest: %v", err)
	}
	_ = conn.Close(ctx)
	db, err := pgxpool.New(ctx, targetURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	var version int
	if err = db.QueryRow(ctx, "SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&version); err != nil || version < 10 {
		t.Fatalf("migration version=%d err=%v, want >= 10", version, err)
	}
	t.Logf("Iteration 13 migration test database: %s", database)
	return db, ctx, targetURL
}

func iteration13Migrations(t *testing.T) []migration {
	t.Helper()
	migrations, err := loadMigrations(filepath.Join("..", "..", "migrations"))
	if err != nil {
		t.Fatal(err)
	}
	return migrations
}

func TestIteration13Migration000010SchemaFinalState(t *testing.T) {
	db, ctx, targetURL := openIteration13MigrationDB(t)

	t.Run("table_exists", func(t *testing.T) {
		var exists bool
		if err := db.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema=current_schema() AND table_name='project_workflow_bindings')").Scan(&exists); err != nil || !exists {
			t.Fatalf("project_workflow_bindings table exists=%t err=%v", exists, err)
		}
	})

	t.Run("columns_and_types", func(t *testing.T) {
		rows, err := db.Query(ctx, "SELECT column_name, data_type, is_nullable, column_default FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='project_workflow_bindings' ORDER BY ordinal_position")
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		type col struct {
			name, dataType, nullable, defaultVal string
		}
		var cols []col
		for rows.Next() {
			var c col
			var d *string
			if err := rows.Scan(&c.name, &c.dataType, &c.nullable, &d); err != nil {
				t.Fatal(err)
			}
			if d != nil {
				c.defaultVal = *d
			}
			cols = append(cols, c)
		}
		if len(cols) != 7 {
			t.Fatalf("expected 7 columns, got %d: %+v", len(cols), cols)
		}
		expected := map[string]struct {
			dataType string
			nullable string
		}{
			"id":                        {"uuid", "NO"},
			"project_id":                {"uuid", "NO"},
			"stage":                     {"text", "NO"},
			"workflow_configuration_id": {"uuid", "NO"},
			"version":                   {"integer", "NO"},
			"created_at":                {"timestamp with time zone", "NO"},
			"updated_at":                {"timestamp with time zone", "NO"},
		}
		for _, c := range cols {
			exp, ok := expected[c.name]
			if !ok {
				t.Fatalf("unexpected column %q", c.name)
			}
			if c.dataType != exp.dataType {
				t.Fatalf("column %s data_type=%q, want %q", c.name, c.dataType, exp.dataType)
			}
			if c.nullable != exp.nullable {
				t.Fatalf("column %s is_nullable=%q, want %q", c.name, c.nullable, exp.nullable)
			}
		}
	})

	t.Run("version_default", func(t *testing.T) {
		var defaultVal *string
		if err := db.QueryRow(ctx, "SELECT column_default FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='project_workflow_bindings' AND column_name='version'").Scan(&defaultVal); err != nil {
			t.Fatal(err)
		}
		if defaultVal == nil || *defaultVal != "1" {
			t.Fatalf("version default = %v, want 1", defaultVal)
		}
	})

	t.Run("stage_check_constraint", func(t *testing.T) {
		var exists bool
		if err := db.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='project_workflow_bindings_stage_check')").Scan(&exists); err != nil || !exists {
			t.Fatalf("stage check constraint exists=%t err=%v", exists, err)
		}
	})

	t.Run("version_check_constraint", func(t *testing.T) {
		var exists bool
		if err := db.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='project_workflow_bindings_version_check')").Scan(&exists); err != nil || !exists {
			t.Fatalf("version check constraint exists=%t err=%v", exists, err)
		}
	})

	t.Run("unique_project_stage_constraint", func(t *testing.T) {
		var exists bool
		if err := db.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='project_workflow_bindings_project_id_stage_key')").Scan(&exists); err != nil || !exists {
			t.Fatalf("unique project_id+stage constraint exists=%t err=%v", exists, err)
		}
	})

	t.Run("foreign_key_project_id", func(t *testing.T) {
		var exists bool
		if err := db.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='project_workflow_bindings_project_id_fkey')").Scan(&exists); err != nil || !exists {
			t.Fatalf("project_id FK exists=%t err=%v", exists, err)
		}
	})

	t.Run("foreign_key_workflow_configuration_id", func(t *testing.T) {
		var exists bool
		if err := db.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='project_workflow_bindings_workflow_configuration_id_fkey')").Scan(&exists); err != nil || !exists {
			t.Fatalf("workflow_configuration_id FK exists=%t err=%v", exists, err)
		}
	})

	t.Run("index_workflow_configuration_id", func(t *testing.T) {
		var exists bool
		if err := db.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_indexes WHERE schemaname=current_schema() AND indexname='project_workflow_bindings_workflow_configuration_id_idx')").Scan(&exists); err != nil || !exists {
			t.Fatalf("workflow_configuration_id index exists=%t err=%v", exists, err)
		}
	})

	t.Run("migration_version_is_10", func(t *testing.T) {
		var version int
		if err := db.QueryRow(ctx, "SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&version); err != nil || version < 10 {
			t.Fatalf("schema_migrations max version=%d err=%v, want >= 10", version, err)
		}
	})

	t.Run("repeat_migration_up_is_idempotent", func(t *testing.T) {
		conn, err := pgx.Connect(ctx, targetURL)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close(ctx)
		migrations := iteration13Migrations(t)
		if err = migrateUp(ctx, conn, migrations); err != nil {
			t.Fatalf("repeat migrate up: %v", err)
		}
		var version int
		if err = db.QueryRow(ctx, "SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&version); err != nil || version < 10 {
			t.Fatalf("version after repeat up=%d err=%v, want >= 10", version, err)
		}
	})
}

func TestIteration13Migration000010SmokeTest(t *testing.T) {
	db, ctx, _ := openIteration13MigrationDB(t)

	projectID := uuid.New()
	otherProjectID := uuid.New()
	connID := uuid.New()
	workflowID := uuid.New()
	otherWorkflowID := uuid.New()

	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)

	if _, err = tx.Exec(ctx, "INSERT INTO projects(id, name, type, created_by) VALUES($1, 'i13-project', 'novel', 'i13-migration')", projectID); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, "INSERT INTO projects(id, name, type, created_by) VALUES($1, 'i13-other', 'novel', 'i13-migration')", otherProjectID); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, "INSERT INTO workflow_connections(id, name, connection_type, base_url, auth_type, timeout_seconds, type_config) VALUES($1, 'i13-conn', 'n8n', 'https://n8n.example.com', 'api_key', 30, '{\"referenceType\":\"workflow_id\",\"referenceValue\":\"wf-1\"}'::jsonb)", connID); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, "INSERT INTO workflow_configurations(id, name, connection_id, applicable_stages, type_config, input_contract_version, output_contract_version) VALUES($1, 'i13-wf-1', $2, '[\"chapter_planning\",\"content_generation\",\"review\",\"rewrite\"]'::jsonb, '{\"referenceType\":\"workflow_id\",\"referenceValue\":\"wf-1\"}'::jsonb, '1.0', '1.0')", workflowID, connID); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, "INSERT INTO workflow_configurations(id, name, connection_id, applicable_stages, type_config, input_contract_version, output_contract_version) VALUES($1, 'i13-wf-2', $2, '[\"review\"]'::jsonb, '{\"referenceType\":\"workflow_id\",\"referenceValue\":\"wf-2\"}'::jsonb, '1.0', '1.0')", otherWorkflowID, connID); err != nil {
		t.Fatal(err)
	}

	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec(context.Background(), "DELETE FROM projects WHERE created_by='i13-migration'")
		_, _ = db.Exec(context.Background(), "DELETE FROM workflow_configurations WHERE name IN ('i13-wf-1','i13-wf-2')")
		_, _ = db.Exec(context.Background(), "DELETE FROM workflow_connections WHERE name='i13-conn'")
	})

	t.Run("insert_four_legal_stages", func(t *testing.T) {
		for _, stage := range []string{"chapter_planning", "content_generation", "review", "rewrite"} {
			if _, err := db.Exec(ctx, "INSERT INTO project_workflow_bindings(id, project_id, stage, workflow_configuration_id) VALUES($1, $2, $3, $4)", uuid.New(), projectID, stage, workflowID); err != nil {
				t.Fatalf("insert legal stage %s: %v", stage, err)
			}
		}
	})

	t.Run("reject_duplicate_project_stage", func(t *testing.T) {
		stage := "chapter_planning"
		_, err := db.Exec(ctx, "INSERT INTO project_workflow_bindings(id, project_id, stage, workflow_configuration_id) VALUES($1, $2, $3, $4)", uuid.New(), projectID, stage, workflowID)
		if err == nil {
			t.Fatal("expected duplicate project+stage to be rejected")
		}
	})

	t.Run("allow_different_project_same_stage", func(t *testing.T) {
		if _, err := db.Exec(ctx, "INSERT INTO project_workflow_bindings(id, project_id, stage, workflow_configuration_id) VALUES($1, $2, 'chapter_planning', $3)", uuid.New(), otherProjectID, workflowID); err != nil {
			t.Fatalf("insert different project same stage: %v", err)
		}
	})

	t.Run("reject_illegal_stage", func(t *testing.T) {
		_, err := db.Exec(ctx, "INSERT INTO project_workflow_bindings(id, project_id, stage, workflow_configuration_id) VALUES($1, $2, 'invalid_stage', $3)", uuid.New(), projectID, workflowID)
		if err == nil {
			t.Fatal("expected invalid stage to be rejected")
		}
	})

	t.Run("reject_version_zero", func(t *testing.T) {
		_, err := db.Exec(ctx, "INSERT INTO project_workflow_bindings(id, project_id, stage, workflow_configuration_id, version) VALUES($1, $2, 'content_generation', $3, 0)", uuid.New(), projectID, otherWorkflowID)
		if err == nil {
			t.Fatal("expected version=0 to be rejected")
		}
	})

	t.Run("reject_version_negative", func(t *testing.T) {
		_, err := db.Exec(ctx, "INSERT INTO project_workflow_bindings(id, project_id, stage, workflow_configuration_id, version) VALUES($1, $2, 'content_generation', $3, -1)", uuid.New(), projectID, otherWorkflowID)
		if err == nil {
			t.Fatal("expected version=-1 to be rejected")
		}
	})

	t.Run("cascade_delete_project", func(t *testing.T) {
		cascadeProjectID := uuid.New()
		if _, err := db.Exec(ctx, "INSERT INTO projects(id, name, type, created_by) VALUES($1, 'i13-cascade', 'novel', 'i13-migration')", cascadeProjectID); err != nil {
			t.Fatal(err)
		}
		bindingID := uuid.New()
		if _, err := db.Exec(ctx, "INSERT INTO project_workflow_bindings(id, project_id, stage, workflow_configuration_id) VALUES($1, $2, 'review', $3)", bindingID, cascadeProjectID, workflowID); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(ctx, "DELETE FROM projects WHERE id=$1", cascadeProjectID); err != nil {
			t.Fatal(err)
		}
		var exists bool
		if err := db.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM project_workflow_bindings WHERE id=$1)", bindingID).Scan(&exists); err != nil || exists {
			t.Fatalf("binding after cascade delete exists=%t err=%v", exists, err)
		}
	})

	t.Run("restrict_delete_referenced_workflow", func(t *testing.T) {
		restrictWorkflowID := uuid.New()
		if _, err := db.Exec(ctx, "INSERT INTO workflow_configurations(id, name, connection_id, applicable_stages, type_config, input_contract_version, output_contract_version) VALUES($1, 'i13-restrict', $2, '[\"rewrite\"]'::jsonb, '{\"referenceType\":\"workflow_id\",\"referenceValue\":\"wf-r\"}'::jsonb, '1.0', '1.0')", restrictWorkflowID, connID); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_, _ = db.Exec(context.Background(), "DELETE FROM workflow_configurations WHERE name='i13-restrict'")
		})
		if _, err := db.Exec(ctx, "INSERT INTO project_workflow_bindings(id, project_id, stage, workflow_configuration_id) VALUES($1, $2, 'rewrite', $3)", uuid.New(), otherProjectID, restrictWorkflowID); err != nil {
			t.Fatal(err)
		}
		_, err := db.Exec(ctx, "DELETE FROM workflow_configurations WHERE id=$1", restrictWorkflowID)
		if err == nil {
			t.Fatal("expected FK restrict to block workflow_configuration delete")
		}
		t.Logf("restrict delete error (expected): %v", err)
	})
}