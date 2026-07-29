CREATE UNIQUE INDEX workflow_run_records_active_review_subject_idx
    ON workflow_run_records (project_id, stage, subject_type, subject_id)
    WHERE stage = 'review'
      AND status IN ('queued', 'running')
      AND subject_type = 'content_version'
      AND subject_id IS NOT NULL;

ALTER TABLE review_reports
    DROP CONSTRAINT review_reports_run_same_scope,
    DROP CONSTRAINT review_reports_provider_key_check,
    DROP CONSTRAINT review_reports_conclusion_check,
    ALTER COLUMN workflow_run_id DROP NOT NULL,
    ALTER COLUMN score DROP NOT NULL,
    ADD COLUMN schema_version VARCHAR(40) NULL,
    ADD COLUMN source_content_version_version INTEGER NULL,
    ADD COLUMN source_content_hash CHAR(64) NULL,
    ADD CONSTRAINT review_reports_provider_key_check CHECK (
        provider_key IN ('mock', 'runtime')
    ),
    ADD CONSTRAINT review_reports_conclusion_check CHECK (
        (provider_key = 'mock' AND conclusion IN ('pass', 'revise'))
        OR
        (provider_key = 'runtime' AND conclusion IN ('passed', 'needs_changes'))
    ),
    ADD CONSTRAINT review_reports_runtime_shape_check CHECK (
        provider_key <> 'runtime'
        OR (
            workflow_run_id IS NOT NULL
            AND schema_version = 'review.output.v1'
            AND source_content_version_version >= 1
            AND source_content_hash ~ '^[0-9a-f]{64}$'
            AND score IS NULL
        )
    );

CREATE OR REPLACE FUNCTION enforce_review_report_workflow_run_scope()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW.workflow_run_id IS NULL THEN
        IF NEW.provider_key = 'runtime' THEN
            RAISE EXCEPTION 'runtime ReviewReport requires workflow_run_id'
                USING ERRCODE = '23503';
        END IF;
        RETURN NEW;
    END IF;

    IF NEW.provider_key = 'mock' THEN
        IF NOT EXISTS (
            SELECT 1
            FROM workflow_runs wr
            WHERE wr.id = NEW.workflow_run_id
              AND wr.project_id = NEW.project_id
              AND wr.content_item_id = NEW.content_item_id
              AND wr.content_version_id = NEW.content_version_id
              AND wr.workflow_key = 'content_mock_review'
        ) THEN
            RAISE EXCEPTION 'P0 ReviewReport workflow_run_id is outside the report scope'
                USING ERRCODE = '23503';
        END IF;
        RETURN NEW;
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM workflow_run_records wr
        WHERE wr.id = NEW.workflow_run_id
          AND wr.project_id = NEW.project_id
          AND wr.stage = 'review'
          AND wr.status = 'succeeded'
          AND wr.subject_type = 'content_version'
          AND wr.subject_id = NEW.content_version_id
    ) THEN
        RAISE EXCEPTION 'Runtime ReviewReport workflow_run_id is not a succeeded review Run for the source ContentVersion'
            USING ERRCODE = '23503';
    END IF;

    RETURN NEW;
END;
$$;

CREATE CONSTRAINT TRIGGER review_reports_workflow_run_scope_trigger
    AFTER INSERT OR UPDATE OF workflow_run_id, provider_key, project_id, content_item_id, content_version_id
    ON review_reports
    DEFERRABLE INITIALLY DEFERRED
    FOR EACH ROW
    EXECUTE FUNCTION enforce_review_report_workflow_run_scope();

ALTER TABLE review_findings
    DROP CONSTRAINT review_findings_category_check,
    DROP CONSTRAINT review_findings_severity_check,
    DROP CONSTRAINT review_findings_sort_order_check,
    ADD COLUMN issue_key VARCHAR(120) NULL,
    ADD COLUMN category_label VARCHAR(120) NULL,
    ADD COLUMN evidence_json JSONB NULL,
    ADD COLUMN suggestion TEXT NULL,
    ADD COLUMN disposition VARCHAR(20) NOT NULL DEFAULT 'open',
    ADD COLUMN version INTEGER NOT NULL DEFAULT 1,
    ADD COLUMN ignored_at TIMESTAMPTZ NULL,
    ADD COLUMN ignored_by VARCHAR(160) NULL,
    ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ADD CONSTRAINT review_findings_category_check CHECK (
        category ~ '^[a-z][a-z0-9_]{0,79}$'
    ),
    ADD CONSTRAINT review_findings_severity_check CHECK (
        severity IN ('low', 'medium', 'high', 'critical', 'warning', 'suggestion')
    ),
    ADD CONSTRAINT review_findings_position_check CHECK (
        (issue_key IS NULL AND sort_order >= 0)
        OR
        (issue_key IS NOT NULL AND sort_order >= 1)
    ),
    ADD CONSTRAINT review_findings_issue_key_check CHECK (
        issue_key IS NULL
        OR (
            char_length(issue_key) BETWEEN 1 AND 120
            AND issue_key ~ '^[A-Za-z0-9][A-Za-z0-9._:-]*$'
        )
    ),
    ADD CONSTRAINT review_findings_category_label_check CHECK (
        category_label IS NULL
        OR char_length(btrim(category_label)) BETWEEN 1 AND 120
    ),
    ADD CONSTRAINT review_findings_evidence_json_check CHECK (
        evidence_json IS NULL OR jsonb_typeof(evidence_json) = 'object'
    ),
    ADD CONSTRAINT review_findings_suggestion_check CHECK (
        suggestion IS NULL OR char_length(suggestion) <= 5000
    ),
    ADD CONSTRAINT review_findings_disposition_check CHECK (
        disposition IN ('open', 'ignored')
    ),
    ADD CONSTRAINT review_findings_version_check CHECK (
        version >= 1
    ),
    ADD CONSTRAINT review_findings_ignored_shape_check CHECK (
        (disposition = 'open' AND ignored_at IS NULL AND ignored_by IS NULL)
        OR
        (disposition = 'ignored' AND ignored_at IS NOT NULL AND ignored_by IS NOT NULL)
    ),
    ADD CONSTRAINT review_findings_runtime_shape_check CHECK (
        issue_key IS NULL
        OR (
            category_label IS NOT NULL
            AND severity IN ('critical', 'warning', 'suggestion')
            AND evidence_json IS NOT NULL
        )
    ),
    ADD CONSTRAINT review_findings_review_issue_key_unique UNIQUE (review_id, issue_key);

CREATE INDEX review_findings_review_severity_disposition_position_idx
    ON review_findings (review_id, severity, disposition, sort_order);
