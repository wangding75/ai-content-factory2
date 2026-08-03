-- Runtime integrity for restart-safe n8n execution tracking and persisted deadlines.

ALTER TABLE workflow_run_records
    ADD COLUMN workflow_connection_id UUID NULL REFERENCES workflow_connections(id) ON DELETE RESTRICT,
    ADD COLUMN deadline_at TIMESTAMPTZ NULL,
    ADD COLUMN cancellation_reason TEXT NULL;

WITH snapshot_connections AS (
    SELECT r.id, c.id AS connection_id
    FROM workflow_run_records r
    JOIN workflow_connections c
      ON c.id::text = r.configuration_snapshot #>> '{workflowConnection,id}'
    WHERE r.configuration_snapshot #>> '{workflowConnection,id}' IS NOT NULL
)
UPDATE workflow_run_records r
SET workflow_connection_id = s.connection_id
FROM snapshot_connections s
WHERE r.id = s.id;

UPDATE workflow_run_records
SET deadline_at = created_at + INTERVAL '15 minutes'
WHERE deadline_at IS NULL
  AND status IN ('queued', 'running', 'cancelling');

UPDATE workflow_run_records
SET cancellation_reason = 'user'
WHERE status = 'cancelling' AND cancellation_requested_at IS NOT NULL;

UPDATE workflow_run_records
SET cancellation_reason = 'timeout'
WHERE status = 'timed_out';

ALTER TABLE workflow_run_records
    ADD CONSTRAINT workflow_run_records_cancellation_reason_check CHECK (
        cancellation_reason IS NULL OR cancellation_reason IN ('user', 'timeout')
    ),
    ADD CONSTRAINT workflow_run_records_cancellation_shape_check CHECK (
        (status = 'cancelling' AND cancellation_reason IS NOT NULL)
        OR (status = 'cancelled' AND cancellation_reason IS DISTINCT FROM 'timeout')
        OR (status = 'timed_out' AND cancellation_reason = 'timeout')
        OR (status NOT IN ('cancelling', 'cancelled', 'timed_out') AND cancellation_reason IS NULL)
    ),
    ADD CONSTRAINT workflow_run_records_deadline_shape_check CHECK (
        deadline_at IS NULL OR deadline_at >= created_at
    );

ALTER TABLE workflow_run_records
    DROP CONSTRAINT workflow_run_records_time_shape_check,
    ADD CONSTRAINT workflow_run_records_time_shape_check CHECK (
        (status = 'queued' AND started_at IS NULL AND finished_at IS NULL AND cancelled_at IS NULL AND timed_out_at IS NULL)
        OR (status = 'running' AND started_at IS NOT NULL AND finished_at IS NULL AND cancelled_at IS NULL AND timed_out_at IS NULL)
        OR (status = 'cancelling' AND finished_at IS NULL AND cancelled_at IS NULL AND timed_out_at IS NULL)
        OR (status IN ('succeeded', 'failed') AND started_at IS NOT NULL AND finished_at IS NOT NULL AND cancelled_at IS NULL AND timed_out_at IS NULL)
        OR (status = 'cancelled' AND cancelled_at IS NOT NULL AND timed_out_at IS NULL)
        OR (status = 'timed_out' AND finished_at IS NOT NULL AND timed_out_at IS NOT NULL AND cancelled_at IS NULL)
    );

DROP INDEX workflow_run_records_connection_execution_unique_idx;
CREATE UNIQUE INDEX workflow_run_records_connection_execution_unique_idx
    ON workflow_run_records (workflow_connection_id, external_execution_id)
    WHERE workflow_connection_id IS NOT NULL AND external_execution_id IS NOT NULL;

CREATE UNIQUE INDEX workflow_run_events_one_execution_started_idx
    ON workflow_run_events (run_id) WHERE event_type = 'execution_started';

ALTER TABLE workflow_configurations
    ADD COLUMN resolved_webhook_path VARCHAR(512) NULL,
    ADD COLUMN resolved_workflow_id VARCHAR(200) NULL,
    ADD COLUMN resolved_workflow_revision VARCHAR(200) NULL,
    ADD CONSTRAINT workflow_configurations_resolution_shape_check CHECK (
        (resolved_webhook_path IS NULL AND resolved_workflow_id IS NULL AND resolved_workflow_revision IS NULL)
        OR (
            char_length(btrim(resolved_webhook_path)) BETWEEN 1 AND 512
            AND char_length(btrim(resolved_workflow_id)) BETWEEN 1 AND 200
            AND char_length(btrim(resolved_workflow_revision)) BETWEEN 1 AND 200
        )
    );
