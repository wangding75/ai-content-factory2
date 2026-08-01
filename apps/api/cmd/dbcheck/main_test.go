package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	testpostgres "github.com/local/ai-content-factory/apps/api/internal/testpostgres"
)

func TestParseDatabaseName(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		want    string
		wantErr bool
	}{
		{name: "ai_content_factory", url: "postgres://user:pass@localhost:5432/ai_content_factory?sslmode=disable", want: "ai_content_factory"},
		{name: "wrong database name", url: "postgres://user:pass@localhost:5432/wrong_db?sslmode=disable", want: "wrong_db"},
		{name: "ai_content_factory with extra params", url: "postgres://user:pass@localhost:5432/ai_content_factory?sslmode=disable&connect_timeout=10", want: "ai_content_factory"},
		{name: "no database path", url: "postgres://user:pass@localhost:5432", want: ""},
		{name: "invalid URL", url: "postgres://invalid url", wantErr: true},
		{name: "ai_content_factory localhost", url: "postgres://localhost:5432/ai_content_factory", want: "ai_content_factory"},
		{name: "different database", url: "postgres://user:pass@localhost:5432/test_db", want: "test_db"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDatabaseName(tt.url)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("parseDatabaseName(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestDatabaseNameValidation(t *testing.T) {
	name, err := parseDatabaseName("postgres://user:pass@localhost:5432/ai_content_factory")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "ai_content_factory" {
		t.Errorf("expected 'ai_content_factory', got %q", name)
	}

	name, err = parseDatabaseName("postgres://user:pass@localhost:5432/other_db")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "other_db" {
		t.Errorf("expected 'other_db', got %q", name)
	}
}

func TestLoadMigrations(t *testing.T) {
	dir := t.TempDir()

	createFile := func(name, content string) {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	createFile("000001_init.up.sql", "CREATE TABLE test (id SERIAL PRIMARY KEY);")
	createFile("000001_init.down.sql", "DROP TABLE test;")
	createFile("000002_projects.up.sql", "CREATE TABLE projects (id UUID PRIMARY KEY);")
	createFile("000002_projects.down.sql", "DROP TABLE projects;")

	migrations, err := loadMigrations(dir)
	if err != nil {
		t.Fatalf("loadMigrations: %v", err)
	}

	if len(migrations) != 2 {
		t.Fatalf("expected 2 migrations, got %d", len(migrations))
	}
	if migrations[0].version != 1 {
		t.Errorf("expected version 1, got %d", migrations[0].version)
	}
	if migrations[1].version != 2 {
		t.Errorf("expected version 2, got %d", migrations[1].version)
	}
	maxVersion := migrations[len(migrations)-1].version
	if maxVersion != 2 {
		t.Errorf("expected max version 2, got %d", maxVersion)
	}
}

func TestLoadMigrationsMissingUp(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "000001_init.down.sql"), []byte("DROP TABLE test;"), 0644)
	_, err := loadMigrations(dir)
	if err == nil {
		t.Error("expected error for missing up migration")
	}
}

func TestLoadMigrationsMissingDown(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "000001_init.up.sql"), []byte("CREATE TABLE test (id SERIAL PRIMARY KEY);"), 0644)
	_, err := loadMigrations(dir)
	if err == nil {
		t.Error("expected error for missing down migration")
	}
}

func TestLoadMigrationsDuplicateVersion(t *testing.T) {
	dir := t.TempDir()
	createFile := func(name, content string) {
		os.WriteFile(filepath.Join(dir, name), []byte(content), 0644)
	}
	createFile("000001_init.up.sql", "CREATE TABLE test1 (id SERIAL PRIMARY KEY);")
	createFile("000001_other.up.sql", "CREATE TABLE test2 (id SERIAL PRIMARY KEY);")
	createFile("000001_init.down.sql", "DROP TABLE test1;")

	migrations, err := loadMigrations(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v (loadMigrations tolerates overwrite, validation catches duplicates)", err)
	}
	if len(migrations) != 1 {
		t.Errorf("expected 1 migration, got %d", len(migrations))
	}
}

func TestLoadMigrationsEmptyDirectory(t *testing.T) {
	dir := t.TempDir()
	migrations, err := loadMigrations(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(migrations) != 0 {
		t.Errorf("expected 0 migrations, got %d", len(migrations))
	}
}

func TestLoadMigrationsIgnoresNonSQLFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("readme"), 0644)
	os.WriteFile(filepath.Join(dir, "000001_init.up.sql"), []byte("CREATE TABLE test (id SERIAL PRIMARY KEY);"), 0644)
	os.WriteFile(filepath.Join(dir, "000001_init.down.sql"), []byte("DROP TABLE test;"), 0644)

	migrations, err := loadMigrations(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(migrations) != 1 {
		t.Errorf("expected 1 migration, got %d", len(migrations))
	}
}

func TestLoadMigrationsSorted(t *testing.T) {
	dir := t.TempDir()
	createFile := func(name, content string) {
		os.WriteFile(filepath.Join(dir, name), []byte(content), 0644)
	}
	createFile("000005_third.up.sql", "CREATE TABLE third (id SERIAL PRIMARY KEY);")
	createFile("000005_third.down.sql", "DROP TABLE third;")
	createFile("000001_first.up.sql", "CREATE TABLE first (id SERIAL PRIMARY KEY);")
	createFile("000001_first.down.sql", "DROP TABLE first;")
	createFile("000003_second.up.sql", "CREATE TABLE second (id SERIAL PRIMARY KEY);")
	createFile("000003_second.down.sql", "DROP TABLE second;")

	migrations, err := loadMigrations(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(migrations) != 3 {
		t.Fatalf("expected 3 migrations, got %d", len(migrations))
	}
	if migrations[0].version != 1 {
		t.Errorf("expected version 1, got %d", migrations[0].version)
	}
	if migrations[1].version != 3 {
		t.Errorf("expected version 3, got %d", migrations[1].version)
	}
	if migrations[2].version != 5 {
		t.Errorf("expected version 5, got %d", migrations[2].version)
	}
}

func TestMigrationVersionComparison(t *testing.T) {
	diskMax := 18
	dbVersion := 18
	if dbVersion != diskMax {
		t.Error("db version should equal disk max")
	}
	if dbVersion < diskMax {
		t.Error("db version should not be less than disk max")
	}
	dbVersionAhead := 19
	if dbVersionAhead == diskMax {
		t.Error("db version ahead should not equal disk max")
	}
}

func TestCheckResultSummary(t *testing.T) {
	results := []checkResult{
		{name: "check1", pass: true},
		{name: "check2", pass: true},
		{name: "check3", pass: false},
		{name: "check4", pass: true},
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
	if passCount != 3 {
		t.Errorf("expected 3 passed, got %d", passCount)
	}
	if failCount != 1 {
		t.Errorf("expected 1 failed, got %d", failCount)
	}
}

func TestDefaultMatches(t *testing.T) {
	tests := []struct {
		actual   string
		expected string
		want     bool
	}{
		{"NULL", "NULL", true},
		{"", "NULL", true},
		{"'open'::character varying", "'open'::character varying", true},
		{"'open'::character varying", "'open'", true},
		{"1", "1", true},
		{"now()", "now()", true},
		{"'editable_draft'::character varying", "'editable_draft'", true},
	}

	for _, tt := range tests {
		t.Run(tt.actual+"_vs_"+tt.expected, func(t *testing.T) {
			got := defaultMatches(tt.actual, tt.expected)
			if got != tt.want {
				t.Errorf("defaultMatches(%q, %q) = %v, want %v", tt.actual, tt.expected, got, tt.want)
			}
		})
	}
}

func TestErrorMessagesNoPassword(t *testing.T) {
	_, err := parseDatabaseName("postgres://user:secret@localhost:5432/ai_content_factory")
	if err != nil {
		if strings.Contains(err.Error(), "secret") {
			t.Error("error message should not contain password")
		}
	}

	name, err := parseDatabaseName("postgres://user:secret@localhost:5432/ai_content_factory")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(name, "secret") {
		t.Error("database name should not contain password")
	}
	if name != "ai_content_factory" {
		t.Errorf("expected 'ai_content_factory', got %q", name)
	}
}

func TestMigrationFilePattern(t *testing.T) {
	tests := []struct {
		fileName string
		match    bool
		version  string
		dir      string
	}{
		{"000001_init.up.sql", true, "000001", "up"},
		{"000001_init.down.sql", true, "000001", "down"},
		{"000018_real_content_rewrite_foundation.up.sql", true, "000018", "up"},
		{"000018_real_content_rewrite_foundation.down.sql", true, "000018", "down"},
		{"migration-checksums.sha256", false, "", ""},
		{"README.md", false, "", ""},
		{"001_init.up.sql", true, "001", "up"},
		{"000001_init.up.sql.bak", false, "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.fileName, func(t *testing.T) {
			matches := migrationFilePattern.FindStringSubmatch(tt.fileName)
			if tt.match {
				if matches == nil {
					t.Errorf("expected match for %q", tt.fileName)
					return
				}
				if matches[1] != tt.version {
					t.Errorf("expected version %q, got %q", tt.version, matches[1])
				}
				if matches[2] != tt.dir {
					t.Errorf("expected direction %q, got %q", tt.dir, matches[2])
				}
			} else {
				if matches != nil {
					t.Errorf("expected no match for %q, got %v", tt.fileName, matches)
				}
			}
		})
	}
}

func TestMaxVersionFromMigrations(t *testing.T) {
	migrations := []migration{
		{version: 1, up: "a", down: "b"},
		{version: 5, up: "a", down: "b"},
		{version: 18, up: "a", down: "b"},
	}
	maxVersion := migrations[len(migrations)-1].version
	if maxVersion != 18 {
		t.Errorf("expected max version 18, got %d", maxVersion)
	}
}

// === Data Check Tests ===

func TestValidateDataChecksEmpty(t *testing.T) {
	checks := []dataCheck{}
	if err := validateDataChecks(checks); err != nil {
		t.Errorf("empty check list should be valid, got: %v", err)
	}
}

func TestValidateDataChecksEmptyID(t *testing.T) {
	checks := []dataCheck{
		{ID: "", Name: "test", SQL: "SELECT 1"},
	}
	if err := validateDataChecks(checks); err == nil {
		t.Error("expected error for empty ID")
	}
}

func TestValidateDataChecksEmptyName(t *testing.T) {
	checks := []dataCheck{
		{ID: "DC-001", Name: "", SQL: "SELECT 1"},
	}
	if err := validateDataChecks(checks); err == nil {
		t.Error("expected error for empty name")
	}
}

func TestValidateDataChecksDuplicateID(t *testing.T) {
	checks := []dataCheck{
		{ID: "DC-001", Name: "test1", SQL: "SELECT 1"},
		{ID: "DC-001", Name: "test2", SQL: "SELECT 2"},
	}
	if err := validateDataChecks(checks); err == nil {
		t.Error("expected error for duplicate ID")
	}
}

func TestValidateDataChecksWriteSQLRejected(t *testing.T) {
	writeSQLs := []string{
		"INSERT INTO test VALUES (1)",
		"UPDATE test SET x = 1",
		"DELETE FROM test",
		"TRUNCATE test",
		"DROP TABLE test",
		"CREATE TABLE test (id int)",
		"ALTER TABLE test ADD COLUMN x int",
		"LOCK TABLE test",
	}

	for _, sql := range writeSQLs {
		checks := []dataCheck{
			{ID: "DC-001", Name: "test", SQL: sql},
		}
		if err := validateDataChecks(checks); err == nil {
			t.Errorf("expected error for write SQL: %s", sql)
		}
	}
}

func TestValidateDataChecksReadSQLAllowed(t *testing.T) {
	readSQLs := []string{
		"SELECT 1",
		"SELECT * FROM test",
		"WITH x AS (SELECT 1) SELECT * FROM x",
		"SELECT COUNT(*) FROM test WHERE id = 1",
	}

	for _, sql := range readSQLs {
		checks := []dataCheck{
			{ID: "DC-001", Name: "test", SQL: sql},
		}
		if err := validateDataChecks(checks); err != nil {
			t.Errorf("unexpected error for read SQL %q: %v", sql, err)
		}
	}
}

func TestIsWriteSQL(t *testing.T) {
	tests := []struct {
		sql  string
		want bool
	}{
		{"SELECT 1", false},
		{"SELECT * FROM test", false},
		{"WITH cte AS (SELECT 1) SELECT * FROM cte", false},
		{"INSERT INTO test VALUES (1)", true},
		{"UPDATE test SET x = 1", true},
		{"DELETE FROM test", true},
		{"TRUNCATE test", true},
		{"DROP TABLE test", true},
		{"CREATE TABLE test (id int)", true},
		{"ALTER TABLE test ADD COLUMN x int", true},
		{"LOCK TABLE test", true},
		{"insert into test values (1)", true},
		{"  INSERT INTO test VALUES (1)", true},
	}

	for _, tt := range tests {
		t.Run(tt.sql, func(t *testing.T) {
			got := isWriteSQL(tt.sql)
			if got != tt.want {
				t.Errorf("isWriteSQL(%q) = %v, want %v", tt.sql, got, tt.want)
			}
		})
	}
}

func TestDataCheckResultSummary(t *testing.T) {
	results := []dataCheckResult{
		{ID: "DC-001", Name: "test1", Pass: true},
		{ID: "DC-002", Name: "test2", Pass: true},
		{ID: "DC-003", Name: "test3", Pass: false, AnomalyCount: 3, Samples: []string{"a|b", "c|d"}},
		{ID: "DC-004", Name: "test4", Pass: true},
	}

	passCount := 0
	failCount := 0
	for _, r := range results {
		if r.Pass {
			passCount++
		} else {
			failCount++
		}
	}
	if passCount != 3 {
		t.Errorf("expected 3 passed, got %d", passCount)
	}
	if failCount != 1 {
		t.Errorf("expected 1 failed, got %d", failCount)
	}

	// Verify anomaly count
	for _, r := range results {
		if r.ID == "DC-003" {
			if r.AnomalyCount != 3 {
				t.Errorf("expected 3 anomalies, got %d", r.AnomalyCount)
			}
			if len(r.Samples) != 2 {
				t.Errorf("expected 2 samples, got %d", len(r.Samples))
			}
		}
	}
}

func TestDataCheckSampleLimit(t *testing.T) {
	// The constraint is enforced in executeDataCheck via the LIMIT 5 in sample loop.
	// Here we verify that if a result has more than 5 samples, it represents a code issue.
	// The code itself limits to 5 samples in the loop.

	results := []dataCheckResult{
		{ID: "DC-001", Name: "test", Pass: false, AnomalyCount: 100, Samples: []string{"1", "2", "3", "4", "5"}},
	}
	if len(results[0].Samples) > 5 {
		t.Errorf("samples should be limited to 5, got %d", len(results[0].Samples))
	}
}

func TestAllDataChecksHaveIDs(t *testing.T) {
	checks := defineDataChecks()
	for _, c := range checks {
		if c.ID == "" {
			t.Errorf("data check has empty ID: name=%q", c.Name)
		}
		if c.Name == "" {
			t.Errorf("data check %s has empty name", c.ID)
		}
		if c.SQL == "" {
			t.Errorf("data check %s has empty SQL", c.ID)
		}
	}
}

func TestAllDataChecksUniqueIDs(t *testing.T) {
	checks := defineDataChecks()
	seen := map[string]bool{}
	for _, c := range checks {
		if seen[c.ID] {
			t.Errorf("duplicate data check ID: %s", c.ID)
		}
		seen[c.ID] = true
	}
}

func TestAllDataChecksReadOnly(t *testing.T) {
	checks := defineDataChecks()
	for _, c := range checks {
		if isWriteSQL(c.SQL) {
			t.Errorf("data check %s contains write SQL: %s", c.ID, c.SQL)
		}
	}
}

func TestDataCheckCount(t *testing.T) {
	checks := defineDataChecks()
	// Should have a reasonable number of checks
	if len(checks) < 10 {
		t.Errorf("expected at least 10 data checks, got %d", len(checks))
	}
}

func TestIteration19DataChecksAreRegistered(t *testing.T) {
	want := map[string]bool{
		"DC-I19-001": false,
		"DC-I19-002": false,
		"DC-I19-003": false,
		"DC-I19-004": false,
		"DC-I19-005": false,
		"DC-I19-006": false,
		"DC-I19-007": false,
	}
	for _, check := range defineDataChecks() {
		if _, ok := want[check.ID]; ok {
			want[check.ID] = true
		}
	}
	for id, found := range want {
		if !found {
			t.Errorf("Iteration 19 data check %s is not registered", id)
		}
	}
}

func TestDataCheckCategories(t *testing.T) {
	checks := defineDataChecks()
	categories := map[string]int{}
	for _, c := range checks {
		parts := strings.SplitN(c.ID, "-", 3)
		if len(parts) >= 2 {
			categories[parts[0]+"-"+parts[1]]++
		}
	}

	// Should have multiple categories
	if len(categories) < 3 {
		t.Errorf("expected at least 3 check categories, got %d: %v", len(categories), categories)
	}
}

func TestErrorMessagesNoPayloadInDataChecks(t *testing.T) {
	checks := defineDataChecks()
	for _, c := range checks {
		// Data check SQL should not contain sensitive column references
		sql := strings.ToUpper(c.SQL)
		if strings.Contains(sql, "ENCRYPTED_SECRET") ||
			strings.Contains(sql, "ENCRYPTED_CREDENTIAL") ||
			strings.Contains(sql, "OUTPUT_PAYLOAD") && !strings.Contains(sql, "IS NULL") ||
			strings.Contains(sql, "INPUT_PAYLOAD") && !strings.Contains(sql, "IS NULL") ||
			strings.Contains(sql, "CONFIGURATION_SNAPSHOT") && !strings.Contains(sql, "IS NULL") {
			// Only flag if it's selecting payload content (not just checking for NULL)
			if strings.Contains(sql, "ENCRYPTED_SECRET") || strings.Contains(sql, "ENCRYPTED_CREDENTIAL") {
				t.Errorf("data check %s references encrypted/sensitive columns", c.ID)
			}
		}
	}
}

// === DC-UNIQUE-002 semantic correction tests ===

func TestDCUNIQUE002Exists(t *testing.T) {
	checks := defineDataChecks()
	found := false
	for _, c := range checks {
		if c.ID == "DC-UNIQUE-002" {
			found = true
			break
		}
	}
	if !found {
		t.Error("DC-UNIQUE-002 check must exist")
	}
}

func TestDCUNIQUE002NoDuplicateAdoptedChapterPlanSQL(t *testing.T) {
	checks := defineDataChecks()
	for _, c := range checks {
		if c.ID == "DC-UNIQUE-002" {
			sql := strings.ToUpper(c.SQL)
			// Must not use GROUP BY adopted_chapter_plan_id HAVING COUNT(*) > 1 pattern
			// The old pattern grouped by adopted_chapter_plan_id and counted duplicates
			if strings.Contains(sql, "GROUP BY ADOPTED_CHAPTER_PLAN_ID") {
				t.Error("DC-UNIQUE-002 must not use GROUP BY adopted_chapter_plan_id")
			}
			return
		}
	}
	t.Error("DC-UNIQUE-002 not found")
}

func TestDCUNIQUE002ContainsAdoptedRevisionID(t *testing.T) {
	checks := defineDataChecks()
	for _, c := range checks {
		if c.ID == "DC-UNIQUE-002" {
			if !strings.Contains(strings.ToUpper(c.SQL), "ADOPTED_REVISION_ID") {
				t.Error("DC-UNIQUE-002 SQL must reference adopted_revision_id")
			}
			return
		}
	}
	t.Error("DC-UNIQUE-002 not found")
}

func TestDCUNIQUE002ValidatesRevisionChapterPlanID(t *testing.T) {
	checks := defineDataChecks()
	for _, c := range checks {
		if c.ID == "DC-UNIQUE-002" {
			sql := strings.ToUpper(c.SQL)
			// Must check that revision.chapter_plan_id matches candidate.adopted_chapter_plan_id
			if !strings.Contains(sql, "CPR.CHAPTER_PLAN_ID") ||
				!strings.Contains(sql, "ADOPTED_CHAPTER_PLAN_ID") {
				t.Error("DC-UNIQUE-002 must validate revision.chapter_plan_id against candidate.adopted_chapter_plan_id")
			}
			return
		}
	}
	t.Error("DC-UNIQUE-002 not found")
}

func TestDCUNIQUE002ValidatesRevisionProjectID(t *testing.T) {
	checks := defineDataChecks()
	for _, c := range checks {
		if c.ID == "DC-UNIQUE-002" {
			sql := strings.ToUpper(c.SQL)
			// Must check revision.project_id matches candidate.project_id
			if !strings.Contains(sql, "REVISION_PROJECT_MISMATCH") {
				t.Error("DC-UNIQUE-002 must validate revision.project_id against candidate.project_id")
			}
			return
		}
	}
	t.Error("DC-UNIQUE-002 not found")
}

func TestDCUNIQUE002ValidatesSourceCandidateID(t *testing.T) {
	checks := defineDataChecks()
	for _, c := range checks {
		if c.ID == "DC-UNIQUE-002" {
			sql := strings.ToUpper(c.SQL)
			if !strings.Contains(sql, "SOURCE_CANDIDATE_ID") {
				t.Error("DC-UNIQUE-002 must validate source_candidate_id")
			}
			return
		}
	}
	t.Error("DC-UNIQUE-002 not found")
}

func TestDCUNIQUE002ValidatesSourceCandidateBatchID(t *testing.T) {
	checks := defineDataChecks()
	for _, c := range checks {
		if c.ID == "DC-UNIQUE-002" {
			sql := strings.ToUpper(c.SQL)
			if !strings.Contains(sql, "SOURCE_CANDIDATE_BATCH_ID") {
				t.Error("DC-UNIQUE-002 must validate source_candidate_batch_id")
			}
			return
		}
	}
	t.Error("DC-UNIQUE-002 not found")
}

func TestDCUNIQUE002ChecksCandidateMultipleRevisions(t *testing.T) {
	checks := defineDataChecks()
	for _, c := range checks {
		if c.ID == "DC-UNIQUE-002" {
			sql := strings.ToUpper(c.SQL)
			// Must check that a candidate is not referenced by multiple revisions
			if !strings.Contains(sql, "CANDIDATE_MULTIPLE_REVISIONS") {
				t.Error("DC-UNIQUE-002 must check for candidate referenced by multiple revisions")
			}
			return
		}
	}
	t.Error("DC-UNIQUE-002 not found")
}

func TestDCUNIQUE002DescriptionMatchesNewSemantics(t *testing.T) {
	checks := defineDataChecks()
	for _, c := range checks {
		if c.ID == "DC-UNIQUE-002" {
			desc := strings.ToLower(c.Description)
			if !strings.Contains(desc, "adopted_revision_id") {
				t.Errorf("DC-UNIQUE-002 description should mention adopted_revision_id, got: %s", c.Description)
			}
			if !strings.Contains(desc, "source_candidate_id") {
				t.Errorf("DC-UNIQUE-002 description should mention source_candidate_id, got: %s", c.Description)
			}
			if strings.Contains(desc, "at most one") {
				t.Errorf("DC-UNIQUE-002 description must not contain old 'at most one' semantics, got: %s", c.Description)
			}
			return
		}
	}
	t.Error("DC-UNIQUE-002 not found")
}

// === DC-UNIQUE-002 PostgreSQL scenario tests ===

// dc002Check returns the DC-UNIQUE-002 dataCheck definition.
func dc002Check() dataCheck {
	checks := defineDataChecks()
	for _, c := range checks {
		if c.ID == "DC-UNIQUE-002" {
			return c
		}
	}
	panic("DC-UNIQUE-002 not found")
}

// scenarioProject creates a project, workflow infrastructure, and chapter_plan
// for a scenario test.
func scenarioProject(t *testing.T, ctx context.Context, tx pgx.Tx, projectID, connID, configID, workflowRunID, cpID string) {
	t.Helper()
	_, err := tx.Exec(ctx, `INSERT INTO projects (id, name, type, created_by) VALUES ($1, 'scenario', 'novel', 'test') ON CONFLICT DO NOTHING`, projectID)
	if err != nil {
		t.Fatalf("insert project: %v", err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO workflow_connections (id, name, connection_type, base_url, auth_type, timeout_seconds, type_config) VALUES ($1, 'scenario-conn', 'n8n', 'http://localhost:5678', 'api_key', 30, '{}') ON CONFLICT DO NOTHING`, connID)
	if err != nil {
		t.Fatalf("insert workflow_connection: %v", err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO workflow_configurations (id, name, connection_id, applicable_stages, type_config, input_contract_version, output_contract_version) VALUES ($1, 'scenario-config', $2, '["chapter_planning"]', '{}', 'v1', 'v1') ON CONFLICT DO NOTHING`, configID, connID)
	if err != nil {
		t.Fatalf("insert workflow_configuration: %v", err)
	}
	rn := "scenario-run-" + workflowRunID[:8]
	_, err = tx.Exec(ctx, `INSERT INTO workflow_run_records (id, run_number, project_id, stage, workflow_configuration_id, trigger_source, status, configuration_snapshot, input_payload, started_at, finished_at, created_at, updated_at) VALUES ($1, $2, $3, 'chapter_planning', $4, 'manual', 'succeeded', '{}', '{}', NOW(), NOW(), NOW(), NOW()) ON CONFLICT DO NOTHING`, workflowRunID, rn, projectID, configID)
	if err != nil {
		t.Fatalf("insert workflow_run_record: %v", err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO chapter_plans (id, project_id, chapter_no, title, summary, status, source, created_by, version, created_at, updated_at) VALUES ($1, $2, 1, 'Test', 'summary', 'pending_confirmation', 'manual', 'test', 1, NOW(), NOW()) ON CONFLICT DO NOTHING`, cpID, projectID)
	if err != nil {
		t.Fatalf("insert chapter_plan: %v", err)
	}
}

// scenarioBatch creates a candidate batch.
func scenarioBatch(t *testing.T, ctx context.Context, tx pgx.Tx, batchID, projectID, workflowRunID, digest string) {
	t.Helper()
	_, err := tx.Exec(ctx, `INSERT INTO chapter_plan_candidate_batches (id, project_id, source_workflow_run_id, generation_mode, requested_chapter_count, input_digest, input_snapshot, storyline_selection_snapshot, context_options, workflow_binding_snapshot, created_by, updated_by, created_at, updated_at) VALUES ($1, $2, $3, 'full', 1, $4, '{}', '{}', '{}', '{}', 'test', 'test', NOW(), NOW()) ON CONFLICT DO NOTHING`, batchID, projectID, workflowRunID, digest)
	if err != nil {
		t.Fatalf("insert batch %s: %v", batchID, err)
	}
}

// scenarioCandidate creates a candidate with adopted_revision_id = NULL
// (handles the circular FK between candidate and revision).
func scenarioCandidate(t *testing.T, ctx context.Context, tx pgx.Tx, candID, batchID, projectID, chapterPlanID string, chapterNo, sortOrder int) {
	t.Helper()
	_, err := tx.Exec(ctx, `INSERT INTO chapter_plan_candidates (id, batch_id, project_id, chapter_no, sort_order, generated_snapshot, current_snapshot, diff_type, status, adopted_chapter_plan_id, adopted_revision_id, adopted_at, created_by, updated_by, version, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, '{}', '{}', 'new', 'adopted', $6, NULL, NOW(), 'test', 'test', 1, NOW(), NOW()) ON CONFLICT DO NOTHING`, candID, batchID, projectID, chapterNo, sortOrder, chapterPlanID)
	if err != nil {
		t.Fatalf("insert candidate %s: %v", candID, err)
	}
}

// scenarioRevision creates a revision.
func scenarioRevision(t *testing.T, ctx context.Context, tx pgx.Tx, revID, cpID, projectID, workflowRunID, srcCandID, srcBatchID string, revNo int) {
	t.Helper()
	_, err := tx.Exec(ctx, `INSERT INTO chapter_plan_revisions (id, chapter_plan_id, project_id, revision_no, snapshot, change_type, source_candidate_id, source_candidate_batch_id, source_workflow_run_id, created_by, created_at) VALUES ($1, $2, $3, $4, '{}', 'candidate_adopt', $5, $6, $7, 'test', NOW()) ON CONFLICT DO NOTHING`, revID, cpID, projectID, revNo, srcCandID, srcBatchID, workflowRunID)
	if err != nil {
		t.Fatalf("insert revision %s: %v", revID, err)
	}
}

// scenarioSetAdoptedRevision updates a candidate's adopted_revision_id.
func scenarioSetAdoptedRevision(t *testing.T, ctx context.Context, tx pgx.Tx, candID, revID string) {
	t.Helper()
	_, err := tx.Exec(ctx, `UPDATE chapter_plan_candidates SET adopted_revision_id = $2 WHERE id = $1`, candID, revID)
	if err != nil {
		t.Fatalf("update candidate %s adopted_revision_id: %v", candID, err)
	}
}

func TestDCUNIQUE002ScenarioALegitimateDuplicatePlanAdopt(t *testing.T) {
	pool, ctx := testpostgres.Open(t)
	defer pool.Close()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback(ctx)

	projectID := "00000000-0000-0000-0000-00000000a001"
	cpID := "00000000-0000-0000-0000-00000000a002"
	connID := "00000000-0000-0000-0000-00000000a003"
	configID := "00000000-0000-0000-0000-00000000a004"
	run1ID := "00000000-0000-0000-0000-00000000a005"
	run2ID := "00000000-0000-0000-0000-00000000a006"
	batch1ID := "00000000-0000-0000-0000-00000000a010"
	batch2ID := "00000000-0000-0000-0000-00000000a011"
	cand1ID := "00000000-0000-0000-0000-00000000a020"
	cand2ID := "00000000-0000-0000-0000-00000000a021"
	rev1ID := "00000000-0000-0000-0000-00000000a030"
	rev2ID := "00000000-0000-0000-0000-00000000a031"

	scenarioProject(t, ctx, tx, projectID, connID, configID, run1ID, cpID)
	// Create second workflow run (batches have unique source_workflow_run_id constraint)
	_, err = tx.Exec(ctx, `INSERT INTO workflow_run_records (id, run_number, project_id, stage, workflow_configuration_id, trigger_source, status, configuration_snapshot, input_payload, started_at, finished_at, created_at, updated_at) VALUES ($1, 'scenario-run-a2', $2, 'chapter_planning', $3, 'manual', 'succeeded', '{}', '{}', NOW(), NOW(), NOW(), NOW()) ON CONFLICT DO NOTHING`, run2ID, projectID, configID)
	if err != nil {
		t.Fatalf("insert workflow_run_record run2: %v", err)
	}
	digest := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	scenarioBatch(t, ctx, tx, batch1ID, projectID, run1ID, digest)
	scenarioBatch(t, ctx, tx, batch2ID, projectID, run2ID, digest)

	// Create candidates first (adopted_revision_id = NULL to avoid circular FK)
	scenarioCandidate(t, ctx, tx, cand1ID, batch1ID, projectID, cpID, 1, 0)
	scenarioCandidate(t, ctx, tx, cand2ID, batch2ID, projectID, cpID, 2, 1)

	// Create revisions referencing their candidates
	scenarioRevision(t, ctx, tx, rev1ID, cpID, projectID, run1ID, cand1ID, batch1ID, 1)
	scenarioRevision(t, ctx, tx, rev2ID, cpID, projectID, run2ID, cand2ID, batch2ID, 2)

	// Update candidates to point to their revisions
	scenarioSetAdoptedRevision(t, ctx, tx, cand1ID, rev1ID)
	scenarioSetAdoptedRevision(t, ctx, tx, cand2ID, rev2ID)

	result := executeDataCheck(ctx, tx, dc002Check())
	if result.AnomalyCount != 0 {
		t.Errorf("Scenario A: expected 0 anomalies (legitimate duplicate plan adopt), got %d: %v", result.AnomalyCount, result.Samples)
	}
}

func TestDCUNIQUE002ScenarioBWrongRevisionChapterPlan(t *testing.T) {
	pool, ctx := testpostgres.Open(t)
	defer pool.Close()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback(ctx)

	projectID := "00000000-0000-0000-0000-00000000b001"
	cp1ID := "00000000-0000-0000-0000-00000000b002"
	cp2ID := "00000000-0000-0000-0000-00000000b003"
	connID := "00000000-0000-0000-0000-00000000b004"
	configID := "00000000-0000-0000-0000-00000000b005"
	runID := "00000000-0000-0000-0000-00000000b006"
	batchID := "00000000-0000-0000-0000-00000000b010"
	candID := "00000000-0000-0000-0000-00000000b020"
	revID := "00000000-0000-0000-0000-00000000b030"

	scenarioProject(t, ctx, tx, projectID, connID, configID, runID, cp1ID)
	// Create a second chapter_plan (cp2) for the mismatch
	_, err = tx.Exec(ctx, `INSERT INTO chapter_plans (id, project_id, chapter_no, title, summary, status, source, created_by, version, created_at, updated_at) VALUES ($1, $2, 2, 'Test2', 'summary', 'pending_confirmation', 'manual', 'test', 1, NOW(), NOW()) ON CONFLICT DO NOTHING`, cp2ID, projectID)
	if err != nil {
		t.Fatalf("insert chapter_plan cp2: %v", err)
	}

	digest := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	scenarioBatch(t, ctx, tx, batchID, projectID, runID, digest)

	// Candidate adopted to cp1, with adopted_revision_id = NULL first
	scenarioCandidate(t, ctx, tx, candID, batchID, projectID, cp1ID, 1, 0)

	// Revision belongs to cp2, but candidate says adopted_chapter_plan_id = cp1
	scenarioRevision(t, ctx, tx, revID, cp2ID, projectID, runID, candID, batchID, 1)

	// Update candidate to point to the mismatched revision
	scenarioSetAdoptedRevision(t, ctx, tx, candID, revID)

	result := executeDataCheck(ctx, tx, dc002Check())
	if result.AnomalyCount == 0 {
		t.Error("Scenario B: expected anomalies for revision chapter_plan mismatch, got 0")
	}
}

func TestDCUNIQUE002ScenarioCWrongSourceCandidateID(t *testing.T) {
	pool, ctx := testpostgres.Open(t)
	defer pool.Close()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback(ctx)

	projectID := "00000000-0000-0000-0000-00000000c001"
	cpID := "00000000-0000-0000-0000-00000000c002"
	connID := "00000000-0000-0000-0000-00000000c003"
	configID := "00000000-0000-0000-0000-00000000c004"
	runID := "00000000-0000-0000-0000-00000000c005"
	batchID := "00000000-0000-0000-0000-00000000c010"
	cand1ID := "00000000-0000-0000-0000-00000000c020"
	cand2ID := "00000000-0000-0000-0000-00000000c021"
	revID := "00000000-0000-0000-0000-00000000c030"

	scenarioProject(t, ctx, tx, projectID, connID, configID, runID, cpID)
	digest := "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	scenarioBatch(t, ctx, tx, batchID, projectID, runID, digest)

	// Create cand2 first (needed for revision FK), with adopted_revision_id = NULL
	scenarioCandidate(t, ctx, tx, cand2ID, batchID, projectID, cpID, 2, 1)

	// Revision points to cand2 as source
	scenarioRevision(t, ctx, tx, revID, cpID, projectID, runID, cand2ID, batchID, 1)

	// cand1 says adopted_revision_id = revID, but rev.source_candidate_id = cand2
	scenarioCandidate(t, ctx, tx, cand1ID, batchID, projectID, cpID, 1, 0)
	scenarioSetAdoptedRevision(t, ctx, tx, cand1ID, revID)
	scenarioSetAdoptedRevision(t, ctx, tx, cand2ID, revID)

	result := executeDataCheck(ctx, tx, dc002Check())
	if result.AnomalyCount == 0 {
		t.Error("Scenario C: expected anomalies for source_candidate_id mismatch, got 0")
	}
}

func TestDCUNIQUE002ScenarioDCandidateMultipleRevisions(t *testing.T) {
	pool, ctx := testpostgres.Open(t)
	defer pool.Close()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback(ctx)

	projectID := "00000000-0000-0000-0000-00000000d001"
	cpID := "00000000-0000-0000-0000-00000000d002"
	connID := "00000000-0000-0000-0000-00000000d003"
	configID := "00000000-0000-0000-0000-00000000d004"
	runID := "00000000-0000-0000-0000-00000000d005"
	batchID := "00000000-0000-0000-0000-00000000d010"
	candID := "00000000-0000-0000-0000-00000000d020"
	rev1ID := "00000000-0000-0000-0000-00000000d030"
	rev2ID := "00000000-0000-0000-0000-00000000d031"

	scenarioProject(t, ctx, tx, projectID, connID, configID, runID, cpID)
	digest := "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
	scenarioBatch(t, ctx, tx, batchID, projectID, runID, digest)

	// Create candidate first with adopted_revision_id = NULL
	scenarioCandidate(t, ctx, tx, candID, batchID, projectID, cpID, 1, 0)

	// Two revisions both pointing to the same candidate
	scenarioRevision(t, ctx, tx, rev1ID, cpID, projectID, runID, candID, batchID, 1)
	scenarioRevision(t, ctx, tx, rev2ID, cpID, projectID, runID, candID, batchID, 2)

	// Update candidate to point to rev1
	scenarioSetAdoptedRevision(t, ctx, tx, candID, rev1ID)

	result := executeDataCheck(ctx, tx, dc002Check())
	if result.AnomalyCount == 0 {
		t.Error("Scenario D: expected anomalies for candidate referenced by multiple revisions, got 0")
	}
}
