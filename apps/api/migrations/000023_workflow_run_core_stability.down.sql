DROP INDEX IF EXISTS workflow_run_records_worker_candidates_idx;

DROP INDEX IF EXISTS workflow_run_records_active_chapter_planning_idx;
CREATE UNIQUE INDEX workflow_run_records_active_chapter_planning_idx
    ON workflow_run_records (project_id)
    WHERE stage = 'chapter_planning' AND status IN ('queued', 'running');

DROP INDEX IF EXISTS workflow_run_events_run_sequence_idx;

ALTER TABLE workflow_run_events
    DROP CONSTRAINT IF EXISTS workflow_run_events_run_id_sequence_unique,
    DROP COLUMN IF EXISTS sequence;

ALTER TABLE workflow_run_records
    DROP COLUMN IF EXISTS next_event_sequence;
