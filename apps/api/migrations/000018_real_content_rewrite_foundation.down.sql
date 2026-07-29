DROP TRIGGER IF EXISTS content_versions_workflow_rewrite_scope_trigger
    ON content_versions;
DROP FUNCTION IF EXISTS enforce_workflow_rewrite_candidate_scope();

DROP TRIGGER IF EXISTS workflow_run_records_rewrite_scope_trigger
    ON workflow_run_records;
DROP FUNCTION IF EXISTS enforce_rewrite_workflow_run_scope();

DROP INDEX IF EXISTS workflow_run_records_active_rewrite_subject_idx;

ALTER TABLE workflow_run_records
    DROP CONSTRAINT IF EXISTS workflow_run_records_rewrite_subject_shape_check;

ALTER TABLE content_versions
    DROP CONSTRAINT IF EXISTS content_versions_workflow_rewrite_shape,
    DROP CONSTRAINT content_versions_source_check,
    ADD CONSTRAINT content_versions_source_check CHECK (
        source IN (
            'manual_created',
            'mock_generated',
            'mock_rewrite',
            'workflow_generated'
        )
    );
