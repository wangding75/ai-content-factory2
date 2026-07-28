ALTER TABLE workflow_run_events DROP CONSTRAINT workflow_run_events_event_type_check;

DROP INDEX content_versions_content_item_source_version_no_id_idx;
DROP INDEX content_versions_source_workflow_run_unique_idx;

ALTER TABLE content_versions
    DROP CONSTRAINT content_versions_workflow_generated_shape,
    DROP CONSTRAINT content_versions_source_workflow_run_id_fkey,
    DROP CONSTRAINT content_versions_source_content_version_fk,
    DROP CONSTRAINT content_versions_source_check,
    ADD CONSTRAINT content_versions_source_check CHECK (
        source IN ('manual_created', 'mock_generated', 'mock_rewrite')
    ),
    DROP COLUMN source_workflow_run_id,
    DROP COLUMN source_content_version_version,
    DROP COLUMN source_content_version_id;

DROP INDEX workflow_run_records_project_subject_stage_created_at_id_idx;
DROP INDEX workflow_run_records_active_content_generation_subject_idx;

ALTER TABLE workflow_run_records
    DROP CONSTRAINT workflow_run_records_subject_pair_check,
    DROP COLUMN subject_id,
    DROP COLUMN subject_type;
