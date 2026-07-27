-- 000014_chapter_plan_candidate_module_repairs.up.sql

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'workflow_run_records_project_id_id_unique'
    ) THEN
        ALTER TABLE workflow_run_records ADD CONSTRAINT workflow_run_records_project_id_id_unique UNIQUE (project_id, id);
    END IF;
END $$;

ALTER TABLE chapter_plan_candidate_batches DROP CONSTRAINT IF EXISTS chapter_plan_candidate_batches_source_workflow_run_id_fkey;
ALTER TABLE chapter_plan_candidate_batches DROP CONSTRAINT IF EXISTS chapter_plan_candidate_batches_source_run_fk;
ALTER TABLE chapter_plan_candidate_batches
    ADD CONSTRAINT chapter_plan_candidate_batches_source_run_fk
    FOREIGN KEY (project_id, source_workflow_run_id) REFERENCES workflow_run_records(project_id, id) ON DELETE RESTRICT;

ALTER TABLE chapter_plan_revisions DROP CONSTRAINT IF EXISTS chapter_plan_revisions_source_workflow_run_id_fkey;
ALTER TABLE chapter_plan_revisions DROP CONSTRAINT IF EXISTS chapter_plan_revisions_source_run_fk;
ALTER TABLE chapter_plan_revisions
    ADD CONSTRAINT chapter_plan_revisions_source_run_fk
    FOREIGN KEY (project_id, source_workflow_run_id) REFERENCES workflow_run_records(project_id, id) ON DELETE RESTRICT;

ALTER TABLE chapter_plans DROP CONSTRAINT IF EXISTS chapter_plans_source_workflow_run_id_fkey;
ALTER TABLE chapter_plans DROP CONSTRAINT IF EXISTS chapter_plans_source_run_fk;
ALTER TABLE chapter_plans
    ADD CONSTRAINT chapter_plans_source_run_fk
    FOREIGN KEY (project_id, source_workflow_run_id) REFERENCES workflow_run_records(project_id, id) ON DELETE RESTRICT;

ALTER TABLE chapter_plan_candidates DROP CONSTRAINT IF EXISTS chapter_plan_candidates_base_plan_fk;
ALTER TABLE chapter_plan_candidates
    ADD CONSTRAINT chapter_plan_candidates_base_plan_fk
    FOREIGN KEY (project_id, base_chapter_plan_id) REFERENCES chapter_plans(project_id, id) ON DELETE SET NULL;

ALTER TABLE chapter_plan_candidates DROP CONSTRAINT IF EXISTS chapter_plan_candidates_base_revision_fk;
ALTER TABLE chapter_plan_candidates
    ADD CONSTRAINT chapter_plan_candidates_base_revision_fk
    FOREIGN KEY (project_id, base_revision_id) REFERENCES chapter_plan_revisions(project_id, id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS chapter_plan_candidate_batches_source_run_idx ON chapter_plan_candidate_batches (project_id, source_workflow_run_id);
CREATE INDEX IF NOT EXISTS chapter_plan_revisions_source_run_idx ON chapter_plan_revisions (project_id, source_workflow_run_id);
CREATE INDEX IF NOT EXISTS chapter_plan_candidates_base_revision_idx ON chapter_plan_candidates (project_id, base_revision_id);
CREATE INDEX IF NOT EXISTS chapter_plans_source_run_idx ON chapter_plans (project_id, source_workflow_run_id);

CREATE UNIQUE INDEX IF NOT EXISTS workflow_run_records_active_chapter_planning_idx
    ON workflow_run_records (project_id)
    WHERE stage = 'chapter_planning' AND status IN ('queued', 'running');

CREATE TABLE IF NOT EXISTS chapter_plan_result_consumptions (
    workflow_run_id UUID PRIMARY KEY REFERENCES workflow_run_records(id) ON DELETE CASCADE,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    status VARCHAR(32) NOT NULL CHECK (status IN ('pending', 'consuming', 'consumed', 'output_validation_failed', 'result_consumption_failed')),
    failure_code VARCHAR(80) NULL,
    safe_reason VARCHAR(300) NULL,
    retry_action VARCHAR(80) NULL,
    candidate_batch_id UUID NULL REFERENCES chapter_plan_candidate_batches(id) ON DELETE RESTRICT,
    consumed_at TIMESTAMPTZ NULL,
    version INTEGER NOT NULL DEFAULT 1 CHECK (version >= 1),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chapter_plan_result_consumptions_project_run_unique UNIQUE (project_id, workflow_run_id)
);

CREATE INDEX IF NOT EXISTS chapter_plan_result_consumptions_project_status_idx
    ON chapter_plan_result_consumptions (project_id, status, updated_at DESC);
