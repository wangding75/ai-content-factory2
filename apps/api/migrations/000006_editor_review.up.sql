ALTER TABLE chapter_plans
    ADD CONSTRAINT chapter_plans_project_id_id_unique UNIQUE (project_id, id);

CREATE TABLE content_items (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL,
    chapter_plan_id UUID NOT NULL,
    title VARCHAR(120) NOT NULL CHECK (char_length(btrim(title)) BETWEEN 1 AND 120),
    status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'in_review', 'reviewed')),
    current_version_id UUID NOT NULL,
    version INTEGER NOT NULL DEFAULT 1 CHECK (version >= 1),
    reviewed_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT content_items_chapter_plan_unique UNIQUE (chapter_plan_id),
    CONSTRAINT content_items_project_id_id_unique UNIQUE (project_id, id),
    CONSTRAINT content_items_chapter_plan_same_project FOREIGN KEY (project_id, chapter_plan_id)
        REFERENCES chapter_plans(project_id, id) ON DELETE CASCADE
);

CREATE TABLE content_versions (
    id UUID PRIMARY KEY,
    content_item_id UUID NOT NULL REFERENCES content_items(id) ON DELETE CASCADE,
    version_no INTEGER NOT NULL CHECK (version_no >= 1),
    title VARCHAR(120) NOT NULL CHECK (char_length(btrim(title)) BETWEEN 1 AND 120),
    content TEXT NOT NULL DEFAULT '' CHECK (char_length(content) <= 200000),
    summary TEXT NULL CHECK (summary IS NULL OR char_length(summary) <= 5000),
    word_count INTEGER NOT NULL DEFAULT 0 CHECK (word_count >= 0),
    source TEXT NOT NULL CHECK (source IN ('manual_created', 'mock_generated')),
    status TEXT NOT NULL DEFAULT 'editable_draft' CHECK (status IN ('editable_draft', 'frozen')),
    generation_parameters JSONB NOT NULL DEFAULT '{}'::jsonb
        CHECK (jsonb_typeof(generation_parameters) = 'object'),
    version INTEGER NOT NULL DEFAULT 1 CHECK (version >= 1),
    frozen_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT content_versions_item_version_no_unique UNIQUE (content_item_id, version_no),
    CONSTRAINT content_versions_item_id_id_unique UNIQUE (content_item_id, id),
    CONSTRAINT content_versions_freeze_shape CHECK (
        (status = 'editable_draft' AND frozen_at IS NULL) OR
        (status = 'frozen' AND frozen_at IS NOT NULL)
    )
);

ALTER TABLE content_items
    ADD CONSTRAINT content_items_current_version_same_item FOREIGN KEY (id, current_version_id)
        REFERENCES content_versions(content_item_id, id) DEFERRABLE INITIALLY DEFERRED;

CREATE TABLE workflow_runs (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL,
    content_item_id UUID NOT NULL,
    content_version_id UUID NOT NULL,
    provider_key TEXT NOT NULL CHECK (provider_key = 'mock'),
    workflow_key TEXT NOT NULL CHECK (workflow_key IN ('content_mock_generate', 'content_mock_review')),
    subject_type TEXT NOT NULL CHECK (subject_type = 'content_item'),
    subject_id UUID NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('running', 'succeeded', 'failed')),
    idempotency_key VARCHAR(128) NOT NULL CHECK (char_length(btrim(idempotency_key)) BETWEEN 1 AND 128),
    request_fingerprint CHAR(64) NOT NULL CHECK (request_fingerprint ~ '^[0-9a-f]{64}$'),
    input_json JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(input_json) = 'object'),
    output_json JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(output_json) = 'object'),
    error_code TEXT NULL CHECK (error_code IS NULL OR char_length(btrim(error_code)) BETWEEN 1 AND 120),
    error_summary TEXT NULL CHECK (error_summary IS NULL OR char_length(error_summary) <= 5000),
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT workflow_runs_project_item_same_project FOREIGN KEY (project_id, content_item_id)
        REFERENCES content_items(project_id, id) ON DELETE CASCADE,
    CONSTRAINT workflow_runs_item_version_same_item FOREIGN KEY (content_item_id, content_version_id)
        REFERENCES content_versions(content_item_id, id) ON DELETE CASCADE,
    CONSTRAINT workflow_runs_subject_is_content_item CHECK (subject_id = content_item_id),
    CONSTRAINT workflow_runs_status_time_shape CHECK (
        (status = 'running' AND finished_at IS NULL) OR
        (status IN ('succeeded', 'failed') AND finished_at IS NOT NULL)
    ),
    CONSTRAINT workflow_runs_success_error_shape CHECK (
        status <> 'succeeded' OR (error_code IS NULL AND error_summary IS NULL)
    ),
    CONSTRAINT workflow_runs_scope_idempotency_key_unique UNIQUE (project_id, content_item_id, workflow_key, idempotency_key),
    CONSTRAINT workflow_runs_project_item_version_id_unique UNIQUE (project_id, content_item_id, content_version_id, id)
);

CREATE TABLE review_reports (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL,
    content_item_id UUID NOT NULL,
    content_version_id UUID NOT NULL,
    workflow_run_id UUID NOT NULL,
    provider_key TEXT NOT NULL CHECK (provider_key = 'mock'),
    status TEXT NOT NULL DEFAULT 'completed' CHECK (status = 'completed'),
    conclusion TEXT NOT NULL CHECK (conclusion IN ('pass', 'revise')),
    score INTEGER NOT NULL CHECK (score BETWEEN 0 AND 100),
    summary TEXT NOT NULL DEFAULT '' CHECK (char_length(summary) <= 5000),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT review_reports_project_item_same_project FOREIGN KEY (project_id, content_item_id)
        REFERENCES content_items(project_id, id) ON DELETE CASCADE,
    CONSTRAINT review_reports_item_version_same_item FOREIGN KEY (content_item_id, content_version_id)
        REFERENCES content_versions(content_item_id, id) ON DELETE CASCADE,
    CONSTRAINT review_reports_run_same_scope FOREIGN KEY (project_id, content_item_id, content_version_id, workflow_run_id)
        REFERENCES workflow_runs(project_id, content_item_id, content_version_id, id) ON DELETE CASCADE,
    CONSTRAINT review_reports_workflow_run_unique UNIQUE (workflow_run_id)
);

CREATE TABLE review_findings (
    id UUID PRIMARY KEY,
    review_id UUID NOT NULL REFERENCES review_reports(id) ON DELETE CASCADE,
    category TEXT NOT NULL CHECK (category IN ('pacing', 'foreshadowing', 'character_consistency', 'world_consistency')),
    severity TEXT NOT NULL CHECK (severity IN ('low', 'medium', 'high')),
    title VARCHAR(200) NOT NULL CHECK (char_length(btrim(title)) BETWEEN 1 AND 200),
    description TEXT NOT NULL DEFAULT '' CHECK (char_length(description) <= 5000),
    location_json JSONB NULL CHECK (location_json IS NULL OR jsonb_typeof(location_json) = 'object'),
    sort_order INTEGER NOT NULL CHECK (sort_order >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT review_findings_report_sort_order_unique UNIQUE (review_id, sort_order)
);

CREATE TABLE review_recommendations (
    id UUID PRIMARY KEY,
    review_id UUID NOT NULL REFERENCES review_reports(id) ON DELETE CASCADE,
    priority TEXT NOT NULL CHECK (priority IN ('low', 'medium', 'high')),
    title VARCHAR(200) NOT NULL CHECK (char_length(btrim(title)) BETWEEN 1 AND 200),
    description TEXT NOT NULL DEFAULT '' CHECK (char_length(description) <= 5000),
    sort_order INTEGER NOT NULL CHECK (sort_order >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT review_recommendations_report_sort_order_unique UNIQUE (review_id, sort_order)
);

CREATE INDEX content_items_project_id_idx ON content_items (project_id);
CREATE INDEX content_versions_content_item_version_no_idx ON content_versions (content_item_id, version_no);
CREATE INDEX workflow_runs_content_item_operation_created_at_idx ON workflow_runs (content_item_id, workflow_key, created_at DESC, id DESC);
CREATE INDEX review_reports_content_item_created_at_id_idx ON review_reports (content_item_id, created_at DESC, id DESC);
CREATE INDEX review_findings_review_sort_order_id_idx ON review_findings (review_id, sort_order, id);
CREATE INDEX review_recommendations_review_sort_order_id_idx ON review_recommendations (review_id, sort_order, id);
