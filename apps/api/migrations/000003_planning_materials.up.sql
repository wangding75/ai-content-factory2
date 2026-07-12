CREATE TABLE project_plannings (
    project_id UUID PRIMARY KEY REFERENCES projects(id) ON DELETE CASCADE,
    premise TEXT NOT NULL DEFAULT '' CHECK (char_length(premise) <= 500),
    audience TEXT NOT NULL DEFAULT '' CHECK (char_length(audience) <= 500),
    style TEXT NOT NULL DEFAULT '' CHECK (char_length(style) <= 120),
    goals_json JSONB NOT NULL CHECK (jsonb_typeof(goals_json) = 'object'),
    constraints_json JSONB NOT NULL CHECK (jsonb_typeof(constraints_json) = 'object'),
    created_by TEXT NOT NULL,
    version INTEGER NOT NULL DEFAULT 1 CHECK (version >= 1),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE materials (
    id UUID PRIMARY KEY,
    type TEXT NOT NULL CHECK (type IN ('character', 'worldview', 'location', 'organization', 'item', 'reference')),
    name VARCHAR(120) NOT NULL CHECK (char_length(btrim(name)) BETWEEN 1 AND 120),
    summary TEXT NOT NULL DEFAULT '' CHECK (char_length(summary) <= 5000),
    content_json JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(content_json) = 'object'),
    tags_json JSONB NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(tags_json) = 'array'),
    created_by TEXT NOT NULL,
    version INTEGER NOT NULL DEFAULT 1 CHECK (version >= 1),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE project_material_usages (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    material_id UUID NOT NULL REFERENCES materials(id) ON DELETE RESTRICT,
    usage_type VARCHAR(120) NOT NULL CHECK (char_length(btrim(usage_type)) BETWEEN 1 AND 120),
    role_name VARCHAR(120) NOT NULL DEFAULT '',
    notes VARCHAR(300) NOT NULL DEFAULT '',
    start_chapter INTEGER NULL CHECK (start_chapter IS NULL OR start_chapter >= 1),
    end_chapter INTEGER NULL CHECK (end_chapter IS NULL OR end_chapter >= 1),
    status TEXT NOT NULL DEFAULT 'active' CHECK (status = 'active'),
    created_by TEXT NOT NULL,
    version INTEGER NOT NULL DEFAULT 1 CHECK (version >= 1),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT project_material_usages_project_material_unique UNIQUE (project_id, material_id),
    CONSTRAINT project_material_usages_chapter_range CHECK (
        start_chapter IS NULL OR end_chapter IS NULL OR start_chapter <= end_chapter
    )
);

CREATE TABLE idempotency_records (
    id UUID PRIMARY KEY,
    scope TEXT NOT NULL,
    idempotency_key VARCHAR(128) NOT NULL CHECK (char_length(btrim(idempotency_key)) BETWEEN 1 AND 128),
    request_hash CHAR(64) NOT NULL,
    response_status INTEGER NOT NULL CHECK (response_status BETWEEN 100 AND 599),
    response_body JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NULL,
    CONSTRAINT idempotency_records_scope_key_unique UNIQUE (scope, idempotency_key)
);

CREATE INDEX materials_type_updated_at_id_idx ON materials (type, updated_at DESC, id ASC);
CREATE INDEX materials_name_idx ON materials (name);
CREATE INDEX project_material_usages_project_id_idx ON project_material_usages (project_id);
CREATE INDEX project_material_usages_material_id_idx ON project_material_usages (material_id);
CREATE INDEX idempotency_records_expires_at_idx ON idempotency_records (expires_at) WHERE expires_at IS NOT NULL;
