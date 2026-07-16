ALTER TABLE content_versions
    DROP CONSTRAINT content_versions_source_check;

ALTER TABLE content_versions
    ADD CONSTRAINT content_versions_source_check CHECK (
        source IN ('manual_created', 'mock_generated', 'mock_rewrite')
    ) NOT VALID,
    ADD CONSTRAINT content_versions_mock_rewrite_shape CHECK (
        source <> 'mock_rewrite' OR (version_no = 2 AND status = 'editable_draft')
    ) NOT VALID;

ALTER TABLE content_versions
    VALIDATE CONSTRAINT content_versions_source_check,
    VALIDATE CONSTRAINT content_versions_mock_rewrite_shape;

ALTER TABLE workflow_runs
    DROP CONSTRAINT workflow_runs_workflow_key_check;

ALTER TABLE workflow_runs
    ADD COLUMN target_content_version_id UUID NULL,
    ADD COLUMN source_review_report_id UUID NULL,
    ADD CONSTRAINT workflow_runs_workflow_key_check CHECK (
        workflow_key IN ('content_mock_generate', 'content_mock_review', 'content_mock_rewrite')
    ) NOT VALID;

ALTER TABLE review_reports
    ADD CONSTRAINT review_reports_project_item_version_id_id_unique
        UNIQUE (project_id, content_item_id, content_version_id, id);

ALTER TABLE workflow_runs
    ADD CONSTRAINT workflow_runs_target_version_same_item FOREIGN KEY (
        content_item_id,
        target_content_version_id
    ) REFERENCES content_versions(content_item_id, id) ON DELETE RESTRICT NOT VALID,
    ADD CONSTRAINT workflow_runs_source_review_same_scope FOREIGN KEY (
        project_id,
        content_item_id,
        content_version_id,
        source_review_report_id
    ) REFERENCES review_reports(project_id, content_item_id, content_version_id, id) ON DELETE RESTRICT NOT VALID,
    ADD CONSTRAINT workflow_runs_mock_rewrite_shape CHECK (
        workflow_key <> 'content_mock_rewrite' OR (
            source_review_report_id IS NOT NULL
            AND (
                (status = 'running' AND target_content_version_id IS NULL)
                OR (status = 'succeeded' AND target_content_version_id IS NOT NULL)
                OR (status = 'failed' AND target_content_version_id IS NULL AND output_json = '{}'::jsonb)
            )
        )
    ) NOT VALID;

ALTER TABLE workflow_runs
    VALIDATE CONSTRAINT workflow_runs_workflow_key_check,
    VALIDATE CONSTRAINT workflow_runs_target_version_same_item,
    VALIDATE CONSTRAINT workflow_runs_source_review_same_scope,
    VALIDATE CONSTRAINT workflow_runs_mock_rewrite_shape;

CREATE INDEX workflow_runs_content_item_workflow_started_at_id_idx
    ON workflow_runs (content_item_id, workflow_key, started_at DESC, id DESC);
