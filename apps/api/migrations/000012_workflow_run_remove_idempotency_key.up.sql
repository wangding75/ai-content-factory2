ALTER TABLE workflow_run_records
    DROP CONSTRAINT workflow_run_records_project_stage_idempotency_key_unique,
    DROP CONSTRAINT workflow_run_records_idempotency_key_check,
    DROP COLUMN idempotency_key;

CREATE INDEX workflow_run_records_created_at_id_idx
    ON workflow_run_records (created_at DESC, id DESC);
