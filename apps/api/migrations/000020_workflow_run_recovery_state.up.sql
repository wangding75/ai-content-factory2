-- A queued run may be cancelled before a worker writes started_at.  Migration 19
-- could not represent that durable cancelling intent.
UPDATE workflow_run_records
SET finished_at = COALESCE(finished_at, cancelled_at)
WHERE status = 'cancelled' AND finished_at IS NULL;

ALTER TABLE workflow_run_records
    DROP CONSTRAINT workflow_run_records_time_shape_check,
    ADD CONSTRAINT workflow_run_records_time_shape_check CHECK (
        (status = 'queued' AND started_at IS NULL AND finished_at IS NULL AND cancelled_at IS NULL AND timed_out_at IS NULL)
        OR (status = 'running' AND started_at IS NOT NULL AND finished_at IS NULL AND cancelled_at IS NULL AND timed_out_at IS NULL)
        OR (status = 'cancelling' AND finished_at IS NULL AND cancelled_at IS NULL AND timed_out_at IS NULL)
        OR (status IN ('succeeded', 'failed') AND started_at IS NOT NULL AND finished_at IS NOT NULL AND cancelled_at IS NULL AND timed_out_at IS NULL)
        OR (status = 'cancelled' AND cancelled_at IS NOT NULL AND timed_out_at IS NULL)
        OR (status = 'timed_out' AND started_at IS NOT NULL AND finished_at IS NOT NULL AND timed_out_at IS NOT NULL AND cancelled_at IS NULL)
    );
