CREATE TABLE workflow_run_records (
    id UUID PRIMARY KEY,
    run_number VARCHAR(80) NOT NULL,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    stage VARCHAR(40) NOT NULL,
    workflow_configuration_id UUID NOT NULL REFERENCES workflow_configurations(id),
    trigger_source VARCHAR(40) NOT NULL,
    status VARCHAR(30) NOT NULL DEFAULT 'queued',
    configuration_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    input_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    output_payload JSONB NULL,
    error_code VARCHAR(80) NULL,
    error_message VARCHAR(300) NULL,
    error_details JSONB NULL,
    idempotency_key VARCHAR(128) NULL,
    retry_of_run_id UUID NULL REFERENCES workflow_run_records(id),
    started_at TIMESTAMPTZ NULL,
    finished_at TIMESTAMPTZ NULL,
    cancelled_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    version INTEGER NOT NULL DEFAULT 1,
    CONSTRAINT workflow_run_records_run_number_key UNIQUE (run_number),
    CONSTRAINT workflow_run_records_stage_check CHECK (stage IN ('chapter_planning', 'content_generation', 'review', 'rewrite')),
    CONSTRAINT workflow_run_records_status_check CHECK (status IN ('queued', 'running', 'succeeded', 'failed', 'cancelled')),
    CONSTRAINT workflow_run_records_version_check CHECK (version >= 1),
    CONSTRAINT workflow_run_records_snapshot_object_check CHECK (jsonb_typeof(configuration_snapshot) = 'object'),
    CONSTRAINT workflow_run_records_input_object_check CHECK (jsonb_typeof(input_payload) = 'object'),
    CONSTRAINT workflow_run_records_output_object_check CHECK (output_payload IS NULL OR jsonb_typeof(output_payload) = 'object'),
    CONSTRAINT workflow_run_records_error_details_object_check CHECK (error_details IS NULL OR jsonb_typeof(error_details) = 'object'),
    CONSTRAINT workflow_run_records_error_shape_check CHECK (
        (status = 'failed' AND error_code IS NOT NULL AND error_message IS NOT NULL)
        OR (status <> 'failed' AND error_code IS NULL AND error_message IS NULL AND error_details IS NULL)
    ),
    CONSTRAINT workflow_run_records_time_shape_check CHECK (
        (status = 'queued' AND started_at IS NULL AND finished_at IS NULL AND cancelled_at IS NULL)
        OR (status = 'running' AND started_at IS NOT NULL AND finished_at IS NULL AND cancelled_at IS NULL)
        OR (status IN ('succeeded', 'failed') AND started_at IS NOT NULL AND finished_at IS NOT NULL AND cancelled_at IS NULL)
        OR (status = 'cancelled' AND cancelled_at IS NOT NULL)
    ),
    CONSTRAINT workflow_run_records_idempotency_key_check CHECK (idempotency_key IS NULL OR char_length(btrim(idempotency_key)) BETWEEN 1 AND 128),
    CONSTRAINT workflow_run_records_project_stage_idempotency_key_unique UNIQUE NULLS NOT DISTINCT (project_id, stage, idempotency_key)
);

CREATE TABLE workflow_run_events (
    id UUID PRIMARY KEY,
    run_id UUID NOT NULL REFERENCES workflow_run_records(id) ON DELETE CASCADE,
    event_type VARCHAR(60) NOT NULL,
    status VARCHAR(30) NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT workflow_run_events_status_check CHECK (status IN ('queued', 'running', 'succeeded', 'failed', 'cancelled')),
    CONSTRAINT workflow_run_events_payload_object_check CHECK (jsonb_typeof(payload) = 'object')
);

CREATE INDEX workflow_run_records_project_created_at_id_idx ON workflow_run_records (project_id, created_at DESC, id DESC);
CREATE INDEX workflow_run_records_project_status_created_at_id_idx ON workflow_run_records (project_id, status, created_at DESC, id DESC);
CREATE INDEX workflow_run_records_workflow_configuration_id_idx ON workflow_run_records (workflow_configuration_id);
CREATE INDEX workflow_run_records_retry_of_run_id_idx ON workflow_run_records (retry_of_run_id) WHERE retry_of_run_id IS NOT NULL;
CREATE INDEX workflow_run_events_run_created_at_id_idx ON workflow_run_events (run_id, created_at ASC, id ASC);
