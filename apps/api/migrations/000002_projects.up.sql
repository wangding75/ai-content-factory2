CREATE TABLE projects (
    id UUID PRIMARY KEY,
    name VARCHAR(120) NOT NULL CHECK (char_length(btrim(name)) BETWEEN 1 AND 120),
    type TEXT NOT NULL CHECK (type = 'novel'),
    status TEXT NOT NULL DEFAULT 'planning' CHECK (status IN ('planning', 'producing', 'archived')),
    description TEXT NOT NULL DEFAULT '' CHECK (char_length(description) <= 5000),
    current_stage TEXT NOT NULL DEFAULT 'project_setup' CHECK (
        current_stage IN (
            'project_setup',
            'project_planning',
            'materials',
            'storylines',
            'chapter_planning',
            'content_production',
            'review',
            'completed'
        )
    ),
    created_by TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX projects_updated_at_id_idx ON projects (updated_at DESC, id DESC);
CREATE INDEX projects_status_updated_at_id_idx ON projects (status, updated_at DESC, id DESC);
