DROP INDEX IF EXISTS workflow_run_events_one_result_consumed_idx;
DROP INDEX IF EXISTS workflow_run_events_one_succeeded_idx;
DROP TABLE IF EXISTS workflow_run_result_consumptions;
ALTER TABLE workflow_run_events DROP CONSTRAINT workflow_run_events_event_type_check;
ALTER TABLE workflow_run_events ADD CONSTRAINT workflow_run_events_event_type_check CHECK (event_type IN (
    'created','queued','worker_started','execution_started','request_sent','response_received','llm_started','llm_completed',
    'output_validated','cancel_requested','cancelled','timed_out','output_validation_failed','result_consumed',
    'result_consumption_failed','succeeded','failed','retry_created'
));
