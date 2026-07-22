DROP INDEX IF EXISTS workflow_run_records_created_at_id_idx;

ALTER TABLE workflow_run_records
    ADD COLUMN idempotency_key VARCHAR(128) NULL,
    ADD CONSTRAINT workflow_run_records_idempotency_key_check CHECK (idempotency_key IS NULL OR char_length(btrim(idempotency_key)) BETWEEN 1 AND 128),
    ADD CONSTRAINT workflow_run_records_project_stage_idempotency_key_unique UNIQUE NULLS NOT DISTINCT (project_id, stage, idempotency_key);
