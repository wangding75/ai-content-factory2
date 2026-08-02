CREATE TABLE workflow_run_result_consumptions (
    workflow_run_id UUID PRIMARY KEY REFERENCES workflow_run_records(id) ON DELETE CASCADE,
    status TEXT NOT NULL CHECK (status IN ('pending','in_progress','failed','completed')),
    lease_until TIMESTAMPTZ NULL,
    attempt_count INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    failure_code VARCHAR(100) NULL,
    safe_error_message VARCHAR(500) NULL,
    completed_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK ((status = 'completed') = (completed_at IS NOT NULL))
);
CREATE INDEX workflow_run_result_consumptions_recovery_idx
    ON workflow_run_result_consumptions (status, lease_until, updated_at)
    WHERE status IN ('pending','in_progress','failed');

CREATE UNIQUE INDEX workflow_run_events_one_succeeded_idx
    ON workflow_run_events (run_id) WHERE event_type = 'succeeded';
CREATE UNIQUE INDEX workflow_run_events_one_result_consumed_idx
    ON workflow_run_events (run_id) WHERE event_type = 'result_consumed';

ALTER TABLE workflow_run_events DROP CONSTRAINT workflow_run_events_event_type_check;
ALTER TABLE workflow_run_events ADD CONSTRAINT workflow_run_events_event_type_check CHECK (event_type IN (
    'created','queued','worker_started','execution_started','request_sent','response_received','llm_started','llm_completed',
    'output_validated','cancel_requested','cancelled','timed_out','output_validation_failed','result_consumed',
    'result_consumption_failed','result_consumption_retried','succeeded','failed','retry_created'
));
