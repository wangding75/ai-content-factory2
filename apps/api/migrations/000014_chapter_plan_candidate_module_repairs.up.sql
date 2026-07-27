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
    FOREIGN KEY (base_chapter_plan_id) REFERENCES chapter_plans(id) ON DELETE SET NULL;

ALTER TABLE chapter_plan_candidates DROP CONSTRAINT IF EXISTS chapter_plan_candidates_base_revision_fk;
ALTER TABLE chapter_plan_candidates
    ADD CONSTRAINT chapter_plan_candidates_base_revision_fk
    FOREIGN KEY (base_revision_id) REFERENCES chapter_plan_revisions(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS chapter_plan_candidate_batches_source_run_idx ON chapter_plan_candidate_batches (project_id, source_workflow_run_id);
CREATE INDEX IF NOT EXISTS chapter_plan_revisions_source_run_idx ON chapter_plan_revisions (project_id, source_workflow_run_id);
CREATE INDEX IF NOT EXISTS chapter_plan_candidates_base_revision_idx ON chapter_plan_candidates (project_id, base_revision_id);
CREATE INDEX IF NOT EXISTS chapter_plans_source_run_idx ON chapter_plans (project_id, source_workflow_run_id);

CREATE UNIQUE INDEX IF NOT EXISTS workflow_run_records_active_chapter_planning_idx
    ON workflow_run_records (project_id)
    WHERE stage = 'chapter_planning' AND status IN ('queued', 'running');
