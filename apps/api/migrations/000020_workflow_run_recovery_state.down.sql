ALTER TABLE workflow_run_records
    DROP CONSTRAINT workflow_run_records_time_shape_check,
    ADD CONSTRAINT workflow_run_records_time_shape_check CHECK (
        (status = 'queued' AND started_at IS NULL AND finished_at IS NULL AND cancelled_at IS NULL AND timed_out_at IS NULL)
        OR (status IN ('running', 'cancelling') AND started_at IS NOT NULL AND finished_at IS NULL AND cancelled_at IS NULL AND timed_out_at IS NULL)
        OR (status IN ('succeeded', 'failed') AND started_at IS NOT NULL AND finished_at IS NOT NULL AND cancelled_at IS NULL AND timed_out_at IS NULL)
        OR (status = 'cancelled' AND cancelled_at IS NOT NULL AND timed_out_at IS NULL)
        OR (status = 'timed_out' AND started_at IS NOT NULL AND finished_at IS NOT NULL AND timed_out_at IS NOT NULL AND cancelled_at IS NULL)
    );
