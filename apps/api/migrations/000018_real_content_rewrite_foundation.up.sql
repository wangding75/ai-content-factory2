ALTER TABLE content_versions
    DROP CONSTRAINT content_versions_source_check;

ALTER TABLE content_versions
    ADD CONSTRAINT content_versions_source_check CHECK (
        source IN (
            'manual_created',
            'mock_generated',
            'mock_rewrite',
            'workflow_generated',
            'workflow_rewrite'
        )
    ) NOT VALID,
    ADD CONSTRAINT content_versions_workflow_rewrite_shape CHECK (
        source <> 'workflow_rewrite'
        OR (
            status = 'editable_draft'
            AND version = 1
            AND frozen_at IS NULL
            AND source_content_version_id IS NOT NULL
            AND source_content_version_id <> id
            AND source_content_version_version IS NOT NULL
            AND source_content_version_version >= 1
            AND source_workflow_run_id IS NOT NULL
        )
    ) NOT VALID;

ALTER TABLE content_versions
    VALIDATE CONSTRAINT content_versions_source_check,
    VALIDATE CONSTRAINT content_versions_workflow_rewrite_shape;

ALTER TABLE workflow_run_records
    ADD CONSTRAINT workflow_run_records_rewrite_subject_shape_check CHECK (
        stage <> 'rewrite'
        OR (
            subject_type = 'review_report'
            AND subject_id IS NOT NULL
        )
    ) NOT VALID;

ALTER TABLE workflow_run_records
    VALIDATE CONSTRAINT workflow_run_records_rewrite_subject_shape_check;

CREATE UNIQUE INDEX workflow_run_records_active_rewrite_subject_idx
    ON workflow_run_records (project_id, stage, subject_type, subject_id)
    WHERE stage = 'rewrite'
      AND status IN ('queued', 'running')
      AND subject_type = 'review_report'
      AND subject_id IS NOT NULL;

CREATE OR REPLACE FUNCTION enforce_rewrite_workflow_run_scope()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    report_project_id UUID;
    report_content_item_id UUID;
    report_content_version_id UUID;
    report_provider_key TEXT;
    report_status TEXT;
    report_schema_version VARCHAR(40);
    selected_count INTEGER;
    selected_unique_count INTEGER;
BEGIN
    IF NEW.stage <> 'rewrite' THEN
        RETURN NEW;
    END IF;

    SELECT
        project_id,
        content_item_id,
        content_version_id,
        provider_key,
        status,
        schema_version
    INTO
        report_project_id,
        report_content_item_id,
        report_content_version_id,
        report_provider_key,
        report_status,
        report_schema_version
    FROM review_reports
    WHERE id = NEW.subject_id;

    IF NOT FOUND
        OR report_project_id <> NEW.project_id
        OR report_provider_key <> 'runtime'
        OR report_status <> 'completed'
        OR report_schema_version <> 'review.output.v1'
    THEN
        RAISE EXCEPTION 'Rewrite WorkflowRun must reference a completed Runtime ReviewReport in the same Project'
            USING ERRCODE = '23503';
    END IF;

    IF NEW.input_payload ->> 'schemaVersion' <> 'rewrite.input.v1'
        OR NEW.input_payload ->> 'projectId' <> NEW.project_id::TEXT
        OR NEW.input_payload ->> 'contentItemId' <> report_content_item_id::TEXT
        OR NEW.input_payload ->> 'sourceContentVersionId' <> report_content_version_id::TEXT
        OR NEW.input_payload ->> 'reviewReportId' <> NEW.subject_id::TEXT
        OR jsonb_typeof(NEW.input_payload -> 'selectedIssues') <> 'array'
        OR NEW.configuration_snapshot ->> 'stage' <> 'rewrite'
        OR NEW.configuration_snapshot #>> '{workflowConfiguration,inputContractVersion}' <> 'rewrite.input.v1'
        OR NEW.configuration_snapshot #>> '{workflowConfiguration,outputContractVersion}' <> 'rewrite.output.v1'
    THEN
        RAISE EXCEPTION 'Rewrite WorkflowRun input or configuration snapshot is outside the frozen Rewrite contract'
            USING ERRCODE = '23514';
    END IF;

    SELECT
        COUNT(*),
        COUNT(DISTINCT selected_issue ->> 'reviewIssueId')
    INTO selected_count, selected_unique_count
    FROM jsonb_array_elements(NEW.input_payload -> 'selectedIssues') AS selected(selected_issue);

    IF selected_count NOT BETWEEN 1 AND 50
        OR selected_unique_count <> selected_count
    THEN
        RAISE EXCEPTION 'Rewrite WorkflowRun must snapshot 1 to 50 unique ReviewIssue IDs'
            USING ERRCODE = '23514';
    END IF;

    IF EXISTS (
        SELECT 1
        FROM jsonb_array_elements(NEW.input_payload -> 'selectedIssues') AS selected(selected_issue)
        LEFT JOIN review_findings issue
            ON issue.id::TEXT = selected.selected_issue ->> 'reviewIssueId'
           AND issue.review_id = NEW.subject_id
        WHERE jsonb_typeof(selected.selected_issue) <> 'object'
           OR selected.selected_issue ->> 'reviewReportId' <> NEW.subject_id::TEXT
           OR selected.selected_issue ->> 'disposition' <> 'open'
           OR issue.id IS NULL
    ) THEN
        RAISE EXCEPTION 'Rewrite WorkflowRun selected ReviewIssues must belong to the source ReviewReport'
            USING ERRCODE = '23503';
    END IF;

    RETURN NEW;
END;
$$;

CREATE CONSTRAINT TRIGGER workflow_run_records_rewrite_scope_trigger
    AFTER INSERT OR UPDATE OF project_id, stage, subject_type, subject_id, input_payload, configuration_snapshot
    ON workflow_run_records
    DEFERRABLE INITIALLY DEFERRED
    FOR EACH ROW
    EXECUTE FUNCTION enforce_rewrite_workflow_run_scope();

CREATE OR REPLACE FUNCTION enforce_workflow_rewrite_candidate_scope()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    candidate_scope_valid BOOLEAN;
BEGIN
    IF NEW.source <> 'workflow_rewrite' THEN
        RETURN NEW;
    END IF;

    SELECT EXISTS (
        SELECT 1
        FROM workflow_run_records run
        JOIN review_reports report
            ON report.id = run.subject_id
        JOIN content_items item
            ON item.id = NEW.content_item_id
        JOIN content_versions source_version
            ON source_version.id = NEW.source_content_version_id
           AND source_version.content_item_id = NEW.content_item_id
        WHERE run.id = NEW.source_workflow_run_id
          AND run.project_id = item.project_id
          AND run.stage = 'rewrite'
          AND run.status = 'succeeded'
          AND run.subject_type = 'review_report'
          AND report.project_id = item.project_id
          AND report.content_item_id = NEW.content_item_id
          AND report.content_version_id = NEW.source_content_version_id
          AND report.provider_key = 'runtime'
          AND report.status = 'completed'
          AND report.schema_version = 'review.output.v1'
          AND source_version.version = NEW.source_content_version_version
          AND run.input_payload ->> 'sourceContentVersionId' = NEW.source_content_version_id::TEXT
          AND run.input_payload ->> 'reviewReportId' = report.id::TEXT
    ) INTO candidate_scope_valid;

    IF NOT candidate_scope_valid THEN
        RAISE EXCEPTION 'workflow_rewrite ContentVersion source Run, ReviewReport or source ContentVersion is outside the Candidate scope'
            USING ERRCODE = '23503';
    END IF;

    RETURN NEW;
END;
$$;

CREATE CONSTRAINT TRIGGER content_versions_workflow_rewrite_scope_trigger
    AFTER INSERT OR UPDATE OF
        content_item_id,
        source,
        source_content_version_id,
        source_content_version_version,
        source_workflow_run_id
    ON content_versions
    DEFERRABLE INITIALLY DEFERRED
    FOR EACH ROW
    EXECUTE FUNCTION enforce_workflow_rewrite_candidate_scope();
