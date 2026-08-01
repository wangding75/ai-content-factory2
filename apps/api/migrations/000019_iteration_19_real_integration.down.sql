-- Reverse only objects introduced by Migration 19.

DROP INDEX workflow_run_records_connection_execution_unique_idx;
DROP INDEX workflow_run_records_project_stage_status_created_idx;
DROP INDEX llm_provider_models_provider_availability_model_idx;
DROP INDEX workflow_configurations_stage_strategy_validation_enabled_idx;
DROP INDEX workflow_connections_validation_enabled_updated_idx;
DROP INDEX llm_provider_configurations_validation_enabled_updated_idx;

DROP INDEX workflow_run_records_active_content_generation_subject_idx;
DROP INDEX workflow_run_records_active_review_subject_idx;
DROP INDEX workflow_run_records_active_rewrite_subject_idx;

CREATE UNIQUE INDEX workflow_run_records_active_content_generation_subject_idx
    ON workflow_run_records (project_id, stage, subject_type, subject_id)
    WHERE stage = 'content_generation' AND status IN ('queued', 'running') AND subject_type = 'content_item' AND subject_id IS NOT NULL;
CREATE UNIQUE INDEX workflow_run_records_active_review_subject_idx
    ON workflow_run_records (project_id, stage, subject_type, subject_id)
    WHERE stage = 'review' AND status IN ('queued', 'running') AND subject_type = 'content_version' AND subject_id IS NOT NULL;
CREATE UNIQUE INDEX workflow_run_records_active_rewrite_subject_idx
    ON workflow_run_records (project_id, stage, subject_type, subject_id)
    WHERE stage = 'rewrite' AND status IN ('queued', 'running') AND subject_type = 'review_report' AND subject_id IS NOT NULL;

ALTER TABLE workflow_run_events
    DROP CONSTRAINT workflow_run_events_status_check,
    DROP CONSTRAINT workflow_run_events_event_type_check;

UPDATE workflow_run_events
SET status = CASE status
    WHEN 'cancelling' THEN 'cancelled'
    WHEN 'timed_out' THEN 'failed'
    ELSE status
END,
event_type = CASE event_type
    WHEN 'created' THEN 'queued'
    WHEN 'execution_started' THEN 'worker_started'
    WHEN 'llm_started' THEN 'request_sent'
    WHEN 'llm_completed' THEN 'response_received'
    WHEN 'cancel_requested' THEN 'cancelled'
    WHEN 'timed_out' THEN 'failed'
    ELSE event_type
END;

ALTER TABLE workflow_run_events
    ADD CONSTRAINT workflow_run_events_status_check CHECK (status IN ('queued', 'running', 'succeeded', 'failed', 'cancelled')),
    ADD CONSTRAINT workflow_run_events_event_type_check CHECK (
        event_type IN ('queued', 'worker_started', 'request_sent', 'response_received', 'output_validated', 'output_validation_failed', 'result_consumed', 'result_consumption_failed', 'succeeded', 'failed', 'cancelled', 'retry_created')
    );

ALTER TABLE workflow_run_records
    DROP CONSTRAINT workflow_run_records_status_check,
    DROP CONSTRAINT workflow_run_records_failure_phase_check,
    DROP CONSTRAINT workflow_run_records_retryability_check,
    DROP CONSTRAINT workflow_run_records_retry_mode_check,
    DROP CONSTRAINT workflow_run_records_retry_not_self_check,
    DROP CONSTRAINT workflow_run_records_snapshot_shape_check,
    DROP CONSTRAINT workflow_run_records_failure_shape_check,
    DROP CONSTRAINT workflow_run_records_time_shape_check;

UPDATE workflow_run_records
SET status = CASE status
        WHEN 'cancelling' THEN 'cancelled'
        WHEN 'timed_out' THEN 'failed'
        ELSE status
    END,
    error_code = CASE WHEN status = 'timed_out' THEN COALESCE(error_code, failure_code, 'timed_out') ELSE error_code END,
    error_message = CASE WHEN status = 'timed_out' THEN COALESCE(error_message, safe_error_message, 'workflow run timed out') ELSE error_message END,
    error_details = CASE WHEN status = 'timed_out' THEN COALESCE(error_details, '{}'::jsonb) ELSE error_details END,
    started_at = CASE WHEN status = 'cancelling' AND started_at IS NULL THEN COALESCE(cancellation_requested_at, updated_at) ELSE started_at END,
    cancelled_at = CASE WHEN status = 'cancelling' THEN COALESCE(cancelled_at, cancellation_requested_at, updated_at) ELSE cancelled_at END,
    finished_at = CASE WHEN status = 'timed_out' THEN COALESCE(finished_at, timed_out_at, updated_at) ELSE finished_at END;

ALTER TABLE workflow_run_records
    ADD CONSTRAINT workflow_run_records_status_check CHECK (status IN ('queued', 'running', 'succeeded', 'failed', 'cancelled')),
    ADD CONSTRAINT workflow_run_records_error_shape_check CHECK (
        (status = 'failed' AND error_code IS NOT NULL AND error_message IS NOT NULL)
        OR (status <> 'failed' AND error_code IS NULL AND error_message IS NULL AND error_details IS NULL)
    ),
    ADD CONSTRAINT workflow_run_records_time_shape_check CHECK (
        (status = 'queued' AND started_at IS NULL AND finished_at IS NULL AND cancelled_at IS NULL)
        OR (status = 'running' AND started_at IS NOT NULL AND finished_at IS NULL AND cancelled_at IS NULL)
        OR (status IN ('succeeded', 'failed') AND started_at IS NOT NULL AND finished_at IS NOT NULL AND cancelled_at IS NULL)
        OR (status = 'cancelled' AND cancelled_at IS NOT NULL)
    );

ALTER TABLE workflow_run_records
    DROP COLUMN llm_policy_snapshot,
    DROP COLUMN connection_snapshot,
    DROP COLUMN binding_snapshot,
    DROP COLUMN timed_out_at,
    DROP COLUMN cancellation_requested_at,
    DROP COLUMN external_execution_id,
    DROP COLUMN retry_mode,
    DROP COLUMN retryability,
    DROP COLUMN safe_error_message,
    DROP COLUMN failure_code,
    DROP COLUMN failure_phase;

ALTER TABLE workflow_configurations
    DROP CONSTRAINT workflow_configurations_llm_strategy_shape_check,
    DROP CONSTRAINT workflow_configurations_verified_version_check,
    DROP CONSTRAINT workflow_configurations_validation_details_check,
    DROP CONSTRAINT workflow_configurations_validation_status_check,
    DROP CONSTRAINT workflow_configurations_llm_provider_id_fkey;
ALTER TABLE workflow_connections
    DROP CONSTRAINT workflow_connections_verified_version_check,
    DROP CONSTRAINT workflow_connections_validation_details_check,
    DROP CONSTRAINT workflow_connections_validation_status_check;
ALTER TABLE llm_provider_configurations
    DROP CONSTRAINT llm_provider_configurations_verified_version_check,
    DROP CONSTRAINT llm_provider_configurations_validation_details_check,
    DROP CONSTRAINT llm_provider_configurations_validation_status_check;

DROP TABLE llm_provider_models;

UPDATE llm_provider_configurations
SET integration_status = CASE integration_status WHEN 'verified' THEN 'connected' ELSE 'not_connected' END;
UPDATE workflow_connections
SET integration_status = CASE integration_status WHEN 'verified' THEN 'connected' ELSE 'not_connected' END;
UPDATE workflow_configurations
SET integration_status = CASE integration_status WHEN 'verified' THEN 'connected' ELSE 'not_connected' END;

ALTER TABLE workflow_configurations
    ALTER COLUMN integration_status SET DEFAULT 'not_connected',
    DROP COLUMN validation_details,
    DROP COLUMN last_verified_version,
    DROP COLUMN llm_model,
    DROP COLUMN llm_provider_id,
    DROP COLUMN llm_strategy;
ALTER TABLE workflow_connections
    ALTER COLUMN integration_status SET DEFAULT 'not_connected',
    DROP COLUMN validation_details,
    DROP COLUMN last_verified_version;
ALTER TABLE llm_provider_configurations
    ALTER COLUMN integration_status SET DEFAULT 'not_connected',
    DROP COLUMN model_catalog_updated_at,
    DROP COLUMN validation_details,
    DROP COLUMN last_verified_version;
