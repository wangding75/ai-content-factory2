CREATE TABLE mock_generation_runs (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    provider_key TEXT NOT NULL CHECK (provider_key = 'mock'),
    workflow_key TEXT NOT NULL CHECK (workflow_key = 'chapter_plan_mock_generate'),
    status TEXT NOT NULL CHECK (status = 'succeeded'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE foreshadowings
    ADD CONSTRAINT foreshadowings_project_id_id_unique UNIQUE (project_id, id);

CREATE TABLE chapter_plans (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    chapter_no INTEGER NOT NULL CHECK (chapter_no >= 1),
    title VARCHAR(120) NOT NULL CHECK (char_length(btrim(title)) BETWEEN 1 AND 120),
    summary TEXT NOT NULL DEFAULT '' CHECK (char_length(summary) <= 5000),
    chapter_goal TEXT NULL CHECK (chapter_goal IS NULL OR char_length(chapter_goal) <= 2000),
    creation_notes TEXT NULL CHECK (creation_notes IS NULL OR char_length(creation_notes) <= 2000),
    status TEXT NOT NULL CHECK (status IN ('pending_confirmation','confirmed')),
    source TEXT NOT NULL CHECK (source = 'mock_generated'),
    mock_generation_run_id UUID NULL REFERENCES mock_generation_runs(id) ON DELETE SET NULL,
    confirmed_at TIMESTAMPTZ NULL,
    created_by TEXT NOT NULL,
    version INTEGER NOT NULL DEFAULT 1 CHECK (version >= 1),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chapter_plans_project_chapter_no_unique UNIQUE(project_id, chapter_no),
    CONSTRAINT chapter_plans_confirmation_shape CHECK ((status='pending_confirmation' AND confirmed_at IS NULL) OR (status='confirmed' AND confirmed_at IS NOT NULL))
);
CREATE TABLE chapter_plan_storylines (
    chapter_plan_id UUID NOT NULL REFERENCES chapter_plans(id) ON DELETE CASCADE,
    project_id UUID NOT NULL,
    storyline_id UUID NOT NULL,
    relation TEXT NOT NULL CHECK (relation IN ('primary','secondary')),
    position INTEGER NOT NULL CHECK (position >= 0),
    PRIMARY KEY(chapter_plan_id, storyline_id), UNIQUE(chapter_plan_id, position),
    FOREIGN KEY(project_id, storyline_id) REFERENCES storylines(project_id,id) ON DELETE RESTRICT
);
CREATE UNIQUE INDEX chapter_plan_storylines_one_primary_idx ON chapter_plan_storylines(chapter_plan_id) WHERE relation='primary';
CREATE TABLE chapter_plan_materials (
    chapter_plan_id UUID NOT NULL REFERENCES chapter_plans(id) ON DELETE CASCADE,
    project_id UUID NOT NULL, material_id UUID NOT NULL, position INTEGER NOT NULL CHECK(position>=0),
    PRIMARY KEY(chapter_plan_id, material_id), UNIQUE(chapter_plan_id, position),
    FOREIGN KEY(project_id, material_id) REFERENCES project_material_usages(project_id,material_id) ON DELETE RESTRICT
);
CREATE TABLE chapter_plan_foreshadowings (
    chapter_plan_id UUID NOT NULL REFERENCES chapter_plans(id) ON DELETE CASCADE,
    project_id UUID NOT NULL, foreshadowing_id UUID NOT NULL, position INTEGER NOT NULL CHECK(position>=0),
    PRIMARY KEY(chapter_plan_id, foreshadowing_id), UNIQUE(chapter_plan_id, position),
    FOREIGN KEY(project_id, foreshadowing_id) REFERENCES foreshadowings(project_id,id) ON DELETE RESTRICT
);
CREATE INDEX chapter_plans_project_chapter_id_idx ON chapter_plans(project_id,chapter_no,id);
