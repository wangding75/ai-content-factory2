CREATE TABLE chapter_plan_candidate_batches (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    source_workflow_run_id UUID NOT NULL REFERENCES workflow_run_records(id) ON DELETE RESTRICT,
    generation_mode VARCHAR(32) NOT NULL CHECK (generation_mode IN ('full', 'append', 'range')),
    range_start INTEGER NULL CHECK (range_start IS NULL OR range_start >= 1),
    range_end INTEGER NULL CHECK (range_end IS NULL OR range_end >= 1),
    requested_chapter_count INTEGER NOT NULL CHECK (requested_chapter_count >= 1),
    input_digest CHAR(64) NOT NULL CHECK (input_digest ~ '^[a-f0-9]{64}$'),
    input_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    storyline_selection_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    context_options JSONB NOT NULL DEFAULT '{}'::jsonb,
    additional_instructions TEXT NULL,
    workflow_binding_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    status VARCHAR(32) NOT NULL DEFAULT 'ready' CHECK (status IN ('ready', 'partially_adopted', 'adopted', 'abandoned')),
    candidate_count INTEGER NOT NULL DEFAULT 0 CHECK (candidate_count >= 0),
    pending_count INTEGER NOT NULL DEFAULT 0 CHECK (pending_count >= 0),
    stale_count INTEGER NOT NULL DEFAULT 0 CHECK (stale_count >= 0),
    adopted_count INTEGER NOT NULL DEFAULT 0 CHECK (adopted_count >= 0),
    discarded_count INTEGER NOT NULL DEFAULT 0 CHECK (discarded_count >= 0),
    completed_at TIMESTAMPTZ NULL,
    abandoned_at TIMESTAMPTZ NULL,
    abandon_reason TEXT NULL,
    created_by VARCHAR(128) NOT NULL DEFAULT 'system',
    updated_by VARCHAR(128) NOT NULL DEFAULT 'system',
    version INTEGER NOT NULL DEFAULT 1 CHECK (version >= 1),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chapter_plan_candidate_batches_source_run_unique UNIQUE (source_workflow_run_id),
    CONSTRAINT chapter_plan_candidate_batches_project_id_id_unique UNIQUE (project_id, id),
    CONSTRAINT chapter_plan_candidate_batches_input_snapshot_check CHECK (jsonb_typeof(input_snapshot) = 'object'),
    CONSTRAINT chapter_plan_candidate_batches_storyline_snapshot_check CHECK (jsonb_typeof(storyline_selection_snapshot) = 'object'),
    CONSTRAINT chapter_plan_candidate_batches_context_options_check CHECK (jsonb_typeof(context_options) = 'object'),
    CONSTRAINT chapter_plan_candidate_batches_binding_snapshot_check CHECK (jsonb_typeof(workflow_binding_snapshot) = 'object')
);

CREATE INDEX chapter_plan_candidate_batches_project_created_idx ON chapter_plan_candidate_batches (project_id, created_at DESC, id DESC);
CREATE INDEX chapter_plan_candidate_batches_project_status_created_idx ON chapter_plan_candidate_batches (project_id, status, created_at DESC, id DESC);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'chapter_plans_project_id_id_unique'
    ) THEN
        ALTER TABLE chapter_plans ADD CONSTRAINT chapter_plans_project_id_id_unique UNIQUE (project_id, id);
    END IF;
END $$;

CREATE TABLE chapter_plan_candidates (
    id UUID PRIMARY KEY,
    batch_id UUID NOT NULL,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    chapter_no INTEGER NOT NULL CHECK (chapter_no >= 1),
    sort_order INTEGER NOT NULL CHECK (sort_order >= 0),
    base_chapter_plan_id UUID NULL,
    base_revision_id UUID NULL,
    base_chapter_version INTEGER NULL CHECK (base_chapter_version IS NULL OR base_chapter_version >= 1),
    base_snapshot JSONB NULL,
    generated_snapshot JSONB NOT NULL,
    current_snapshot JSONB NOT NULL,
    diff_type VARCHAR(32) NOT NULL DEFAULT 'new' CHECK (diff_type IN ('new', 'replace', 'no_change', 'stale_conflict')),
    status VARCHAR(32) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'stale', 'adopted', 'discarded')),
    adopted_chapter_plan_id UUID NULL,
    adopted_revision_id UUID NULL,
    adopted_at TIMESTAMPTZ NULL,
    discarded_at TIMESTAMPTZ NULL,
    discard_reason TEXT NULL,
    created_by VARCHAR(128) NOT NULL DEFAULT 'system',
    updated_by VARCHAR(128) NOT NULL DEFAULT 'system',
    last_edited_by VARCHAR(128) NULL,
    last_edited_at TIMESTAMPTZ NULL,
    version INTEGER NOT NULL DEFAULT 1 CHECK (version >= 1),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chapter_plan_candidates_batch_fk FOREIGN KEY (project_id, batch_id) REFERENCES chapter_plan_candidate_batches(project_id, id) ON DELETE CASCADE,
    CONSTRAINT chapter_plan_candidates_base_plan_fk FOREIGN KEY (project_id, base_chapter_plan_id) REFERENCES chapter_plans(project_id, id) ON DELETE SET NULL,
    CONSTRAINT chapter_plan_candidates_batch_chapter_no_unique UNIQUE (batch_id, chapter_no),
    CONSTRAINT chapter_plan_candidates_batch_sort_order_unique UNIQUE (batch_id, sort_order),
    CONSTRAINT chapter_plan_candidates_project_id_id_unique UNIQUE (project_id, id),
    CONSTRAINT chapter_plan_candidates_base_snapshot_check CHECK (base_snapshot IS NULL OR jsonb_typeof(base_snapshot) = 'object'),
    CONSTRAINT chapter_plan_candidates_generated_snapshot_check CHECK (jsonb_typeof(generated_snapshot) = 'object'),
    CONSTRAINT chapter_plan_candidates_current_snapshot_check CHECK (jsonb_typeof(current_snapshot) = 'object')
);

CREATE INDEX chapter_plan_candidates_batch_status_idx ON chapter_plan_candidates (batch_id, status, chapter_no, id);
CREATE INDEX chapter_plan_candidates_project_base_plan_idx ON chapter_plan_candidates (project_id, base_chapter_plan_id, base_chapter_version);

CREATE TABLE chapter_plan_revisions (
    id UUID PRIMARY KEY,
    chapter_plan_id UUID NOT NULL,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    revision_no INTEGER NOT NULL CHECK (revision_no >= 1),
    snapshot JSONB NOT NULL,
    change_type VARCHAR(32) NOT NULL CHECK (change_type IN ('manual_create', 'manual_edit', 'candidate_adopt', 'confirm', 'legacy_backfill')),
    source_candidate_id UUID NULL,
    source_candidate_batch_id UUID NULL,
    source_workflow_run_id UUID NULL REFERENCES workflow_run_records(id) ON DELETE RESTRICT,
    created_by VARCHAR(128) NOT NULL DEFAULT 'system',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chapter_plan_revisions_chapter_plan_fk FOREIGN KEY (project_id, chapter_plan_id) REFERENCES chapter_plans(project_id, id) ON DELETE CASCADE,
    CONSTRAINT chapter_plan_revisions_source_candidate_fk FOREIGN KEY (project_id, source_candidate_id) REFERENCES chapter_plan_candidates(project_id, id) ON DELETE RESTRICT,
    CONSTRAINT chapter_plan_revisions_source_batch_fk FOREIGN KEY (project_id, source_candidate_batch_id) REFERENCES chapter_plan_candidate_batches(project_id, id) ON DELETE RESTRICT,
    CONSTRAINT chapter_plan_revisions_chapter_plan_no_unique UNIQUE (chapter_plan_id, revision_no),
    CONSTRAINT chapter_plan_revisions_project_id_id_unique UNIQUE (project_id, id),
    CONSTRAINT chapter_plan_revisions_snapshot_check CHECK (jsonb_typeof(snapshot) = 'object')
);

CREATE INDEX chapter_plan_revisions_history_idx ON chapter_plan_revisions (chapter_plan_id, revision_no DESC);

ALTER TABLE chapter_plan_candidates
    ADD CONSTRAINT chapter_plan_candidates_adopted_plan_fk FOREIGN KEY (project_id, adopted_chapter_plan_id) REFERENCES chapter_plans(project_id, id) ON DELETE RESTRICT,
    ADD CONSTRAINT chapter_plan_candidates_adopted_revision_fk FOREIGN KEY (project_id, adopted_revision_id) REFERENCES chapter_plan_revisions(project_id, id) ON DELETE RESTRICT;

ALTER TABLE chapter_plans
    ADD COLUMN current_revision_id UUID NULL,
    ADD COLUMN source_candidate_id UUID NULL,
    ADD COLUMN source_candidate_batch_id UUID NULL,
    ADD COLUMN source_workflow_run_id UUID NULL REFERENCES workflow_run_records(id) ON DELETE RESTRICT;

ALTER TABLE chapter_plans
    ADD CONSTRAINT chapter_plans_current_revision_fk FOREIGN KEY (project_id, current_revision_id) REFERENCES chapter_plan_revisions(project_id, id) ON DELETE RESTRICT,
    ADD CONSTRAINT chapter_plans_source_candidate_fk FOREIGN KEY (project_id, source_candidate_id) REFERENCES chapter_plan_candidates(project_id, id) ON DELETE RESTRICT,
    ADD CONSTRAINT chapter_plans_source_batch_fk FOREIGN KEY (project_id, source_candidate_batch_id) REFERENCES chapter_plan_candidate_batches(project_id, id) ON DELETE RESTRICT;

ALTER TABLE chapter_plans DROP CONSTRAINT IF EXISTS chapter_plans_source_check;
ALTER TABLE chapter_plans ADD CONSTRAINT chapter_plans_source_check CHECK (source IN ('manual', 'mock_generated', 'candidate_adopted'));

DO $$
DECLARE
    cp RECORD;
    rev_id UUID;
    snap JSONB;
    st_json JSONB;
    mat_json JSONB;
    fore_json JSONB;
BEGIN
    FOR cp IN SELECT * FROM chapter_plans LOOP
        IF cp.current_revision_id IS NULL THEN
            SELECT jsonb_agg(jsonb_build_object(
                'id', st.storyline_id,
                'label', s.name,
                'relation', st.relation,
                'position', st.position,
                'version', s.version
            ))
            INTO st_json
            FROM chapter_plan_storylines st
            JOIN storylines s ON s.id = st.storyline_id WHERE st.chapter_plan_id = cp.id;

            SELECT jsonb_agg(jsonb_build_object(
                'id', pm.material_id,
                'label', m.name,
                'relation', 'material_ref',
                'position', pm.position,
                'version', m.version
            ))
            INTO mat_json
            FROM chapter_plan_materials pm
            JOIN materials m ON m.id = pm.material_id WHERE pm.chapter_plan_id = cp.id;

            SELECT jsonb_agg(jsonb_build_object(
                'id', pf.foreshadowing_id,
                'label', f.title,
                'relation', 'foreshadowing_ref',
                'position', pf.position,
                'version', f.version
            ))
            INTO fore_json
            FROM chapter_plan_foreshadowings pf
            JOIN foreshadowings f ON f.id = pf.foreshadowing_id WHERE pf.chapter_plan_id = cp.id;

            snap := jsonb_build_object(
                'chapterNo', cp.chapter_no,
                'title', cp.title,
                'summary', cp.summary,
                'chapterPurpose', 'other',
                'storylineRefs', COALESCE(st_json, '[]'::jsonb),
                'materialRefs', COALESCE(mat_json, '[]'::jsonb),
                'foreshadowingRefs', COALESCE(fore_json, '[]'::jsonb),
                'generationBasis', jsonb_build_object('contextSummary', '', 'additionalInstructions', NULL)
            );

            rev_id := gen_random_uuid();
            INSERT INTO chapter_plan_revisions (id, chapter_plan_id, project_id, revision_no, snapshot, change_type, created_by, created_at)
            VALUES (rev_id, cp.id, cp.project_id, 1, snap, 'legacy_backfill', cp.created_by, cp.created_at);

            UPDATE chapter_plans SET current_revision_id = rev_id WHERE id = cp.id;
        END IF;
    END LOOP;
END $$;

CREATE OR REPLACE FUNCTION check_chapter_plan_current_revision_id() RETURNS TRIGGER AS $$
BEGIN
    IF NEW.current_revision_id IS NOT NULL THEN
        IF NOT EXISTS (
            SELECT 1 FROM chapter_plan_revisions
            WHERE id = NEW.current_revision_id AND chapter_plan_id = NEW.id
        ) THEN
            RAISE EXCEPTION 'current_revision_id must belong to the same chapter_plan';
        END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER chapter_plans_check_current_revision_id
BEFORE INSERT OR UPDATE OF current_revision_id ON chapter_plans
FOR EACH ROW EXECUTE FUNCTION check_chapter_plan_current_revision_id();
