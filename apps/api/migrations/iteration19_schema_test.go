package migrations

import (
	"os"
	"strings"
	"testing"
)

func TestIteration19SchemaMapping(t *testing.T) {
	t.Parallel()
	up := readMigration(t, "000019_iteration_19_real_integration.up.sql")

	required := []string{
		"CREATE TABLE llm_provider_models",
		"ADD COLUMN last_verified_version INTEGER NULL",
		"ADD COLUMN validation_details JSONB NOT NULL DEFAULT '{}'::jsonb",
		"ADD COLUMN model_catalog_updated_at TIMESTAMPTZ NULL",
		"ADD COLUMN llm_strategy TEXT NULL",
		"ADD COLUMN llm_provider_id UUID NULL",
		"ADD COLUMN llm_model VARCHAR(200) NULL",
		"workflow_configurations_llm_strategy_shape_check",
		"workflow_run_records_retry_not_self_check",
		"ADD COLUMN failure_phase TEXT NULL",
		"ADD COLUMN retryability TEXT NOT NULL DEFAULT 'not_retryable'",
		"ADD COLUMN retry_mode TEXT NULL",
		"ADD COLUMN external_execution_id VARCHAR(200) NULL",
		"ADD COLUMN cancellation_requested_at TIMESTAMPTZ NULL",
		"ADD COLUMN timed_out_at TIMESTAMPTZ NULL",
		"ADD COLUMN binding_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb",
		"ADD COLUMN connection_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb",
		"ADD COLUMN llm_policy_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb",
		"workflow_run_records_connection_execution_unique_idx",
	}
	for _, fragment := range required {
		if !strings.Contains(up, fragment) {
			t.Errorf("Migration 19 up is missing %q", fragment)
		}
	}

	forbidden := []string{
		"ADD COLUMN executable",
		"ADD COLUMN bound",
		"workflow_configuration_histories",
		"workflow_configuration_versions",
		"secret_histories",
		"credential_histories",
	}
	for _, fragment := range forbidden {
		if strings.Contains(strings.ToLower(up), strings.ToLower(fragment)) {
			t.Errorf("Migration 19 persists forbidden derived/history state %q", fragment)
		}
	}
}

func TestIteration19DownReversesOwnedObjects(t *testing.T) {
	t.Parallel()
	down := readMigration(t, "000019_iteration_19_real_integration.down.sql")
	required := []string{
		"DROP TABLE llm_provider_models",
		"DROP COLUMN model_catalog_updated_at",
		"DROP COLUMN llm_strategy",
		"DROP COLUMN failure_phase",
		"DROP COLUMN binding_snapshot",
		"DROP INDEX workflow_run_records_connection_execution_unique_idx",
	}
	for _, fragment := range required {
		if !strings.Contains(down, fragment) {
			t.Errorf("Migration 19 down is missing %q", fragment)
		}
	}
	if strings.Contains(down, "DROP TABLE workflow_run_records") || strings.Contains(down, "DROP TABLE workflow_configurations") {
		t.Fatal("Migration 19 down must not drop a pre-existing aggregate table")
	}
}

func TestWorkflowRunRuntimeIntegrityMigration(t *testing.T) {
	t.Parallel()
	up := readMigration(t, "000022_workflow_run_runtime_integrity.up.sql")
	for _, fragment := range []string{
		"ADD COLUMN workflow_connection_id UUID NULL",
		"ADD COLUMN deadline_at TIMESTAMPTZ NULL",
		"ADD COLUMN cancellation_reason TEXT NULL",
		"workflow_run_records_connection_execution_unique_idx",
		"resolved_webhook_path",
		"resolved_workflow_id",
		"resolved_workflow_revision",
	} {
		if !strings.Contains(up, fragment) {
			t.Errorf("Migration 22 up is missing %q", fragment)
		}
	}
	down := readMigration(t, "000022_workflow_run_runtime_integrity.down.sql")
	for _, fragment := range []string{"DROP COLUMN cancellation_reason", "DROP COLUMN deadline_at", "DROP COLUMN workflow_connection_id", "DROP COLUMN resolved_webhook_path"} {
		if !strings.Contains(down, fragment) {
			t.Errorf("Migration 22 down is missing %q", fragment)
		}
	}
}

func TestWorkflowRunCoreStabilityMigration(t *testing.T) {
	t.Parallel()
	up := readMigration(t, "000023_workflow_run_core_stability.up.sql")
	for _, fragment := range []string{
		"ADD COLUMN next_event_sequence BIGINT NOT NULL DEFAULT 1",
		"ADD COLUMN sequence BIGINT NULL",
		"workflow_run_events_run_id_sequence_unique",
		"workflow_run_events_run_sequence_idx",
		"status IN ('queued', 'running', 'cancelling')",
		"workflow_run_records_active_chapter_planning_idx",
	} {
		if !strings.Contains(up, fragment) {
			t.Errorf("Migration 23 up is missing %q", fragment)
		}
	}
	down := readMigration(t, "000023_workflow_run_core_stability.down.sql")
	for _, fragment := range []string{
		"DROP COLUMN IF EXISTS sequence",
		"DROP COLUMN IF EXISTS next_event_sequence",
		"status IN ('queued', 'running')",
	} {
		if !strings.Contains(down, fragment) {
			t.Errorf("Migration 23 down is missing %q", fragment)
		}
	}
}

func readMigration(t *testing.T, name string) string {
	t.Helper()
	contents, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return string(contents)
}
