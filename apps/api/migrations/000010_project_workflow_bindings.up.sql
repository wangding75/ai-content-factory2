CREATE TABLE project_workflow_bindings (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL,
    stage TEXT NOT NULL,
    workflow_configuration_id UUID NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT project_workflow_bindings_project_id_stage_key UNIQUE(project_id, stage),
    CONSTRAINT project_workflow_bindings_stage_check CHECK (
        stage IN ('chapter_planning', 'content_generation', 'review', 'rewrite')
    ),
    CONSTRAINT project_workflow_bindings_version_check CHECK (version >= 1),
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    FOREIGN KEY (workflow_configuration_id) REFERENCES workflow_configurations(id)
);
CREATE INDEX project_workflow_bindings_workflow_configuration_id_idx ON project_workflow_bindings(workflow_configuration_id);