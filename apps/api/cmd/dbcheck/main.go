package main

import (
	"context"
	"fmt"
	"net/url"
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

type checkResult struct {
	name     string
	expected string
	actual   string
	pass     bool
}

func main() {
	os.Exit(run())
}

func run() int {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		fmt.Fprintln(os.Stderr, "FAIL: DATABASE_URL is required")
		return 1
	}

	// Parse and validate database name
	dbName, err := parseDatabaseName(databaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: cannot parse DATABASE_URL: %v\n", err)
		return 1
	}
	if dbName != "ai_content_factory" {
		fmt.Fprintf(os.Stderr, "FAIL: database name must be 'ai_content_factory', got '%s'\n", dbName)
		return 1
	}

	// Locate migrations directory
	migrationsDir := filepath.Join("migrations")

	// Load migrations from disk
	diskMigrations, err := loadMigrations(migrationsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: load migrations: %v\n", err)
		return 1
	}

	diskMaxVersion := diskMigrations[len(diskMigrations)-1].version

	// Connect to database
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: connect to database: %v\n", err)
		return 1
	}
	defer conn.Close(ctx)

	if err := conn.Ping(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: database ping: %v\n", err)
		return 1
	}

	// === Migration state check ===
	dbVersion, dirty, err := checkMigrationState(ctx, conn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: migration state check: %v\n", err)
		return 1
	}

	if dirty {
		fmt.Fprintln(os.Stderr, "FAIL: schema_migrations is dirty")
		return 1
	}

	if dbVersion != diskMaxVersion {
		fmt.Fprintf(os.Stderr, "FAIL: database version %d != disk version %d\n", dbVersion, diskMaxVersion)
		return 1
	}

	fmt.Printf("Migration check: db=%d disk=%d dirty=%v PASS\n", dbVersion, diskMaxVersion, dirty)

	// === Schema structure checks ===
	results := runSchemaChecks(ctx, conn)
	allPass := true
	for _, r := range results {
		status := "PASS"
		if !r.pass {
			status = "FAIL"
			allPass = false
		}
		fmt.Printf("  %s: %s\n", status, r.name)
		if !r.pass {
			fmt.Fprintf(os.Stderr, "    expected: %s\n", r.expected)
			fmt.Fprintf(os.Stderr, "    actual:   %s\n", r.actual)
		}
	}

	passCount := 0
	failCount := 0
	for _, r := range results {
		if r.pass {
			passCount++
		} else {
			failCount++
		}
	}
	fmt.Printf("Schema checks: %d total, %d passed, %d failed\n", len(results), passCount, failCount)

	if !allPass {
		return 1
	}

	fmt.Println("PASS")
	return 0
}

func parseDatabaseName(databaseURL string) (string, error) {
	u, err := url.Parse(databaseURL)
	if err != nil {
		return "", err
	}
	path := strings.TrimPrefix(u.Path, "/")
	return path, nil
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

func checkMigrationState(ctx context.Context, conn *pgx.Conn) (version int, dirty bool, err error) {
	// Check if schema_migrations table exists
	var exists bool
	err = conn.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = current_schema() AND table_name = 'schema_migrations')").Scan(&exists)
	if err != nil {
		return 0, false, fmt.Errorf("check schema_migrations existence: %w", err)
	}
	if !exists {
		return 0, false, fmt.Errorf("schema_migrations table does not exist")
	}

	// Get current version (max version in schema_migrations)
	err = conn.QueryRow(ctx, "SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&version)
	if err != nil {
		return 0, false, fmt.Errorf("read current version: %w", err)
	}

	// The schema_migrations table has (version BIGINT PRIMARY KEY, applied_at TIMESTAMPTZ)
	// There is no explicit dirty column. The framework is dirty if there is a gap
	// in versions (i.e., some versions are missing while higher ones exist).
	// Check for gaps: count should equal max - min + 1, or if empty, version 0.
	if version > 0 {
		var count int
		err = conn.QueryRow(ctx, "SELECT COUNT(*) FROM schema_migrations").Scan(&count)
		if err != nil {
			return 0, false, fmt.Errorf("count schema_migrations: %w", err)
		}
		var minVersion int
		err = conn.QueryRow(ctx, "SELECT MIN(version) FROM schema_migrations").Scan(&minVersion)
		if err != nil {
			return 0, false, fmt.Errorf("read min version: %w", err)
		}
		expectedCount := version - minVersion + 1
		if count != expectedCount {
			dirty = true
		}
	}

	return version, dirty, nil
}

// runSchemaChecks runs all schema checks for iteration 15-18 migrations.
func runSchemaChecks(ctx context.Context, conn *pgx.Conn) []checkResult {
	var results []checkResult

	// --- 000015: content_generation_data_foundation ---

	// workflow_run_records: subject_type column
	results = append(results, checkColumn(ctx, conn, "workflow_run_records", "subject_type",
		"character varying", "YES", "NULL"))

	// workflow_run_records: subject_id column
	results = append(results, checkColumn(ctx, conn, "workflow_run_records", "subject_id",
		"uuid", "YES", "NULL"))

	// workflow_run_records: subject_pair_check constraint
	results = append(results, checkConstraintExists(ctx, conn, "workflow_run_records_subject_pair_check"))

	// workflow_run_records: active_content_generation_subject unique index
	results = append(results, checkIndexExists(ctx, conn, "workflow_run_records_active_content_generation_subject_idx"))

	// workflow_run_records: project_subject_stage_created_at_id index
	results = append(results, checkIndexExists(ctx, conn, "workflow_run_records_project_subject_stage_created_at_id_idx"))

	// content_versions: source_content_version_id column
	results = append(results, checkColumn(ctx, conn, "content_versions", "source_content_version_id",
		"uuid", "YES", "NULL"))

	// content_versions: source_content_version_version column
	results = append(results, checkColumn(ctx, conn, "content_versions", "source_content_version_version",
		"integer", "YES", "NULL"))

	// content_versions: source_workflow_run_id column
	results = append(results, checkColumn(ctx, conn, "content_versions", "source_workflow_run_id",
		"uuid", "YES", "NULL"))

	// content_versions: source_content_version_fk FK
	results = append(results, checkConstraintExists(ctx, conn, "content_versions_source_content_version_fk"))

	// content_versions: source_workflow_run_id_fkey FK
	results = append(results, checkConstraintExists(ctx, conn, "content_versions_source_workflow_run_id_fkey"))

	// content_versions: source_workflow_run_unique_idx unique index
	results = append(results, checkIndexExists(ctx, conn, "content_versions_source_workflow_run_unique_idx"))

	// content_versions: content_item_source_version_no_id index
	results = append(results, checkIndexExists(ctx, conn, "content_versions_content_item_source_version_no_id_idx"))

	// workflow_run_events: event_type_check constraint (includes 'output_validation_failed' from 000016)
	results = append(results, checkConstraintExists(ctx, conn, "workflow_run_events_event_type_check"))

	// --- 000017: real_content_review_foundation ---

	// workflow_run_records: active_review_subject unique index
	results = append(results, checkIndexExists(ctx, conn, "workflow_run_records_active_review_subject_idx"))

	// review_reports: schema_version column
	results = append(results, checkColumn(ctx, conn, "review_reports", "schema_version",
		"character varying", "YES", "NULL"))

	// review_reports: source_content_version_version column
	results = append(results, checkColumn(ctx, conn, "review_reports", "source_content_version_version",
		"integer", "YES", "NULL"))

	// review_reports: source_content_hash column
	results = append(results, checkColumn(ctx, conn, "review_reports", "source_content_hash",
		"character", "YES", "NULL"))

	// review_reports: workflow_run_id nullable (was changed to DROP NOT NULL)
	results = append(results, checkColumn(ctx, conn, "review_reports", "workflow_run_id",
		"uuid", "YES", "NULL"))

	// review_reports: score nullable (was changed to DROP NOT NULL)
	results = append(results, checkColumn(ctx, conn, "review_reports", "score",
		"integer", "YES", "NULL"))

	// review_reports: provider_key_check constraint
	results = append(results, checkConstraintExists(ctx, conn, "review_reports_provider_key_check"))

	// review_reports: conclusion_check constraint
	results = append(results, checkConstraintExists(ctx, conn, "review_reports_conclusion_check"))

	// review_reports: runtime_shape_check constraint
	results = append(results, checkConstraintExists(ctx, conn, "review_reports_runtime_shape_check"))

	// review_reports: trigger function
	results = append(results, checkFunctionExists(ctx, conn, "enforce_review_report_workflow_run_scope"))

	// review_reports: constraint trigger
	results = append(results, checkTriggerExists(ctx, conn, "review_reports_workflow_run_scope_trigger"))

	// review_findings: issue_key column
	results = append(results, checkColumn(ctx, conn, "review_findings", "issue_key",
		"character varying", "YES", "NULL"))

	// review_findings: category_label column
	results = append(results, checkColumn(ctx, conn, "review_findings", "category_label",
		"character varying", "YES", "NULL"))

	// review_findings: evidence_json column
	results = append(results, checkColumn(ctx, conn, "review_findings", "evidence_json",
		"jsonb", "YES", "NULL"))

	// review_findings: suggestion column
	results = append(results, checkColumn(ctx, conn, "review_findings", "suggestion",
		"text", "YES", "NULL"))

	// review_findings: disposition column
	results = append(results, checkColumn(ctx, conn, "review_findings", "disposition",
		"character varying", "NO", "'open'::character varying"))

	// review_findings: version column
	results = append(results, checkColumn(ctx, conn, "review_findings", "version",
		"integer", "NO", "1"))

	// review_findings: ignored_at column
	results = append(results, checkColumn(ctx, conn, "review_findings", "ignored_at",
		"timestamp with time zone", "YES", "NULL"))

	// review_findings: ignored_by column
	results = append(results, checkColumn(ctx, conn, "review_findings", "ignored_by",
		"character varying", "YES", "NULL"))

	// review_findings: updated_at column
	results = append(results, checkColumn(ctx, conn, "review_findings", "updated_at",
		"timestamp with time zone", "NO", "now()"))

	// review_findings: category_check constraint
	results = append(results, checkConstraintExists(ctx, conn, "review_findings_category_check"))

	// review_findings: severity_check constraint
	results = append(results, checkConstraintExists(ctx, conn, "review_findings_severity_check"))

	// review_findings: position_check constraint
	results = append(results, checkConstraintExists(ctx, conn, "review_findings_position_check"))

	// review_findings: issue_key_check constraint
	results = append(results, checkConstraintExists(ctx, conn, "review_findings_issue_key_check"))

	// review_findings: category_label_check constraint
	results = append(results, checkConstraintExists(ctx, conn, "review_findings_category_label_check"))

	// review_findings: evidence_json_check constraint
	results = append(results, checkConstraintExists(ctx, conn, "review_findings_evidence_json_check"))

	// review_findings: suggestion_check constraint
	results = append(results, checkConstraintExists(ctx, conn, "review_findings_suggestion_check"))

	// review_findings: disposition_check constraint
	results = append(results, checkConstraintExists(ctx, conn, "review_findings_disposition_check"))

	// review_findings: version_check constraint
	results = append(results, checkConstraintExists(ctx, conn, "review_findings_version_check"))

	// review_findings: ignored_shape_check constraint
	results = append(results, checkConstraintExists(ctx, conn, "review_findings_ignored_shape_check"))

	// review_findings: runtime_shape_check constraint
	results = append(results, checkConstraintExists(ctx, conn, "review_findings_runtime_shape_check"))

	// review_findings: review_issue_key_unique unique constraint
	results = append(results, checkConstraintExists(ctx, conn, "review_findings_review_issue_key_unique"))

	// review_findings: review_severity_disposition_position index
	results = append(results, checkIndexExists(ctx, conn, "review_findings_review_severity_disposition_position_idx"))

	// --- 000018: real_content_rewrite_foundation ---

	// workflow_run_records: rewrite_subject_shape_check constraint
	results = append(results, checkConstraintExists(ctx, conn, "workflow_run_records_rewrite_subject_shape_check"))

	// workflow_run_records: active_rewrite_subject unique index
	results = append(results, checkIndexExists(ctx, conn, "workflow_run_records_active_rewrite_subject_idx"))

	// workflow_run_records: enforce_rewrite_workflow_run_scope function
	results = append(results, checkFunctionExists(ctx, conn, "enforce_rewrite_workflow_run_scope"))

	// workflow_run_records: rewrite_scope_trigger constraint trigger
	results = append(results, checkTriggerExists(ctx, conn, "workflow_run_records_rewrite_scope_trigger"))

	// content_versions: enforce_workflow_rewrite_candidate_scope function
	results = append(results, checkFunctionExists(ctx, conn, "enforce_workflow_rewrite_candidate_scope"))

	// content_versions: workflow_rewrite_scope_trigger constraint trigger
	results = append(results, checkTriggerExists(ctx, conn, "content_versions_workflow_rewrite_scope_trigger"))

	// content_versions: workflow_rewrite_shape constraint
	results = append(results, checkConstraintExists(ctx, conn, "content_versions_workflow_rewrite_shape"))

	return results
}

// checkColumn checks that a column exists with the expected type, nullable, and default.
func checkColumn(ctx context.Context, conn *pgx.Conn, table, column, expectedType, isNullable, expectedDefault string) checkResult {
	name := fmt.Sprintf("column %s.%s", table, column)
	var dataType, nullable string
	var columnDefault *string
	err := conn.QueryRow(ctx, `
		SELECT data_type, is_nullable, column_default
		FROM information_schema.columns
		WHERE table_schema = current_schema()
		  AND table_name = $1
		  AND column_name = $2
	`, table, column).Scan(&dataType, &nullable, &columnDefault)

	if err != nil {
		return checkResult{
			name:     name,
			expected: fmt.Sprintf("type=%s nullable=%s default=%s", expectedType, isNullable, expectedDefault),
			actual:   fmt.Sprintf("MISSING: %v", err),
			pass:     false,
		}
	}

	actualDefault := "NULL"
	if columnDefault != nil {
		actualDefault = *columnDefault
	}

	typeMatch := strings.EqualFold(dataType, expectedType)
	nullableMatch := nullable == isNullable

	// For default comparison, handle extra whitespace/casting
	defaultMatch := defaultMatches(actualDefault, expectedDefault)

	pass := typeMatch && nullableMatch && defaultMatch
	actual := fmt.Sprintf("type=%s nullable=%s default=%s", dataType, nullable, actualDefault)
	expected := fmt.Sprintf("type=%s nullable=%s default=%s", expectedType, isNullable, expectedDefault)

	return checkResult{name: name, expected: expected, actual: actual, pass: pass}
}

func defaultMatches(actual, expected string) bool {
	// Normalize NULL
	if expected == "NULL" {
		return actual == "NULL" || actual == ""
	}
	// Strip ::type casting from actual for comparison
	actual = strings.TrimSpace(actual)
	expected = strings.TrimSpace(expected)
	// Handle cases like 'open'::character varying vs 'open'::character varying
	// and 1 vs 1
	if strings.EqualFold(actual, expected) {
		return true
	}
	// Strip type casting
	actualNoCast := regexp.MustCompile(`::.*$`).ReplaceAllString(actual, "")
	expectedNoCast := regexp.MustCompile(`::.*$`).ReplaceAllString(expected, "")
	return strings.EqualFold(strings.TrimSpace(actualNoCast), strings.TrimSpace(expectedNoCast))
}

func checkConstraintExists(ctx context.Context, conn *pgx.Conn, constraintName string) checkResult {
	var exists bool
	err := conn.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM pg_constraint c
			JOIN pg_namespace n ON n.oid = c.connamespace
			WHERE n.nspname = current_schema()
			  AND c.conname = $1
		)
	`, constraintName).Scan(&exists)
	if err != nil || !exists {
		actual := "MISSING"
		if err != nil {
			actual = fmt.Sprintf("ERROR: %v", err)
		}
		return checkResult{
			name:     fmt.Sprintf("constraint %s", constraintName),
			expected: "exists",
			actual:   actual,
			pass:     false,
		}
	}
	return checkResult{
		name:     fmt.Sprintf("constraint %s", constraintName),
		expected: "exists",
		actual:   "exists",
		pass:     true,
	}
}

func checkIndexExists(ctx context.Context, conn *pgx.Conn, indexName string) checkResult {
	var exists bool
	err := conn.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM pg_indexes
			WHERE schemaname = current_schema()
			  AND indexname = $1
		)
	`, indexName).Scan(&exists)
	if err != nil || !exists {
		actual := "MISSING"
		if err != nil {
			actual = fmt.Sprintf("ERROR: %v", err)
		}
		return checkResult{
			name:     fmt.Sprintf("index %s", indexName),
			expected: "exists",
			actual:   actual,
			pass:     false,
		}
	}
	return checkResult{
		name:     fmt.Sprintf("index %s", indexName),
		expected: "exists",
		actual:   "exists",
		pass:     true,
	}
}

func checkFunctionExists(ctx context.Context, conn *pgx.Conn, funcName string) checkResult {
	var exists bool
	err := conn.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM pg_proc p
			JOIN pg_namespace n ON n.oid = p.pronamespace
			WHERE n.nspname = current_schema()
			  AND p.proname = $1
		)
	`, funcName).Scan(&exists)
	if err != nil || !exists {
		actual := "MISSING"
		if err != nil {
			actual = fmt.Sprintf("ERROR: %v", err)
		}
		return checkResult{
			name:     fmt.Sprintf("function %s", funcName),
			expected: "exists",
			actual:   actual,
			pass:     false,
		}
	}
	return checkResult{
		name:     fmt.Sprintf("function %s", funcName),
		expected: "exists",
		actual:   "exists",
		pass:     true,
	}
}

func checkTriggerExists(ctx context.Context, conn *pgx.Conn, triggerName string) checkResult {
	var exists bool
	err := conn.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM pg_trigger t
			WHERE t.tgname = $1
		)
	`, triggerName).Scan(&exists)
	if err != nil || !exists {
		actual := "MISSING"
		if err != nil {
			actual = fmt.Sprintf("ERROR: %v", err)
		}
		return checkResult{
			name:     fmt.Sprintf("trigger %s", triggerName),
			expected: "exists",
			actual:   actual,
			pass:     false,
		}
	}
	return checkResult{
		name:     fmt.Sprintf("trigger %s", triggerName),
		expected: "exists",
		actual:   "exists",
		pass:     true,
	}
}
