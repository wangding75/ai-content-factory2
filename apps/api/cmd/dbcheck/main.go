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

// dataCheck defines a domain data consistency check.
type dataCheck struct {
	ID          string // stable unique check ID
	Name        string // short human-readable name
	SQL         string // read-only SQL returning anomaly count and sample rows
	Description string // what this check validates
}

// dataCheckResult holds the result of a data check.
type dataCheckResult struct {
	ID           string
	Name         string
	Pass         bool
	AnomalyCount int
	Samples      []string // up to 5 sample rows
	Description  string
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

	// Set read-only transaction for data checks
	if _, err := conn.Exec(ctx, "SET TRANSACTION READ ONLY"); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: cannot set read-only transaction: %v\n", err)
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

	// === Domain data consistency checks ===
	dataChecks := defineDataChecks()
	if err := validateDataChecks(dataChecks); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: data check definition error: %v\n", err)
		return 1
	}

	dataResults := runDataChecks(ctx, conn, dataChecks)
	dataAllPass := true
	for _, dr := range dataResults {
		status := "PASS"
		if !dr.Pass {
			status = "FAIL"
			dataAllPass = false
		}
		fmt.Printf("  %s: %s (%s) - anomalies=%d\n", status, dr.ID, dr.Name, dr.AnomalyCount)
		if !dr.Pass {
			for _, s := range dr.Samples {
				fmt.Fprintf(os.Stderr, "    sample: %s\n", s)
			}
		}
	}

	dataPassCount := 0
	dataFailCount := 0
	for _, dr := range dataResults {
		if dr.Pass {
			dataPassCount++
		} else {
			dataFailCount++
		}
	}
	fmt.Printf("Data checks: %d total, %d passed, %d failed\n", len(dataResults), dataPassCount, dataFailCount)

	if !dataAllPass {
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
	var exists bool
	err = conn.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = current_schema() AND table_name = 'schema_migrations')").Scan(&exists)
	if err != nil {
		return 0, false, fmt.Errorf("check schema_migrations existence: %w", err)
	}
	if !exists {
		return 0, false, fmt.Errorf("schema_migrations table does not exist")
	}

	err = conn.QueryRow(ctx, "SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&version)
	if err != nil {
		return 0, false, fmt.Errorf("read current version: %w", err)
	}

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

// runSchemaChecks runs all schema checks for iteration 15-19 migrations.
func runSchemaChecks(ctx context.Context, conn *pgx.Conn) []checkResult {
	var results []checkResult

	// --- 000015: content_generation_data_foundation ---

	results = append(results, checkColumn(ctx, conn, "workflow_run_records", "subject_type",
		"character varying", "YES", "NULL"))
	results = append(results, checkColumn(ctx, conn, "workflow_run_records", "subject_id",
		"uuid", "YES", "NULL"))
	results = append(results, checkConstraintExists(ctx, conn, "workflow_run_records_subject_pair_check"))
	results = append(results, checkIndexExists(ctx, conn, "workflow_run_records_active_content_generation_subject_idx"))
	results = append(results, checkIndexExists(ctx, conn, "workflow_run_records_project_subject_stage_created_at_id_idx"))
	results = append(results, checkColumn(ctx, conn, "content_versions", "source_content_version_id",
		"uuid", "YES", "NULL"))
	results = append(results, checkColumn(ctx, conn, "content_versions", "source_content_version_version",
		"integer", "YES", "NULL"))
	results = append(results, checkColumn(ctx, conn, "content_versions", "source_workflow_run_id",
		"uuid", "YES", "NULL"))
	results = append(results, checkConstraintExists(ctx, conn, "content_versions_source_content_version_fk"))
	results = append(results, checkConstraintExists(ctx, conn, "content_versions_source_workflow_run_id_fkey"))
	results = append(results, checkIndexExists(ctx, conn, "content_versions_source_workflow_run_unique_idx"))
	results = append(results, checkIndexExists(ctx, conn, "content_versions_content_item_source_version_no_id_idx"))
	results = append(results, checkConstraintExists(ctx, conn, "workflow_run_events_event_type_check"))

	// --- 000017: real_content_review_foundation ---

	results = append(results, checkIndexExists(ctx, conn, "workflow_run_records_active_review_subject_idx"))
	results = append(results, checkColumn(ctx, conn, "review_reports", "schema_version",
		"character varying", "YES", "NULL"))
	results = append(results, checkColumn(ctx, conn, "review_reports", "source_content_version_version",
		"integer", "YES", "NULL"))
	results = append(results, checkColumn(ctx, conn, "review_reports", "source_content_hash",
		"character", "YES", "NULL"))
	results = append(results, checkColumn(ctx, conn, "review_reports", "workflow_run_id",
		"uuid", "YES", "NULL"))
	results = append(results, checkColumn(ctx, conn, "review_reports", "score",
		"integer", "YES", "NULL"))
	results = append(results, checkConstraintExists(ctx, conn, "review_reports_provider_key_check"))
	results = append(results, checkConstraintExists(ctx, conn, "review_reports_conclusion_check"))
	results = append(results, checkConstraintExists(ctx, conn, "review_reports_runtime_shape_check"))
	results = append(results, checkFunctionExists(ctx, conn, "enforce_review_report_workflow_run_scope"))
	results = append(results, checkTriggerExists(ctx, conn, "review_reports_workflow_run_scope_trigger"))
	results = append(results, checkColumn(ctx, conn, "review_findings", "issue_key",
		"character varying", "YES", "NULL"))
	results = append(results, checkColumn(ctx, conn, "review_findings", "category_label",
		"character varying", "YES", "NULL"))
	results = append(results, checkColumn(ctx, conn, "review_findings", "evidence_json",
		"jsonb", "YES", "NULL"))
	results = append(results, checkColumn(ctx, conn, "review_findings", "suggestion",
		"text", "YES", "NULL"))
	results = append(results, checkColumn(ctx, conn, "review_findings", "disposition",
		"character varying", "NO", "'open'::character varying"))
	results = append(results, checkColumn(ctx, conn, "review_findings", "version",
		"integer", "NO", "1"))
	results = append(results, checkColumn(ctx, conn, "review_findings", "ignored_at",
		"timestamp with time zone", "YES", "NULL"))
	results = append(results, checkColumn(ctx, conn, "review_findings", "ignored_by",
		"character varying", "YES", "NULL"))
	results = append(results, checkColumn(ctx, conn, "review_findings", "updated_at",
		"timestamp with time zone", "NO", "now()"))
	results = append(results, checkConstraintExists(ctx, conn, "review_findings_category_check"))
	results = append(results, checkConstraintExists(ctx, conn, "review_findings_severity_check"))
	results = append(results, checkConstraintExists(ctx, conn, "review_findings_position_check"))
	results = append(results, checkConstraintExists(ctx, conn, "review_findings_issue_key_check"))
	results = append(results, checkConstraintExists(ctx, conn, "review_findings_category_label_check"))
	results = append(results, checkConstraintExists(ctx, conn, "review_findings_evidence_json_check"))
	results = append(results, checkConstraintExists(ctx, conn, "review_findings_suggestion_check"))
	results = append(results, checkConstraintExists(ctx, conn, "review_findings_disposition_check"))
	results = append(results, checkConstraintExists(ctx, conn, "review_findings_version_check"))
	results = append(results, checkConstraintExists(ctx, conn, "review_findings_ignored_shape_check"))
	results = append(results, checkConstraintExists(ctx, conn, "review_findings_runtime_shape_check"))
	results = append(results, checkConstraintExists(ctx, conn, "review_findings_review_issue_key_unique"))
	results = append(results, checkIndexExists(ctx, conn, "review_findings_review_severity_disposition_position_idx"))

	// --- 000018: real_content_rewrite_foundation ---

	results = append(results, checkConstraintExists(ctx, conn, "workflow_run_records_rewrite_subject_shape_check"))
	results = append(results, checkIndexExists(ctx, conn, "workflow_run_records_active_rewrite_subject_idx"))
	results = append(results, checkFunctionExists(ctx, conn, "enforce_rewrite_workflow_run_scope"))
	results = append(results, checkTriggerExists(ctx, conn, "workflow_run_records_rewrite_scope_trigger"))
	results = append(results, checkFunctionExists(ctx, conn, "enforce_workflow_rewrite_candidate_scope"))
	results = append(results, checkTriggerExists(ctx, conn, "content_versions_workflow_rewrite_scope_trigger"))
	results = append(results, checkConstraintExists(ctx, conn, "content_versions_workflow_rewrite_shape"))

	// --- 000019: iteration_19_real_integration ---

	results = append(results, checkColumn(ctx, conn, "llm_provider_configurations", "last_verified_version", "integer", "YES", "NULL"))
	results = append(results, checkColumn(ctx, conn, "llm_provider_configurations", "validation_details", "jsonb", "NO", "'{}'::jsonb"))
	results = append(results, checkColumn(ctx, conn, "llm_provider_configurations", "model_catalog_updated_at", "timestamp with time zone", "YES", "NULL"))
	results = append(results, checkColumn(ctx, conn, "llm_provider_models", "provider_id", "uuid", "NO", "NULL"))
	results = append(results, checkColumn(ctx, conn, "llm_provider_models", "model_key", "character varying", "NO", "NULL"))
	results = append(results, checkColumn(ctx, conn, "llm_provider_models", "source", "text", "NO", "NULL"))
	results = append(results, checkColumn(ctx, conn, "llm_provider_models", "availability", "text", "NO", "NULL"))
	results = append(results, checkColumn(ctx, conn, "workflow_connections", "last_verified_version", "integer", "YES", "NULL"))
	results = append(results, checkColumn(ctx, conn, "workflow_connections", "validation_details", "jsonb", "NO", "'{}'::jsonb"))
	results = append(results, checkColumn(ctx, conn, "workflow_configurations", "llm_strategy", "text", "NO", "'none'::text"))
	results = append(results, checkColumn(ctx, conn, "workflow_configurations", "llm_provider_id", "uuid", "YES", "NULL"))
	results = append(results, checkColumn(ctx, conn, "workflow_configurations", "llm_model", "character varying", "YES", "NULL"))
	results = append(results, checkColumn(ctx, conn, "workflow_configurations", "last_verified_version", "integer", "YES", "NULL"))
	results = append(results, checkColumn(ctx, conn, "workflow_configurations", "validation_details", "jsonb", "NO", "'{}'::jsonb"))
	results = append(results, checkColumn(ctx, conn, "workflow_run_records", "failure_phase", "text", "YES", "NULL"))
	results = append(results, checkColumn(ctx, conn, "workflow_run_records", "failure_code", "character varying", "YES", "NULL"))
	results = append(results, checkColumn(ctx, conn, "workflow_run_records", "safe_error_message", "character varying", "YES", "NULL"))
	results = append(results, checkColumn(ctx, conn, "workflow_run_records", "retryability", "text", "NO", "'not_retryable'::text"))
	results = append(results, checkColumn(ctx, conn, "workflow_run_records", "retry_mode", "text", "YES", "NULL"))
	results = append(results, checkColumn(ctx, conn, "workflow_run_records", "external_execution_id", "character varying", "YES", "NULL"))
	results = append(results, checkColumn(ctx, conn, "workflow_run_records", "cancellation_requested_at", "timestamp with time zone", "YES", "NULL"))
	results = append(results, checkColumn(ctx, conn, "workflow_run_records", "timed_out_at", "timestamp with time zone", "YES", "NULL"))
	results = append(results, checkColumn(ctx, conn, "workflow_run_records", "binding_snapshot", "jsonb", "NO", "'{}'::jsonb"))
	results = append(results, checkColumn(ctx, conn, "workflow_run_records", "connection_snapshot", "jsonb", "NO", "'{}'::jsonb"))
	results = append(results, checkColumn(ctx, conn, "workflow_run_records", "llm_policy_snapshot", "jsonb", "NO", "'{}'::jsonb"))
	for _, constraint := range []string{
		"llm_provider_models_provider_model_key",
		"llm_provider_configurations_verified_version_check",
		"workflow_connections_verified_version_check",
		"workflow_configurations_verified_version_check",
		"workflow_configurations_llm_strategy_shape_check",
		"workflow_run_records_retry_not_self_check",
		"workflow_run_records_snapshot_shape_check",
		"workflow_run_records_failure_shape_check",
	} {
		results = append(results, checkConstraintExists(ctx, conn, constraint))
	}
	for _, index := range []string{
		"llm_provider_models_provider_availability_model_idx",
		"workflow_configurations_stage_strategy_validation_enabled_idx",
		"workflow_run_records_project_stage_status_created_idx",
		"workflow_run_records_connection_execution_unique_idx",
	} {
		results = append(results, checkIndexExists(ctx, conn, index))
	}

	// --- 000021: workflow_run_result_consumption ---
	results = append(results, checkColumn(ctx, conn, "workflow_run_result_consumptions", "workflow_run_id", "uuid", "NO", "NULL"))
	results = append(results, checkColumn(ctx, conn, "workflow_run_result_consumptions", "status", "text", "NO", "NULL"))
	results = append(results, checkColumn(ctx, conn, "workflow_run_result_consumptions", "attempt_count", "integer", "NO", "0"))
	results = append(results, checkIndexExists(ctx, conn, "workflow_run_result_consumptions_recovery_idx"))
	results = append(results, checkIndexExists(ctx, conn, "workflow_run_events_one_succeeded_idx"))
	results = append(results, checkIndexExists(ctx, conn, "workflow_run_events_one_result_consumed_idx"))

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
	defaultMatch := defaultMatches(actualDefault, expectedDefault)

	pass := typeMatch && nullableMatch && defaultMatch
	actual := fmt.Sprintf("type=%s nullable=%s default=%s", dataType, nullable, actualDefault)
	expected := fmt.Sprintf("type=%s nullable=%s default=%s", expectedType, isNullable, expectedDefault)

	return checkResult{name: name, expected: expected, actual: actual, pass: pass}
}

func defaultMatches(actual, expected string) bool {
	if expected == "NULL" {
		return actual == "NULL" || actual == ""
	}
	actual = strings.TrimSpace(actual)
	expected = strings.TrimSpace(expected)
	if strings.EqualFold(actual, expected) {
		return true
	}
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

// === Data Check Framework ===

// writeSQLPattern matches SQL write keywords for detection.
var writeSQLPattern = regexp.MustCompile(`(?i)\b(INSERT|UPDATE|DELETE|TRUNCATE|DROP|CREATE|ALTER|GRANT|REVOKE|LOCK|VACUUM|REINDEX|CLUSTER|COPY|CALL|EXECUTE|DISCARD|LISTEN|NOTIFY|UNLISTEN|MOVE|FETCH|DECLARE|PREPARE|DEALLOCATE|REASSIGN|REFRESH|SECURITY|SET\s+(ROLE|SESSION AUTHORIZATION))\b`)

// isWriteSQL checks if a SQL string contains write keywords.
func isWriteSQL(sql string) bool {
	return writeSQLPattern.MatchString(sql)
}

// validateDataChecks validates all data check definitions.
func validateDataChecks(checks []dataCheck) error {
	seenIDs := map[string]bool{}
	for _, c := range checks {
		if c.ID == "" {
			return fmt.Errorf("data check has empty ID")
		}
		if c.Name == "" {
			return fmt.Errorf("data check %s has empty name", c.ID)
		}
		if seenIDs[c.ID] {
			return fmt.Errorf("duplicate data check ID: %s", c.ID)
		}
		seenIDs[c.ID] = true
		if isWriteSQL(c.SQL) {
			return fmt.Errorf("data check %s contains write SQL keywords", c.ID)
		}
	}
	return nil
}

// runDataChecks executes all data checks and returns results.
func runDataChecks(ctx context.Context, conn *pgx.Conn, checks []dataCheck) []dataCheckResult {
	results := make([]dataCheckResult, 0, len(checks))
	for _, c := range checks {
		results = append(results, executeDataCheck(ctx, conn, c))
	}
	return results
}

// formatValue formats a pgx value for display, handling special types like UUID.
func formatValue(v interface{}) string {
	switch val := v.(type) {
	case [16]uint8:
		// UUID as [16]byte — format as standard UUID string
		return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
			val[0:4], val[4:6], val[6:8], val[8:10], val[10:16])
	default:
		return fmt.Sprintf("%v", v)
	}
}

// querier abstracts over *pgx.Conn and pgx.Tx for read-only queries.
type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// executeDataCheck runs a single data check and returns its result.
// The SQL must return rows where the first column is the anomaly count,
// and the remaining columns are sample identification fields.
func executeDataCheck(ctx context.Context, q querier, c dataCheck) dataCheckResult {
	result := dataCheckResult{
		ID:          c.ID,
		Name:        c.Name,
		Description: c.Description,
		Pass:        true,
	}

	rows, err := q.Query(ctx, c.SQL)
	if err != nil {
		result.Pass = false
		result.AnomalyCount = -1
		result.Samples = []string{fmt.Sprintf("query error: %v", err)}
		return result
	}
	defer rows.Close()

	// Read anomaly count from first row (first column).
	if rows.Next() {
		vals, err := rows.Values()
		if err != nil {
			result.Pass = false
			result.AnomalyCount = -1
			result.Samples = []string{fmt.Sprintf("scan error: %v", err)}
			return result
		}
		if len(vals) == 0 {
			result.Pass = false
			result.AnomalyCount = -1
			result.Samples = []string{"no columns returned"}
			return result
		}

		// First column is the anomaly count
		switch v := vals[0].(type) {
		case int64:
			result.AnomalyCount = int(v)
		case int32:
			result.AnomalyCount = int(v)
		case int:
			result.AnomalyCount = v
		default:
			result.Pass = false
			result.AnomalyCount = -1
			result.Samples = []string{fmt.Sprintf("unexpected count type: %T", vals[0])}
			return result
		}

		if result.AnomalyCount > 0 {
			result.Pass = false
			// Use remaining columns from first row as first sample
			parts := make([]string, len(vals)-1)
			for i := 1; i < len(vals); i++ {
				parts[i-1] = formatValue(vals[i])
			}
			result.Samples = append(result.Samples, strings.Join(parts, " | "))
		}
	}

	// Read additional sample rows (up to 5 total)
	for rows.Next() && len(result.Samples) < 5 {
		vals, err := rows.Values()
		if err != nil {
			result.Samples = append(result.Samples, fmt.Sprintf("row error: %v", err))
			continue
		}
		// Skip first column (count) and use remaining columns
		parts := make([]string, len(vals)-1)
		for i := 1; i < len(vals); i++ {
			parts[i-1] = formatValue(vals[i])
		}
		result.Samples = append(result.Samples, strings.Join(parts, " | "))
	}

	return result
}

// sampleQuery builds a SQL fragment for sample row retrieval.
// For checks that use WITH ... SELECT count, we return sample rows.
// The SQL must return: count, then sample columns.

// === Data Check Definitions ===

func defineDataChecks() []dataCheck {
	checks := []dataCheck{
		// === FK Orphan Checks ===

		{
			ID:   "DC-ORPHAN-001",
			Name: "content_items current_version_id orphan",
			SQL: `WITH orphans AS (
				SELECT ci.id, ci.project_id, ci.current_version_id
				FROM content_items ci
				LEFT JOIN content_versions cv ON cv.id = ci.current_version_id
				WHERE ci.current_version_id IS NOT NULL AND cv.id IS NULL
			)
			SELECT (SELECT COUNT(*) FROM orphans) AS cnt,
			       id, project_id, current_version_id
			FROM orphans LIMIT 5`,
			Description: "content_items.current_version_id must reference an existing content_versions row",
		},
		{
			ID:   "DC-ORPHAN-002",
			Name: "review_reports content_version_id orphan",
			SQL: `WITH orphans AS (
				SELECT rr.id, rr.project_id, rr.content_version_id
				FROM review_reports rr
				LEFT JOIN content_versions cv ON cv.id = rr.content_version_id
				WHERE cv.id IS NULL
			)
			SELECT (SELECT COUNT(*) FROM orphans) AS cnt,
			       id, project_id, content_version_id
			FROM orphans LIMIT 5`,
			Description: "review_reports.content_version_id must reference an existing content_versions row",
		},
		{
			ID:   "DC-ORPHAN-003",
			Name: "review_findings review_id orphan",
			SQL: `WITH orphans AS (
				SELECT rf.id, rf.review_id
				FROM review_findings rf
				LEFT JOIN review_reports rr ON rr.id = rf.review_id
				WHERE rr.id IS NULL
			)
			SELECT (SELECT COUNT(*) FROM orphans) AS cnt,
			       id, review_id
			FROM orphans LIMIT 5`,
			Description: "review_findings.review_id must reference an existing review_reports row",
		},
		{
			ID:   "DC-ORPHAN-004",
			Name: "review_recommendations review_id orphan",
			SQL: `WITH orphans AS (
				SELECT rec.id, rec.review_id
				FROM review_recommendations rec
				LEFT JOIN review_reports rr ON rr.id = rec.review_id
				WHERE rr.id IS NULL
			)
			SELECT (SELECT COUNT(*) FROM orphans) AS cnt,
			       id, review_id
			FROM orphans LIMIT 5`,
			Description: "review_recommendations.review_id must reference an existing review_reports row",
		},
		{
			ID:   "DC-ORPHAN-005",
			Name: "chapter_plan_candidates batch_id orphan",
			SQL: `WITH orphans AS (
				SELECT cpc.id, cpc.batch_id, cpc.project_id
				FROM chapter_plan_candidates cpc
				LEFT JOIN chapter_plan_candidate_batches cpb ON cpb.id = cpc.batch_id
				WHERE cpb.id IS NULL
			)
			SELECT (SELECT COUNT(*) FROM orphans) AS cnt,
			       id, batch_id, project_id
			FROM orphans LIMIT 5`,
			Description: "chapter_plan_candidates.batch_id must reference an existing candidate_batches row",
		},
		{
			ID:   "DC-ORPHAN-006",
			Name: "chapter_plan_result_consumptions workflow_run_id orphan",
			SQL: `WITH orphans AS (
				SELECT cprc.workflow_run_id, cprc.project_id, cprc.status
				FROM chapter_plan_result_consumptions cprc
				LEFT JOIN workflow_run_records wrr ON wrr.id = cprc.workflow_run_id
				WHERE wrr.id IS NULL
			)
			SELECT (SELECT COUNT(*) FROM orphans) AS cnt,
			       workflow_run_id, project_id, status
			FROM orphans LIMIT 5`,
			Description: "chapter_plan_result_consumptions.workflow_run_id must reference an existing workflow_run_records row",
		},
		{
			ID:   "DC-ORPHAN-007",
			Name: "workflow_run_events run_id orphan",
			SQL: `WITH orphans AS (
				SELECT wre.id, wre.run_id
				FROM workflow_run_events wre
				LEFT JOIN workflow_run_records wrr ON wrr.id = wre.run_id
				WHERE wrr.id IS NULL
			)
			SELECT (SELECT COUNT(*) FROM orphans) AS cnt,
			       id, run_id
			FROM orphans LIMIT 5`,
			Description: "workflow_run_events.run_id must reference an existing workflow_run_records row",
		},
		{
			ID:   "DC-ORPHAN-008",
			Name: "workflow_run_records workflow_configuration_id orphan",
			SQL: `WITH orphans AS (
				SELECT wrr.id, wrr.project_id, wrr.workflow_configuration_id
				FROM workflow_run_records wrr
				LEFT JOIN workflow_configurations wc ON wc.id = wrr.workflow_configuration_id
				WHERE wc.id IS NULL
			)
			SELECT (SELECT COUNT(*) FROM orphans) AS cnt,
			       id, project_id, workflow_configuration_id
			FROM orphans LIMIT 5`,
			Description: "workflow_run_records.workflow_configuration_id must reference an existing workflow_configurations row",
		},
		{
			ID:   "DC-ORPHAN-009",
			Name: "content_versions content_item_id orphan",
			SQL: `WITH orphans AS (
				SELECT cv.id, cv.content_item_id
				FROM content_versions cv
				LEFT JOIN content_items ci ON ci.id = cv.content_item_id
				WHERE ci.id IS NULL
			)
			SELECT (SELECT COUNT(*) FROM orphans) AS cnt,
			       id, content_item_id
			FROM orphans LIMIT 5`,
			Description: "content_versions.content_item_id must reference an existing content_items row",
		},

		// === Cross-Project Checks ===

		{
			ID:   "DC-CROSS-001",
			Name: "content_items / chapter_plans project mismatch",
			SQL: `WITH mismatches AS (
				SELECT ci.id, ci.project_id AS ci_project, cp.project_id AS cp_project
				FROM content_items ci
				JOIN chapter_plans cp ON cp.id = ci.chapter_plan_id
				WHERE ci.project_id <> cp.project_id
			)
			SELECT (SELECT COUNT(*) FROM mismatches) AS cnt,
			       id, ci_project, cp_project
			FROM mismatches LIMIT 5`,
			Description: "content_items.project_id must match its chapter_plan.project_id",
		},
		{
			ID:   "DC-CROSS-002",
			Name: "review_reports / content_items project mismatch",
			SQL: `WITH mismatches AS (
				SELECT rr.id, rr.project_id AS rr_project, ci.project_id AS ci_project
				FROM review_reports rr
				JOIN content_items ci ON ci.id = rr.content_item_id
				WHERE rr.project_id <> ci.project_id
			)
			SELECT (SELECT COUNT(*) FROM mismatches) AS cnt,
			       id, rr_project, ci_project
			FROM mismatches LIMIT 5`,
			Description: "review_reports.project_id must match its content_item.project_id",
		},
		{
			ID:   "DC-CROSS-003",
			Name: "review_reports / content_versions content_item mismatch",
			SQL: `WITH mismatches AS (
				SELECT rr.id, rr.content_item_id, rr.content_version_id, cv.content_item_id AS cv_item
				FROM review_reports rr
				JOIN content_versions cv ON cv.id = rr.content_version_id
				WHERE cv.content_item_id <> rr.content_item_id
			)
			SELECT (SELECT COUNT(*) FROM mismatches) AS cnt,
			       id, content_item_id, content_version_id, cv_item
			FROM mismatches LIMIT 5`,
			Description: "review_reports.content_version must belong to the same content_item",
		},
		{
			ID:   "DC-CROSS-004",
			Name: "chapter_plan_candidates / batch project mismatch",
			SQL: `WITH mismatches AS (
				SELECT cpc.id, cpc.project_id AS cand_project, cpb.project_id AS batch_project
				FROM chapter_plan_candidates cpc
				JOIN chapter_plan_candidate_batches cpb ON cpb.id = cpc.batch_id
				WHERE cpc.project_id <> cpb.project_id
			)
			SELECT (SELECT COUNT(*) FROM mismatches) AS cnt,
			       id, cand_project, batch_project
			FROM mismatches LIMIT 5`,
			Description: "chapter_plan_candidates.project_id must match its batch.project_id",
		},
		{
			ID:   "DC-CROSS-005",
			Name: "chapter_plan_revisions / chapter_plan project mismatch",
			SQL: `WITH mismatches AS (
				SELECT cpr.id, cpr.project_id AS rev_project, cp.project_id AS cp_project
				FROM chapter_plan_revisions cpr
				JOIN chapter_plans cp ON cp.id = cpr.chapter_plan_id
				WHERE cpr.project_id <> cp.project_id
			)
			SELECT (SELECT COUNT(*) FROM mismatches) AS cnt,
			       id, rev_project, cp_project
			FROM mismatches LIMIT 5`,
			Description: "chapter_plan_revisions.project_id must match its chapter_plan.project_id",
		},
		{
			ID:   "DC-CROSS-006",
			Name: "chapter_plan_result_consumptions / workflow_run_records project mismatch",
			SQL: `WITH mismatches AS (
				SELECT cprc.workflow_run_id, cprc.project_id AS cons_project, wrr.project_id AS run_project
				FROM chapter_plan_result_consumptions cprc
				JOIN workflow_run_records wrr ON wrr.id = cprc.workflow_run_id
				WHERE cprc.project_id <> wrr.project_id
			)
			SELECT (SELECT COUNT(*) FROM mismatches) AS cnt,
			       workflow_run_id, cons_project, run_project
			FROM mismatches LIMIT 5`,
			Description: "chapter_plan_result_consumptions.project_id must match its workflow_run_record.project_id",
		},

		// === Current Version Checks ===

		{
			ID:   "DC-CURVER-001",
			Name: "content_items current_version_id missing",
			SQL: `WITH invalid AS (
				SELECT id, project_id, current_version_id
				FROM content_items
				WHERE current_version_id IS NULL
			)
			SELECT (SELECT COUNT(*) FROM invalid) AS cnt,
			       id, project_id, current_version_id
			FROM invalid LIMIT 5`,
			Description: "content_items.current_version_id must not be NULL",
		},
		{
			ID:   "DC-CURVER-002",
			Name: "content_items current_version wrong content_item",
			SQL: `WITH invalid AS (
				SELECT ci.id, ci.project_id, ci.current_version_id, cv.content_item_id AS cv_item
				FROM content_items ci
				JOIN content_versions cv ON cv.id = ci.current_version_id
				WHERE cv.content_item_id <> ci.id
			)
			SELECT (SELECT COUNT(*) FROM invalid) AS cnt,
			       id, project_id, current_version_id, cv_item
			FROM invalid LIMIT 5`,
			Description: "content_items.current_version_id must belong to the same content_item",
		},
		{
			ID:   "DC-CURVER-003",
			Name: "chapter_plans current_revision_id missing",
			SQL: `WITH invalid AS (
				SELECT id, project_id, current_revision_id
				FROM chapter_plans
				WHERE current_revision_id IS NULL
			)
			SELECT (SELECT COUNT(*) FROM invalid) AS cnt,
			       id, project_id, current_revision_id
			FROM invalid LIMIT 5`,
			Description: "chapter_plans.current_revision_id must not be NULL",
		},
		{
			ID:   "DC-CURVER-004",
			Name: "chapter_plans current_revision wrong chapter_plan",
			SQL: `WITH invalid AS (
				SELECT cp.id, cp.project_id, cp.current_revision_id, cpr.chapter_plan_id AS cpr_cp
				FROM chapter_plans cp
				JOIN chapter_plan_revisions cpr ON cpr.id = cp.current_revision_id
				WHERE cpr.chapter_plan_id <> cp.id
			)
			SELECT (SELECT COUNT(*) FROM invalid) AS cnt,
			       id, project_id, current_revision_id, cpr_cp
			FROM invalid LIMIT 5`,
			Description: "chapter_plans.current_revision_id must belong to the same chapter_plan",
		},

		// === Version Chain Checks ===

		{
			ID:   "DC-VERCHAIN-001",
			Name: "content_versions source_content_version_id orphan",
			SQL: `WITH orphans AS (
				SELECT cv.id, cv.content_item_id, cv.source_content_version_id
				FROM content_versions cv
				LEFT JOIN content_versions scv ON scv.id = cv.source_content_version_id
				WHERE cv.source_content_version_id IS NOT NULL AND scv.id IS NULL
			)
			SELECT (SELECT COUNT(*) FROM orphans) AS cnt,
			       id, content_item_id, source_content_version_id
			FROM orphans LIMIT 5`,
			Description: "content_versions.source_content_version_id must reference an existing content_versions row",
		},
		{
			ID:   "DC-VERCHAIN-002",
			Name: "content_versions source version cross-item",
			SQL: `WITH invalid AS (
				SELECT cv.id, cv.content_item_id, cv.source_content_version_id, scv.content_item_id AS src_item
				FROM content_versions cv
				JOIN content_versions scv ON scv.id = cv.source_content_version_id
				WHERE cv.content_item_id <> scv.content_item_id
			)
			SELECT (SELECT COUNT(*) FROM invalid) AS cnt,
			       id, content_item_id, source_content_version_id, src_item
			FROM invalid LIMIT 5`,
			Description: "content_versions.source_content_version_id must belong to the same content_item",
		},
		{
			ID:   "DC-VERCHAIN-003",
			Name: "content_versions source references self",
			SQL: `WITH self_ref AS (
				SELECT id, content_item_id, source_content_version_id
				FROM content_versions
				WHERE source_content_version_id IS NOT NULL
				  AND source_content_version_id = id
			)
			SELECT (SELECT COUNT(*) FROM self_ref) AS cnt,
			       id, content_item_id, source_content_version_id
			FROM self_ref LIMIT 5`,
			Description: "content_versions must not reference itself as source",
		},
		{
			ID:   "DC-VERCHAIN-004",
			Name: "storylines parent_id self-reference",
			SQL: `WITH self_ref AS (
				SELECT id, project_id, parent_id
				FROM storylines
				WHERE parent_id IS NOT NULL AND parent_id = id
			)
			SELECT (SELECT COUNT(*) FROM self_ref) AS cnt,
			       id, project_id, parent_id
			FROM self_ref LIMIT 5`,
			Description: "storylines.parent_id must not equal id",
		},
		{
			ID:   "DC-VERCHAIN-005",
			Name: "workflow_run_records retry_of_run_id self-reference",
			SQL: `WITH self_ref AS (
				SELECT id, project_id, retry_of_run_id
				FROM workflow_run_records
				WHERE retry_of_run_id IS NOT NULL AND retry_of_run_id = id
			)
			SELECT (SELECT COUNT(*) FROM self_ref) AS cnt,
			       id, project_id, retry_of_run_id
			FROM self_ref LIMIT 5`,
			Description: "workflow_run_records.retry_of_run_id must not equal id",
		},

		// === Status Consistency Checks ===

		{
			ID:   "DC-STATUS-001",
			Name: "workflow_run_records succeeded without finished_at",
			SQL: `WITH invalid AS (
				SELECT id, project_id, stage, status, finished_at
				FROM workflow_run_records
				WHERE status IN ('succeeded', 'failed', 'timed_out') AND finished_at IS NULL
			)
			SELECT (SELECT COUNT(*) FROM invalid) AS cnt,
			       id, project_id, stage, status
			FROM invalid LIMIT 5`,
			Description: "succeeded/failed/timed_out workflow_run_records must have finished_at",
		},
		{
			ID:   "DC-STATUS-002",
			Name: "workflow_run_records failed without error",
			SQL: `WITH invalid AS (
				SELECT id, project_id, stage, status, failure_code, safe_error_message, error_code, error_message
				FROM workflow_run_records
				WHERE status IN ('failed', 'timed_out')
				  AND (COALESCE(failure_code, error_code) IS NULL OR COALESCE(safe_error_message, error_message) IS NULL)
			)
			SELECT (SELECT COUNT(*) FROM invalid) AS cnt,
			       id, project_id, stage, status
			FROM invalid LIMIT 5`,
			Description: "failed/timed_out workflow_run_records must have a stable failure code and safe message",
		},
		{
			ID:   "DC-STATUS-003",
			Name: "workflow_run_records succeeded with error",
			SQL: `WITH invalid AS (
				SELECT id, project_id, stage, status, failure_code, error_code
				FROM workflow_run_records
				WHERE status NOT IN ('failed', 'timed_out')
				  AND (failure_code IS NOT NULL OR safe_error_message IS NOT NULL OR error_code IS NOT NULL OR error_message IS NOT NULL OR error_details IS NOT NULL)
			)
			SELECT (SELECT COUNT(*) FROM invalid) AS cnt,
			       id, project_id, stage, status, error_code
			FROM invalid LIMIT 5`,
			Description: "non-failed workflow_run_records must not have failure fields",
		},
		{
			ID:   "DC-STATUS-004",
			Name: "workflow_run_records queued with time fields",
			SQL: `WITH invalid AS (
				SELECT id, project_id, status, started_at, finished_at, cancelled_at
				FROM workflow_run_records
				WHERE status = 'queued'
				  AND (started_at IS NOT NULL OR finished_at IS NOT NULL OR cancelled_at IS NOT NULL)
			)
			SELECT (SELECT COUNT(*) FROM invalid) AS cnt,
			       id, project_id, status
			FROM invalid LIMIT 5`,
			Description: "queued workflow_run_records must have NULL started_at, finished_at, cancelled_at",
		},
		{
			ID:   "DC-STATUS-005",
			Name: "workflow_run_records running without started_at",
			SQL: `WITH invalid AS (
				SELECT id, project_id, status, started_at, finished_at
				FROM workflow_run_records
				WHERE status = 'running' AND (started_at IS NULL OR finished_at IS NOT NULL)
			)
			SELECT (SELECT COUNT(*) FROM invalid) AS cnt,
			       id, project_id, status
			FROM invalid LIMIT 5`,
			Description: "running workflow_run_records must have started_at and NULL finished_at",
		},
		{
			ID:   "DC-STATUS-006",
			Name: "content_versions frozen without frozen_at",
			SQL: `WITH invalid AS (
				SELECT id, content_item_id, status, frozen_at
				FROM content_versions
				WHERE status = 'frozen' AND frozen_at IS NULL
			)
			SELECT (SELECT COUNT(*) FROM invalid) AS cnt,
			       id, content_item_id, status
			FROM invalid LIMIT 5`,
			Description: "frozen content_versions must have frozen_at",
		},
		{
			ID:   "DC-STATUS-007",
			Name: "content_versions editable_draft with frozen_at",
			SQL: `WITH invalid AS (
				SELECT id, content_item_id, status, frozen_at
				FROM content_versions
				WHERE status = 'editable_draft' AND frozen_at IS NOT NULL
			)
			SELECT (SELECT COUNT(*) FROM invalid) AS cnt,
			       id, content_item_id, status
			FROM invalid LIMIT 5`,
			Description: "editable_draft content_versions must have NULL frozen_at",
		},
		{
			ID:   "DC-STATUS-008",
			Name: "chapter_plans confirmed without confirmed_at",
			SQL: `WITH invalid AS (
				SELECT id, project_id, status, confirmed_at
				FROM chapter_plans
				WHERE status = 'confirmed' AND confirmed_at IS NULL
			)
			SELECT (SELECT COUNT(*) FROM invalid) AS cnt,
			       id, project_id, status
			FROM invalid LIMIT 5`,
			Description: "confirmed chapter_plans must have confirmed_at",
		},
		{
			ID:   "DC-STATUS-009",
			Name: "chapter_plans pending_confirmation with confirmed_at",
			SQL: `WITH invalid AS (
				SELECT id, project_id, status, confirmed_at
				FROM chapter_plans
				WHERE status = 'pending_confirmation' AND confirmed_at IS NOT NULL
			)
			SELECT (SELECT COUNT(*) FROM invalid) AS cnt,
			       id, project_id, status
			FROM invalid LIMIT 5`,
			Description: "pending_confirmation chapter_plans must have NULL confirmed_at",
		},
		{
			ID:   "DC-STATUS-010",
			Name: "chapter_plan_candidates adopted without adopted_chapter_plan",
			SQL: `WITH invalid AS (
				SELECT id, project_id, batch_id, status, adopted_chapter_plan_id, adopted_at
				FROM chapter_plan_candidates
				WHERE status = 'adopted' AND (adopted_chapter_plan_id IS NULL OR adopted_at IS NULL)
			)
			SELECT (SELECT COUNT(*) FROM invalid) AS cnt,
			       id, project_id, batch_id, status
			FROM invalid LIMIT 5`,
			Description: "adopted chapter_plan_candidates must have adopted_chapter_plan_id and adopted_at",
		},
		{
			ID:   "DC-STATUS-011",
			Name: "chapter_plan_result_consumptions consumed without consumed_at",
			SQL: `WITH invalid AS (
				SELECT workflow_run_id, project_id, status, consumed_at
				FROM chapter_plan_result_consumptions
				WHERE status = 'consumed' AND consumed_at IS NULL
			)
			SELECT (SELECT COUNT(*) FROM invalid) AS cnt,
			       workflow_run_id, project_id, status
			FROM invalid LIMIT 5`,
			Description: "consumed chapter_plan_result_consumptions must have consumed_at",
		},

		// === Business Uniqueness Checks ===

		{
			ID:   "DC-UNIQUE-001",
			Name: "content_items duplicate chapter_plan",
			SQL: `WITH dups AS (
				SELECT chapter_plan_id, COUNT(*) AS cnt
				FROM content_items
				GROUP BY chapter_plan_id
				HAVING COUNT(*) > 1
			)
			SELECT (SELECT COUNT(*) FROM dups) AS cnt,
			       chapter_plan_id, cnt
			FROM dups LIMIT 5`,
			Description: "each chapter_plan must have at most one content_item",
		},
		{
			ID:   "DC-UNIQUE-002",
			Name: "chapter_plan_candidates adopted relation inconsistency",
			SQL: `WITH anomalies AS (
				SELECT cpc.id, cpc.project_id, cpc.adopted_chapter_plan_id, cpc.adopted_revision_id,
				       'adopted_revision_id_null' AS reason
				FROM chapter_plan_candidates cpc
				WHERE cpc.status = 'adopted' AND cpc.adopted_revision_id IS NULL

				UNION ALL

				SELECT cpc.id, cpc.project_id, cpc.adopted_chapter_plan_id, cpc.adopted_revision_id,
				       'adopted_revision_not_found' AS reason
				FROM chapter_plan_candidates cpc
				LEFT JOIN chapter_plan_revisions cpr ON cpr.id = cpc.adopted_revision_id
				WHERE cpc.status = 'adopted' AND cpc.adopted_revision_id IS NOT NULL AND cpr.id IS NULL

				UNION ALL

				SELECT cpc.id, cpc.project_id, cpc.adopted_chapter_plan_id, cpc.adopted_revision_id,
				       'revision_chapter_plan_mismatch' AS reason
				FROM chapter_plan_candidates cpc
				JOIN chapter_plan_revisions cpr ON cpr.id = cpc.adopted_revision_id
				WHERE cpc.status = 'adopted' AND cpr.chapter_plan_id <> cpc.adopted_chapter_plan_id

				UNION ALL

				SELECT cpc.id, cpc.project_id, cpc.adopted_chapter_plan_id, cpc.adopted_revision_id,
				       'revision_project_mismatch' AS reason
				FROM chapter_plan_candidates cpc
				JOIN chapter_plan_revisions cpr ON cpr.id = cpc.adopted_revision_id
				WHERE cpc.status = 'adopted' AND cpr.project_id <> cpc.project_id

				UNION ALL

				SELECT cpc.id, cpc.project_id, cpc.adopted_chapter_plan_id, cpc.adopted_revision_id,
				       'revision_source_candidate_mismatch' AS reason
				FROM chapter_plan_candidates cpc
				JOIN chapter_plan_revisions cpr ON cpr.id = cpc.adopted_revision_id
				WHERE cpc.status = 'adopted' AND (cpr.source_candidate_id IS NULL OR cpr.source_candidate_id <> cpc.id)

				UNION ALL

				SELECT cpc.id, cpc.project_id, cpc.adopted_chapter_plan_id, cpc.adopted_revision_id,
				       'revision_source_batch_mismatch' AS reason
				FROM chapter_plan_candidates cpc
				JOIN chapter_plan_revisions cpr ON cpr.id = cpc.adopted_revision_id
				WHERE cpc.status = 'adopted' AND (cpr.source_candidate_batch_id IS NULL OR cpr.source_candidate_batch_id <> cpc.batch_id)

				UNION ALL

				SELECT cpc.id, cpc.project_id, cpc.adopted_chapter_plan_id, cpc.adopted_revision_id,
				       'candidate_multiple_revisions' AS reason
				FROM chapter_plan_candidates cpc
				JOIN (
					SELECT source_candidate_id, COUNT(*) AS cnt
					FROM chapter_plan_revisions
					WHERE source_candidate_id IS NOT NULL
					GROUP BY source_candidate_id
					HAVING COUNT(*) > 1
				) multi ON multi.source_candidate_id = cpc.id
				WHERE cpc.status = 'adopted'
			)
			SELECT (SELECT COUNT(*) FROM anomalies) AS cnt,
			       id, project_id, adopted_chapter_plan_id, adopted_revision_id, reason
			FROM anomalies LIMIT 5`,
			Description: "adopted candidates must have a valid adopted_revision_id referencing a revision with matching chapter_plan_id, project_id, source_candidate_id, and source_candidate_batch_id",
		},
		{
			ID:   "DC-UNIQUE-003",
			Name: "review_findings duplicate issue_key per review",
			SQL: `WITH dups AS (
				SELECT review_id, issue_key, COUNT(*) AS cnt
				FROM review_findings
				WHERE issue_key IS NOT NULL
				GROUP BY review_id, issue_key
				HAVING COUNT(*) > 1
			)
			SELECT (SELECT COUNT(*) FROM dups) AS cnt,
			       review_id, issue_key, cnt
			FROM dups LIMIT 5`,
			Description: "each review must have unique issue_key values",
		},

		// === Time Consistency Checks ===

		{
			ID:   "DC-TIME-001",
			Name: "workflow_run_records finished_at before started_at",
			SQL: `WITH invalid AS (
				SELECT id, project_id, stage, status, started_at, finished_at
				FROM workflow_run_records
				WHERE started_at IS NOT NULL AND finished_at IS NOT NULL
				  AND finished_at < started_at
			)
			SELECT (SELECT COUNT(*) FROM invalid) AS cnt,
			       id, project_id, stage, status
			FROM invalid LIMIT 5`,
			Description: "workflow_run_records.finished_at must not be before started_at",
		},
		{
			ID:   "DC-TIME-002",
			Name: "workflow_run_records created_at after finished_at",
			SQL: `WITH invalid AS (
				SELECT id, project_id, stage, status, created_at, finished_at
				FROM workflow_run_records
				WHERE finished_at IS NOT NULL AND created_at > finished_at
			)
			SELECT (SELECT COUNT(*) FROM invalid) AS cnt,
			       id, project_id, stage, status
			FROM invalid LIMIT 5`,
			Description: "workflow_run_records.created_at must not be after finished_at",
		},
		{
			ID:   "DC-TIME-003",
			Name: "content_versions frozen_at before created_at",
			SQL: `WITH invalid AS (
				SELECT id, content_item_id, status, created_at, frozen_at
				FROM content_versions
				WHERE frozen_at IS NOT NULL AND frozen_at < created_at
			)
			SELECT (SELECT COUNT(*) FROM invalid) AS cnt,
			       id, content_item_id, status
			FROM invalid LIMIT 5`,
			Description: "content_versions.frozen_at must not be before created_at",
		},
		{
			ID:   "DC-TIME-004",
			Name: "review_reports completed_at before created_at",
			SQL: `WITH invalid AS (
				SELECT id, project_id, created_at, completed_at
				FROM review_reports
				WHERE completed_at IS NOT NULL AND completed_at < created_at
			)
			SELECT (SELECT COUNT(*) FROM invalid) AS cnt,
			       id, project_id
			FROM invalid LIMIT 5`,
			Description: "review_reports.completed_at must not be before created_at",
		},
		{
			ID:   "DC-TIME-005",
			Name: "chapter_plans confirmed_at before created_at",
			SQL: `WITH invalid AS (
				SELECT id, project_id, status, created_at, confirmed_at
				FROM chapter_plans
				WHERE confirmed_at IS NOT NULL AND confirmed_at < created_at
			)
			SELECT (SELECT COUNT(*) FROM invalid) AS cnt,
			       id, project_id, status
			FROM invalid LIMIT 5`,
			Description: "chapter_plans.confirmed_at must not be before created_at",
		},
		{
			ID:   "DC-TIME-006",
			Name: "chapter_plan_candidates adopted_at before created_at",
			SQL: `WITH invalid AS (
				SELECT id, project_id, batch_id, status, created_at, adopted_at
				FROM chapter_plan_candidates
				WHERE adopted_at IS NOT NULL AND adopted_at < created_at
			)
			SELECT (SELECT COUNT(*) FROM invalid) AS cnt,
			       id, project_id, batch_id, status
			FROM invalid LIMIT 5`,
			Description: "chapter_plan_candidates.adopted_at must not be before created_at",
		},
		{
			ID:   "DC-TIME-007",
			Name: "workflow_run_events created_at too old",
			SQL: `WITH invalid AS (
				SELECT wre.id, wre.run_id, wre.event_type, wre.created_at, wrr.created_at AS run_created
				FROM workflow_run_events wre
				JOIN workflow_run_records wrr ON wrr.id = wre.run_id
				WHERE wre.created_at < wrr.created_at
			)
			SELECT (SELECT COUNT(*) FROM invalid) AS cnt,
			       id, run_id, event_type
			FROM invalid LIMIT 5`,
			Description: "workflow_run_events.created_at must not be before its run's created_at",
		},

		// === Iteration 19 integration persistence checks ===

		{
			ID:   "DC-I19-001",
			Name: "workflow configuration LLM strategy shape",
			SQL: `WITH invalid AS (
				SELECT id, llm_strategy, llm_provider_id, llm_model
				FROM workflow_configurations
				WHERE llm_strategy NOT IN ('acf_managed', 'n8n_managed', 'none')
				   OR (llm_strategy = 'acf_managed' AND (llm_provider_id IS NULL OR llm_model IS NULL OR btrim(llm_model) = ''))
				   OR (llm_strategy IN ('n8n_managed', 'none') AND (llm_provider_id IS NOT NULL OR llm_model IS NOT NULL))
			)
			SELECT (SELECT COUNT(*) FROM invalid) AS cnt,
			       id, llm_strategy, llm_provider_id, llm_model
			FROM invalid LIMIT 5`,
			Description: "workflow configurations must satisfy the frozen LLM strategy field combinations",
		},
		{
			ID:   "DC-I19-002",
			Name: "verified configuration version mismatch",
			SQL: `WITH invalid AS (
				SELECT 'llm_provider' AS kind, id, integration_status, version, last_verified_version
				FROM llm_provider_configurations
				WHERE (integration_status = 'verified' AND last_verified_version IS DISTINCT FROM version)
				   OR (last_verified_version IS NOT NULL AND last_verified_version NOT BETWEEN 1 AND version)
				UNION ALL
				SELECT 'workflow_connection', id, integration_status, version, last_verified_version
				FROM workflow_connections
				WHERE (integration_status = 'verified' AND last_verified_version IS DISTINCT FROM version)
				   OR (last_verified_version IS NOT NULL AND last_verified_version NOT BETWEEN 1 AND version)
				UNION ALL
				SELECT 'workflow_configuration', id, integration_status, version, last_verified_version
				FROM workflow_configurations
				WHERE (integration_status = 'verified' AND last_verified_version IS DISTINCT FROM version)
				   OR (last_verified_version IS NOT NULL AND last_verified_version NOT BETWEEN 1 AND version)
			)
			SELECT (SELECT COUNT(*) FROM invalid) AS cnt,
			       kind, id, integration_status, version, last_verified_version
			FROM invalid LIMIT 5`,
			Description: "verified records must point at their current version and all verified versions must be in range",
		},
		{
			ID:   "DC-I19-003",
			Name: "LLM model catalogue provider orphan",
			SQL: `WITH invalid AS (
				SELECT model.id, model.provider_id, model.model_key
				FROM llm_provider_models model
				LEFT JOIN llm_provider_configurations provider ON provider.id = model.provider_id
				WHERE provider.id IS NULL
			)
			SELECT (SELECT COUNT(*) FROM invalid) AS cnt,
			       id, provider_id, model_key
			FROM invalid LIMIT 5`,
			Description: "every model catalogue entry must reference an existing LLM provider",
		},
		{
			ID:   "DC-I19-004",
			Name: "workflow run failure state shape",
			SQL: `WITH invalid AS (
				SELECT id, project_id, status, failure_phase, failure_code, retryability
				FROM workflow_run_records
				WHERE (status IN ('failed', 'timed_out') AND (COALESCE(failure_code, error_code) IS NULL OR COALESCE(safe_error_message, error_message) IS NULL))
				   OR (status NOT IN ('failed', 'timed_out') AND (failure_phase IS NOT NULL OR failure_code IS NOT NULL OR safe_error_message IS NOT NULL))
				   OR failure_phase IS NOT NULL AND failure_phase NOT IN ('external_execution', 'output_validation', 'result_consumption', 'cancellation')
				   OR retryability NOT IN ('runtime_retry', 'result_consumption_retry', 'not_retryable')
			)
			SELECT (SELECT COUNT(*) FROM invalid) AS cnt,
			       id, project_id, status, failure_phase, failure_code, retryability
			FROM invalid LIMIT 5`,
			Description: "workflow run failure and retryability fields must match the runtime state",
		},
		{
			ID:   "DC-I19-005",
			Name: "workflow run retry chain mismatch",
			SQL: `WITH invalid AS (
				SELECT retry.id, retry.project_id, retry.stage, retry.retry_of_run_id, original.project_id AS original_project, original.stage AS original_stage
				FROM workflow_run_records retry
				LEFT JOIN workflow_run_records original ON original.id = retry.retry_of_run_id
				WHERE retry.retry_of_run_id IS NOT NULL
				  AND (original.id IS NULL OR retry.id = retry.retry_of_run_id OR retry.project_id <> original.project_id OR retry.stage <> original.stage OR retry.retry_mode IS NULL)
			)
			SELECT (SELECT COUNT(*) FROM invalid) AS cnt,
			       id, project_id, stage, retry_of_run_id, original_project, original_stage
			FROM invalid LIMIT 5`,
			Description: "retry runs must reference a different run for the same project and stage and record their retry mode",
		},
		{
			ID:   "DC-I19-006",
			Name: "workflow run snapshot safety and shape",
			SQL: `WITH invalid AS (
				SELECT id, project_id, stage
				FROM workflow_run_records
				WHERE jsonb_typeof(binding_snapshot) <> 'object'
				   OR jsonb_typeof(connection_snapshot) <> 'object'
				   OR jsonb_typeof(llm_policy_snapshot) <> 'object'
				   OR (binding_snapshot::text || connection_snapshot::text || llm_policy_snapshot::text)
				      ~* ('"(password|secret|credential|authorization|cookie|api[_-]?key|access[_-]?token|ref' || 'resh[_-]?token)"[[:space:]]*:')
			)
			SELECT (SELECT COUNT(*) FROM invalid) AS cnt,
			       id, project_id, stage
			FROM invalid LIMIT 5`,
			Description: "workflow run snapshots must be JSON objects without secret-bearing values",
		},
		{
			ID:   "DC-I19-007",
			Name: "workflow run cancellation and timeout timestamps",
			SQL: `WITH invalid AS (
				SELECT id, project_id, status, cancellation_requested_at, cancelled_at, timed_out_at, finished_at
				FROM workflow_run_records
				WHERE (status = 'cancelling' AND cancellation_requested_at IS NULL)
				   OR (status = 'cancelled' AND cancelled_at IS NULL)
				   OR (status = 'timed_out' AND (timed_out_at IS NULL OR finished_at IS NULL))
				   OR (timed_out_at IS NOT NULL AND status <> 'timed_out')
			)
			SELECT (SELECT COUNT(*) FROM invalid) AS cnt,
			       id, project_id, status, cancellation_requested_at, cancelled_at, timed_out_at, finished_at
			FROM invalid LIMIT 5`,
			Description: "cancelling, cancelled, and timed_out runs must carry their matching timestamps",
		},
	}

	return checks
}
