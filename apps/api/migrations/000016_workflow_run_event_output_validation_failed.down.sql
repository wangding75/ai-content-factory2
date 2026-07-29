ALTER TABLE workflow_run_events
    DROP CONSTRAINT workflow_run_events_event_type_check;

ALTER TABLE workflow_run_events
    ADD CONSTRAINT workflow_run_events_event_type_check CHECK (
        event_type IN (
            'queued', 'worker_started', 'request_sent', 'response_received',
            'output_validated', 'result_consumed', 'result_consumption_failed',
            'succeeded', 'failed', 'cancelled', 'retry_created'
        )
    );
