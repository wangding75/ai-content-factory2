-- WorkflowRun core stability: durable event sequence, active includes cancelling for chapter planning.

ALTER TABLE workflow_run_records
    ADD COLUMN next_event_sequence BIGINT NOT NULL DEFAULT 1;

ALTER TABLE workflow_run_events
    ADD COLUMN sequence BIGINT NULL;

-- Deterministic historical backfill: business order becomes sequence; created_at remains display-only.
WITH ordered AS (
    SELECT
        id,
        ROW_NUMBER() OVER (PARTITION BY run_id ORDER BY created_at ASC, id ASC) AS seq
    FROM workflow_run_events
)
UPDATE workflow_run_events e
SET sequence = o.seq
FROM ordered o
WHERE e.id = o.id;

UPDATE workflow_run_records r
SET next_event_sequence = COALESCE(
    (SELECT MAX(e.sequence) + 1 FROM workflow_run_events e WHERE e.run_id = r.id),
    1
);

ALTER TABLE workflow_run_events
    ALTER COLUMN sequence SET NOT NULL,
    ADD CONSTRAINT workflow_run_events_run_id_sequence_unique UNIQUE (run_id, sequence);

CREATE INDEX workflow_run_events_run_sequence_idx
    ON workflow_run_events (run_id, sequence ASC);

-- Chapter planning active uniqueness must treat cancelling as Active (other stages already do in 019).
DROP INDEX IF EXISTS workflow_run_records_active_chapter_planning_idx;
CREATE UNIQUE INDEX workflow_run_records_active_chapter_planning_idx
    ON workflow_run_records (project_id)
    WHERE stage = 'chapter_planning' AND status IN ('queued', 'running', 'cancelling');

CREATE INDEX IF NOT EXISTS workflow_run_records_worker_candidates_idx
    ON workflow_run_records (status, created_at ASC, id ASC)
    WHERE status IN ('queued', 'running', 'cancelling');
