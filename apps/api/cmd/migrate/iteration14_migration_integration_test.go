package main

import (
	"context"
	"testing"
)

func TestIteration14MigrationWorkflowRunRuntimeFinalState(t *testing.T) {
	db, ctx, _ := openIteration13MigrationDB(t)
	for _, table := range []string{"workflow_run_records", "workflow_run_events"} {
		var exists bool
		if err := db.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema=current_schema() AND table_name=$1)", table).Scan(&exists); err != nil || !exists {
			t.Fatalf("table %s exists=%t err=%v", table, exists, err)
		}
	}
	for _, index := range []string{"workflow_run_records_project_created_at_id_idx", "workflow_run_records_project_status_created_at_id_idx", "workflow_run_events_run_created_at_id_idx"} {
		var exists bool
		if err := db.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_indexes WHERE schemaname=current_schema() AND indexname=$1)", index).Scan(&exists); err != nil || !exists {
			t.Fatalf("index %s exists=%t err=%v", index, exists, err)
		}
	}
	for _, constraint := range []string{"workflow_run_records_status_check", "workflow_run_records_time_shape_check", "workflow_run_events_status_check"} {
		var exists bool
		if err := db.QueryRow(context.Background(), "SELECT EXISTS (SELECT 1 FROM pg_constraint WHERE conname=$1)", constraint).Scan(&exists); err != nil || !exists {
			t.Fatalf("constraint %s exists=%t err=%v", constraint, exists, err)
		}
	}
	var idempotencyColumnExists bool
	if err := db.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='workflow_run_records' AND column_name='idempotency_key')").Scan(&idempotencyColumnExists); err != nil || idempotencyColumnExists {
		t.Fatalf("workflow_run_records idempotency_key exists=%t err=%v", idempotencyColumnExists, err)
	}
	for _, name := range []string{"workflow_run_records_idempotency_key_check", "workflow_run_records_project_stage_idempotency_key_unique"} {
		var exists bool
		if err := db.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_constraint WHERE conname=$1)", name).Scan(&exists); err != nil || exists {
			t.Fatalf("idempotency constraint %s exists=%t err=%v", name, exists, err)
		}
	}
}
