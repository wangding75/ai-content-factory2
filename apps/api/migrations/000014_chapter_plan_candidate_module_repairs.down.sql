-- 000014_chapter_plan_candidate_module_repairs.down.sql

DROP INDEX IF EXISTS workflow_run_records_active_chapter_planning_idx;
DROP INDEX IF EXISTS chapter_plan_result_consumptions_project_status_idx;
DROP TABLE IF EXISTS chapter_plan_result_consumptions;
DROP INDEX IF EXISTS chapter_plans_source_run_idx;
DROP INDEX IF EXISTS chapter_plan_candidates_base_revision_idx;
DROP INDEX IF EXISTS chapter_plan_revisions_source_run_idx;
DROP INDEX IF EXISTS chapter_plan_candidate_batches_source_run_idx;

ALTER TABLE chapter_plan_candidates DROP CONSTRAINT IF EXISTS chapter_plan_candidates_base_revision_fk;
ALTER TABLE chapter_plan_candidates
    ADD CONSTRAINT chapter_plan_candidates_base_revision_fk
    FOREIGN KEY (base_revision_id) REFERENCES chapter_plan_revisions(id) ON DELETE SET NULL;

ALTER TABLE chapter_plan_candidates DROP CONSTRAINT IF EXISTS chapter_plan_candidates_base_plan_fk;
ALTER TABLE chapter_plan_candidates
    ADD CONSTRAINT chapter_plan_candidates_base_plan_fk
    FOREIGN KEY (project_id, base_chapter_plan_id) REFERENCES chapter_plans(project_id, id) ON DELETE SET NULL;

ALTER TABLE chapter_plans DROP CONSTRAINT IF EXISTS chapter_plans_source_run_fk;
ALTER TABLE chapter_plans
    ADD CONSTRAINT chapter_plans_source_workflow_run_id_fkey
    FOREIGN KEY (source_workflow_run_id) REFERENCES workflow_run_records(id) ON DELETE RESTRICT;

ALTER TABLE chapter_plan_revisions DROP CONSTRAINT IF EXISTS chapter_plan_revisions_source_run_fk;
ALTER TABLE chapter_plan_revisions
    ADD CONSTRAINT chapter_plan_revisions_source_workflow_run_id_fkey
    FOREIGN KEY (source_workflow_run_id) REFERENCES workflow_run_records(id) ON DELETE RESTRICT;

ALTER TABLE chapter_plan_candidate_batches DROP CONSTRAINT IF EXISTS chapter_plan_candidate_batches_source_run_fk;
ALTER TABLE chapter_plan_candidate_batches
    ADD CONSTRAINT chapter_plan_candidate_batches_source_workflow_run_id_fkey
    FOREIGN KEY (source_workflow_run_id) REFERENCES workflow_run_records(id) ON DELETE RESTRICT;

ALTER TABLE workflow_run_records DROP CONSTRAINT IF EXISTS workflow_run_records_project_id_id_unique;
