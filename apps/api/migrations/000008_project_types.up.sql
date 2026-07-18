ALTER TABLE projects DROP CONSTRAINT IF EXISTS projects_type_check;
ALTER TABLE projects ADD CONSTRAINT projects_type_check
    CHECK (type IN ('novel', 'short_film', 'series', 'graphic_text', 'image'));
