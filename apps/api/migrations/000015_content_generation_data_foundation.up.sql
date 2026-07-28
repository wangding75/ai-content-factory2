ALTER TABLE workflow_run_records
    ADD COLUMN subject_type VARCHAR(40) NULL,
    ADD COLUMN subject_id UUID NULL,
    ADD CONSTRAINT workflow_run_records_subject_pair_check CHECK (
        (subject_type IS NULL AND subject_id IS NULL) OR
        (subject_type IS NOT NULL AND subject_id IS NOT NULL)
    );

CREATE UNIQUE INDEX workflow_run_records_active_content_generation_subject_idx
    ON workflow_run_records (project_id, stage, subject_type, subject_id)
    WHERE stage = 'content_generation'
      AND status IN ('queued', 'running')
      AND subject_type = 'content_item'
      AND subject_id IS NOT NULL;

CREATE INDEX workflow_run_records_project_subject_stage_created_at_id_idx
    ON workflow_run_records (project_id, subject_type, subject_id, stage, created_at DESC, id DESC);

ALTER TABLE content_versions
    ADD COLUMN source_content_version_id UUID NULL,
    ADD COLUMN source_content_version_version INTEGER NULL,
    ADD COLUMN source_workflow_run_id UUID NULL;

ALTER TABLE content_versions DROP CONSTRAINT content_versions_source_check;
ALTER TABLE content_versions
    ADD CONSTRAINT content_versions_source_check CHECK (
        source IN ('manual_created', 'mock_generated', 'mock_rewrite', 'workflow_generated')
    ),
    ADD CONSTRAINT content_versions_source_content_version_fk
        FOREIGN KEY (content_item_id, source_content_version_id)
        REFERENCES content_versions(content_item_id, id) ON DELETE RESTRICT,
    ADD CONSTRAINT content_versions_source_workflow_run_id_fkey
        FOREIGN KEY (source_workflow_run_id)
        REFERENCES workflow_run_records(id) ON DELETE RESTRICT,
    ADD CONSTRAINT content_versions_workflow_generated_shape CHECK (
        source <> 'workflow_generated' OR (
            status = 'editable_draft' AND version = 1 AND frozen_at IS NULL AND
            source_content_version_id IS NOT NULL AND
            source_content_version_version IS NOT NULL AND source_content_version_version >= 1 AND
            source_workflow_run_id IS NOT NULL
        )
    );

CREATE UNIQUE INDEX content_versions_source_workflow_run_unique_idx
    ON content_versions (source_workflow_run_id)
    WHERE source_workflow_run_id IS NOT NULL;

CREATE INDEX content_versions_content_item_source_version_no_id_idx
    ON content_versions (content_item_id, source, version_no DESC, id DESC);

ALTER TABLE workflow_run_events
    ADD CONSTRAINT workflow_run_events_event_type_check CHECK (
        event_type IN (
            'queued', 'worker_started', 'request_sent', 'response_received',
            'output_validated', 'result_consumed', 'result_consumption_failed',
            'succeeded', 'failed', 'cancelled', 'retry_created'
        )
    );
