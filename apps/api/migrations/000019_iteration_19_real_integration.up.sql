-- Iteration 19: persistence for validated integrations and recoverable runtime.

ALTER TABLE llm_provider_configurations
    ALTER COLUMN integration_status SET DEFAULT 'unverified',
    ADD COLUMN last_verified_version INTEGER NULL,
    ADD COLUMN validation_details JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN model_catalog_updated_at TIMESTAMPTZ NULL;

ALTER TABLE workflow_connections
    ALTER COLUMN integration_status SET DEFAULT 'unverified',
    ADD COLUMN last_verified_version INTEGER NULL,
    ADD COLUMN validation_details JSONB NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE workflow_configurations
    ALTER COLUMN integration_status SET DEFAULT 'unverified',
    ADD COLUMN llm_strategy TEXT NULL,
    ADD COLUMN llm_provider_id UUID NULL,
    ADD COLUMN llm_model VARCHAR(200) NULL,
    ADD COLUMN last_verified_version INTEGER NULL,
    ADD COLUMN validation_details JSONB NOT NULL DEFAULT '{}'::jsonb;

CREATE TABLE llm_provider_models (
    id UUID PRIMARY KEY,
    provider_id UUID NOT NULL REFERENCES llm_provider_configurations(id) ON DELETE CASCADE,
    model_key VARCHAR(200) NOT NULL,
    source TEXT NOT NULL,
    availability TEXT NOT NULL,
    last_seen_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT llm_provider_models_provider_model_key UNIQUE (provider_id, model_key),
    CONSTRAINT llm_provider_models_source_check CHECK (source IN ('discovered', 'manual')),
    CONSTRAINT llm_provider_models_availability_check CHECK (availability IN ('available', 'unavailable')),
    CONSTRAINT llm_provider_models_model_key_check CHECK (char_length(btrim(model_key)) BETWEEN 1 AND 200)
);

UPDATE llm_provider_configurations
SET last_verified_version = CASE WHEN integration_status = 'connected' THEN version ELSE NULL END,
    integration_status = CASE integration_status
        WHEN 'connected' THEN 'verified'
        WHEN 'not_connected' THEN 'unverified'
        ELSE integration_status
    END;

UPDATE workflow_connections
SET last_verified_version = CASE WHEN integration_status = 'connected' THEN version ELSE NULL END,
    integration_status = CASE integration_status
        WHEN 'connected' THEN 'verified'
        WHEN 'not_connected' THEN 'unverified'
        ELSE integration_status
    END;

UPDATE workflow_configurations
SET last_verified_version = CASE WHEN integration_status = 'connected' THEN version ELSE NULL END,
    integration_status = CASE integration_status
        WHEN 'connected' THEN 'verified'
        WHEN 'not_connected' THEN 'unverified'
        ELSE integration_status
    END,
    llm_strategy = 'none';

ALTER TABLE workflow_configurations
    ALTER COLUMN llm_strategy SET NOT NULL,
    ALTER COLUMN llm_strategy SET DEFAULT 'none',
    ADD CONSTRAINT workflow_configurations_llm_provider_id_fkey
        FOREIGN KEY (llm_provider_id) REFERENCES llm_provider_configurations(id) ON DELETE RESTRICT;

ALTER TABLE llm_provider_configurations
    ADD CONSTRAINT llm_provider_configurations_validation_status_check CHECK (
        integration_status IN ('unverified', 'verifying', 'verified', 'failed', 'stale')
    ) NOT VALID,
    ADD CONSTRAINT llm_provider_configurations_validation_details_check CHECK (
        jsonb_typeof(validation_details) = 'object'
    ) NOT VALID,
    ADD CONSTRAINT llm_provider_configurations_verified_version_check CHECK (
        (integration_status = 'verified' AND last_verified_version = version)
        OR (integration_status <> 'verified' AND (last_verified_version IS NULL OR last_verified_version BETWEEN 1 AND version))
    ) NOT VALID;

ALTER TABLE workflow_connections
    ADD CONSTRAINT workflow_connections_validation_status_check CHECK (
        integration_status IN ('unverified', 'verifying', 'verified', 'failed', 'stale')
    ) NOT VALID,
    ADD CONSTRAINT workflow_connections_validation_details_check CHECK (
        jsonb_typeof(validation_details) = 'object'
    ) NOT VALID,
    ADD CONSTRAINT workflow_connections_verified_version_check CHECK (
        (integration_status = 'verified' AND last_verified_version = version)
        OR (integration_status <> 'verified' AND (last_verified_version IS NULL OR last_verified_version BETWEEN 1 AND version))
    ) NOT VALID;

ALTER TABLE workflow_configurations
    ADD CONSTRAINT workflow_configurations_validation_status_check CHECK (
        integration_status IN ('unverified', 'verifying', 'verified', 'failed', 'stale')
    ) NOT VALID,
    ADD CONSTRAINT workflow_configurations_validation_details_check CHECK (
        jsonb_typeof(validation_details) = 'object'
    ) NOT VALID,
    ADD CONSTRAINT workflow_configurations_verified_version_check CHECK (
        (integration_status = 'verified' AND last_verified_version = version)
        OR (integration_status <> 'verified' AND (last_verified_version IS NULL OR last_verified_version BETWEEN 1 AND version))
    ) NOT VALID,
    ADD CONSTRAINT workflow_configurations_llm_strategy_shape_check CHECK (
        (llm_strategy = 'acf_managed' AND llm_provider_id IS NOT NULL AND llm_model IS NOT NULL AND char_length(btrim(llm_model)) BETWEEN 1 AND 200)
        OR (llm_strategy IN ('n8n_managed', 'none') AND llm_provider_id IS NULL AND llm_model IS NULL)
    ) NOT VALID;

ALTER TABLE llm_provider_configurations
    VALIDATE CONSTRAINT llm_provider_configurations_validation_status_check,
    VALIDATE CONSTRAINT llm_provider_configurations_validation_details_check,
    VALIDATE CONSTRAINT llm_provider_configurations_verified_version_check;

ALTER TABLE workflow_connections
    VALIDATE CONSTRAINT workflow_connections_validation_status_check,
    VALIDATE CONSTRAINT workflow_connections_validation_details_check,
    VALIDATE CONSTRAINT workflow_connections_verified_version_check;

ALTER TABLE workflow_configurations
    VALIDATE CONSTRAINT workflow_configurations_validation_status_check,
    VALIDATE CONSTRAINT workflow_configurations_validation_details_check,
    VALIDATE CONSTRAINT workflow_configurations_verified_version_check,
    VALIDATE CONSTRAINT workflow_configurations_llm_strategy_shape_check;

ALTER TABLE workflow_run_records
    ADD COLUMN failure_phase TEXT NULL,
    ADD COLUMN failure_code VARCHAR(100) NULL,
    ADD COLUMN safe_error_message VARCHAR(500) NULL,
    ADD COLUMN retryability TEXT NOT NULL DEFAULT 'not_retryable',
    ADD COLUMN retry_mode TEXT NULL,
    ADD COLUMN external_execution_id VARCHAR(200) NULL,
    ADD COLUMN cancellation_requested_at TIMESTAMPTZ NULL,
    ADD COLUMN timed_out_at TIMESTAMPTZ NULL,
    ADD COLUMN binding_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN connection_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN llm_policy_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE workflow_run_records
    DROP CONSTRAINT workflow_run_records_status_check,
    DROP CONSTRAINT workflow_run_records_error_shape_check,
    DROP CONSTRAINT workflow_run_records_time_shape_check,
    ADD CONSTRAINT workflow_run_records_status_check CHECK (
        status IN ('queued', 'running', 'cancelling', 'succeeded', 'failed', 'cancelled', 'timed_out')
    ),
    ADD CONSTRAINT workflow_run_records_failure_phase_check CHECK (
        failure_phase IS NULL OR failure_phase IN ('external_execution', 'output_validation', 'result_consumption', 'cancellation')
    ),
    ADD CONSTRAINT workflow_run_records_retryability_check CHECK (
        retryability IN ('runtime_retry', 'result_consumption_retry', 'not_retryable')
    ),
    ADD CONSTRAINT workflow_run_records_retry_mode_check CHECK (
        retry_mode IS NULL OR retry_mode IN ('current_configuration', 'original_configuration')
    ),
    ADD CONSTRAINT workflow_run_records_retry_not_self_check CHECK (
        retry_of_run_id IS NULL OR retry_of_run_id <> id
    ),
    ADD CONSTRAINT workflow_run_records_snapshot_shape_check CHECK (
        jsonb_typeof(binding_snapshot) = 'object'
        AND jsonb_typeof(connection_snapshot) = 'object'
        AND jsonb_typeof(llm_policy_snapshot) = 'object'
    ),
    ADD CONSTRAINT workflow_run_records_failure_shape_check CHECK (
        (status IN ('failed', 'timed_out') AND COALESCE(failure_code, error_code) IS NOT NULL AND COALESCE(safe_error_message, error_message) IS NOT NULL)
        OR (status NOT IN ('failed', 'timed_out') AND failure_phase IS NULL AND failure_code IS NULL AND safe_error_message IS NULL AND error_code IS NULL AND error_message IS NULL AND error_details IS NULL)
    ),
    ADD CONSTRAINT workflow_run_records_time_shape_check CHECK (
        (status = 'queued' AND started_at IS NULL AND finished_at IS NULL AND cancelled_at IS NULL AND timed_out_at IS NULL)
        OR (status IN ('running', 'cancelling') AND started_at IS NOT NULL AND finished_at IS NULL AND cancelled_at IS NULL AND timed_out_at IS NULL)
        OR (status IN ('succeeded', 'failed') AND started_at IS NOT NULL AND finished_at IS NOT NULL AND cancelled_at IS NULL AND timed_out_at IS NULL)
        OR (status = 'cancelled' AND cancelled_at IS NOT NULL AND timed_out_at IS NULL)
        OR (status = 'timed_out' AND started_at IS NOT NULL AND finished_at IS NOT NULL AND timed_out_at IS NOT NULL AND cancelled_at IS NULL)
    );

ALTER TABLE workflow_run_events
    DROP CONSTRAINT workflow_run_events_status_check,
    DROP CONSTRAINT workflow_run_events_event_type_check,
    ADD CONSTRAINT workflow_run_events_status_check CHECK (
        status IN ('queued', 'running', 'cancelling', 'succeeded', 'failed', 'cancelled', 'timed_out')
    ),
    ADD CONSTRAINT workflow_run_events_event_type_check CHECK (
        event_type IN (
            'created', 'queued', 'worker_started', 'execution_started', 'request_sent', 'response_received',
            'llm_started', 'llm_completed', 'output_validated', 'cancel_requested', 'cancelled', 'timed_out',
            'output_validation_failed', 'result_consumed', 'result_consumption_failed', 'succeeded', 'failed', 'retry_created'
        )
    );

DROP INDEX workflow_run_records_active_content_generation_subject_idx;
DROP INDEX workflow_run_records_active_review_subject_idx;
DROP INDEX workflow_run_records_active_rewrite_subject_idx;

CREATE UNIQUE INDEX workflow_run_records_active_content_generation_subject_idx
    ON workflow_run_records (project_id, stage, subject_type, subject_id)
    WHERE stage = 'content_generation' AND status IN ('queued', 'running', 'cancelling') AND subject_type = 'content_item' AND subject_id IS NOT NULL;
CREATE UNIQUE INDEX workflow_run_records_active_review_subject_idx
    ON workflow_run_records (project_id, stage, subject_type, subject_id)
    WHERE stage = 'review' AND status IN ('queued', 'running', 'cancelling') AND subject_type = 'content_version' AND subject_id IS NOT NULL;
CREATE UNIQUE INDEX workflow_run_records_active_rewrite_subject_idx
    ON workflow_run_records (project_id, stage, subject_type, subject_id)
    WHERE stage = 'rewrite' AND status IN ('queued', 'running', 'cancelling') AND subject_type = 'review_report' AND subject_id IS NOT NULL;

CREATE INDEX llm_provider_configurations_validation_enabled_updated_idx
    ON llm_provider_configurations (integration_status, enabled, updated_at DESC);
CREATE INDEX workflow_connections_validation_enabled_updated_idx
    ON workflow_connections (integration_status, enabled, updated_at DESC);
CREATE INDEX workflow_configurations_stage_strategy_validation_enabled_idx
    ON workflow_configurations ((applicable_stages), llm_strategy, integration_status, enabled);
CREATE INDEX llm_provider_models_provider_availability_model_idx
    ON llm_provider_models (provider_id, availability, model_key);
CREATE INDEX workflow_run_records_project_stage_status_created_idx
    ON workflow_run_records (project_id, stage, status, created_at DESC, id DESC);
CREATE UNIQUE INDEX workflow_run_records_connection_execution_unique_idx
    ON workflow_run_records ((configuration_snapshot #>> '{connection,id}'), external_execution_id)
    WHERE external_execution_id IS NOT NULL;
