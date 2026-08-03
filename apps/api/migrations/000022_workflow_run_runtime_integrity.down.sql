ALTER TABLE workflow_configurations
    DROP CONSTRAINT workflow_configurations_resolution_shape_check,
    DROP COLUMN resolved_workflow_revision,
    DROP COLUMN resolved_workflow_id,
    DROP COLUMN resolved_webhook_path;

DROP INDEX workflow_run_events_one_execution_started_idx;

DROP INDEX workflow_run_records_connection_execution_unique_idx;
CREATE UNIQUE INDEX workflow_run_records_connection_execution_unique_idx
    ON workflow_run_records ((configuration_snapshot #>> '{connection,id}'), external_execution_id)
    WHERE external_execution_id IS NOT NULL;

ALTER TABLE workflow_run_records
    DROP CONSTRAINT workflow_run_records_time_shape_check;

UPDATE workflow_run_records
SET started_at = timed_out_at
WHERE status = 'timed_out' AND started_at IS NULL;

ALTER TABLE workflow_run_records
    ADD CONSTRAINT workflow_run_records_time_shape_check CHECK (
        (status = 'queued' AND started_at IS NULL AND finished_at IS NULL AND cancelled_at IS NULL AND timed_out_at IS NULL)
        OR (status = 'running' AND started_at IS NOT NULL AND finished_at IS NULL AND cancelled_at IS NULL AND timed_out_at IS NULL)
        OR (status = 'cancelling' AND finished_at IS NULL AND cancelled_at IS NULL AND timed_out_at IS NULL)
        OR (status IN ('succeeded', 'failed') AND started_at IS NOT NULL AND finished_at IS NOT NULL AND cancelled_at IS NULL AND timed_out_at IS NULL)
        OR (status = 'cancelled' AND cancelled_at IS NOT NULL AND timed_out_at IS NULL)
        OR (status = 'timed_out' AND started_at IS NOT NULL AND finished_at IS NOT NULL AND timed_out_at IS NOT NULL AND cancelled_at IS NULL)
    );

ALTER TABLE workflow_run_records
    DROP CONSTRAINT workflow_run_records_deadline_shape_check,
    DROP CONSTRAINT workflow_run_records_cancellation_shape_check,
    DROP CONSTRAINT workflow_run_records_cancellation_reason_check,
    DROP COLUMN cancellation_reason,
    DROP COLUMN deadline_at,
    DROP COLUMN workflow_connection_id;
