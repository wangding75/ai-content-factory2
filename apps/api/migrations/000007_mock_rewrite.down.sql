DROP INDEX IF EXISTS workflow_runs_content_item_workflow_started_at_id_idx;

ALTER TABLE workflow_runs
    DROP CONSTRAINT IF EXISTS workflow_runs_mock_rewrite_shape,
    DROP CONSTRAINT IF EXISTS workflow_runs_source_review_same_scope,
    DROP CONSTRAINT IF EXISTS workflow_runs_target_version_same_item,
    DROP CONSTRAINT IF EXISTS workflow_runs_workflow_key_check;

ALTER TABLE workflow_runs
    DROP COLUMN IF EXISTS source_review_report_id,
    DROP COLUMN IF EXISTS target_content_version_id,
    ADD CONSTRAINT workflow_runs_workflow_key_check CHECK (
        workflow_key IN ('content_mock_generate', 'content_mock_review')
    );

ALTER TABLE review_reports
    DROP CONSTRAINT IF EXISTS review_reports_project_item_version_id_id_unique;

ALTER TABLE content_versions
    DROP CONSTRAINT IF EXISTS content_versions_mock_rewrite_shape,
    DROP CONSTRAINT IF EXISTS content_versions_source_check,
    ADD CONSTRAINT content_versions_source_check CHECK (
        source IN ('manual_created', 'mock_generated')
    );
