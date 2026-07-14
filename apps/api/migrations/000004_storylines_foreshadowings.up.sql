CREATE TABLE storylines (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    parent_id UUID NULL,
    type TEXT NOT NULL CHECK (type IN ('main', 'child')),
    relation TEXT NOT NULL CHECK (relation IN ('root', 'child')),
    name VARCHAR(120) NOT NULL CHECK (char_length(btrim(name)) BETWEEN 1 AND 120),
    summary TEXT NOT NULL DEFAULT '' CHECK (char_length(summary) <= 5000),
    start_chapter INTEGER NULL CHECK (start_chapter IS NULL OR start_chapter >= 1),
    end_chapter INTEGER NULL CHECK (end_chapter IS NULL OR end_chapter >= 1),
    status TEXT NOT NULL CHECK (status IN ('active', 'completed', 'archived')),
    sort_order INTEGER NOT NULL CHECK (sort_order >= 0),
    created_by TEXT NOT NULL,
    version INTEGER NOT NULL DEFAULT 1 CHECK (version >= 1),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT storylines_range CHECK (start_chapter IS NULL OR end_chapter IS NULL OR start_chapter <= end_chapter),
    CONSTRAINT storylines_shape CHECK (
        (parent_id IS NULL AND type = 'main' AND relation = 'root') OR
        (parent_id IS NOT NULL AND parent_id <> id AND type = 'child' AND relation = 'child')
    ),
    CONSTRAINT storylines_project_id_id_unique UNIQUE (project_id, id),
    CONSTRAINT storylines_parent_same_project FOREIGN KEY (project_id, parent_id)
        REFERENCES storylines(project_id, id) ON DELETE RESTRICT
);

CREATE TABLE foreshadowings (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    title VARCHAR(120) NOT NULL CHECK (char_length(btrim(title)) BETWEEN 1 AND 120),
    description TEXT NOT NULL DEFAULT '' CHECK (char_length(description) <= 5000),
    priority TEXT NOT NULL CHECK (priority IN ('low', 'medium', 'high')),
    status TEXT NOT NULL CHECK (status IN ('planned', 'planted', 'paid_off')),
    planted_plot_line_id UUID NULL,
    payoff_plot_line_id UUID NULL,
    planned_plant_chapter INTEGER NULL CHECK (planned_plant_chapter IS NULL OR planned_plant_chapter >= 1),
    planned_payoff_chapter INTEGER NULL CHECK (planned_payoff_chapter IS NULL OR planned_payoff_chapter >= 1),
    created_by TEXT NOT NULL,
    version INTEGER NOT NULL DEFAULT 1 CHECK (version >= 1),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT foreshadowings_range CHECK (planned_plant_chapter IS NULL OR planned_payoff_chapter IS NULL OR planned_plant_chapter <= planned_payoff_chapter),
    CONSTRAINT foreshadowings_planted_same_project FOREIGN KEY (project_id, planted_plot_line_id)
        REFERENCES storylines(project_id, id) ON DELETE RESTRICT,
    CONSTRAINT foreshadowings_payoff_same_project FOREIGN KEY (project_id, payoff_plot_line_id)
        REFERENCES storylines(project_id, id) ON DELETE RESTRICT
);

CREATE INDEX storylines_project_parent_sort_id_idx ON storylines (project_id, parent_id, sort_order, id);
CREATE INDEX storylines_project_status_idx ON storylines (project_id, status);
CREATE INDEX foreshadowings_project_status_priority_id_idx ON foreshadowings (project_id, status, priority, id);
CREATE INDEX foreshadowings_project_priority_plant_id_idx ON foreshadowings (
    project_id,
    (CASE priority WHEN 'high' THEN 1 WHEN 'medium' THEN 2 WHEN 'low' THEN 3 END),
    planned_plant_chapter NULLS LAST,
    id
);
CREATE INDEX foreshadowings_planted_plot_line_id_idx ON foreshadowings (planted_plot_line_id) WHERE planted_plot_line_id IS NOT NULL;
CREATE INDEX foreshadowings_payoff_plot_line_id_idx ON foreshadowings (payoff_plot_line_id) WHERE payoff_plot_line_id IS NOT NULL;