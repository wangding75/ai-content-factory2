DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM projects WHERE type <> 'novel') THEN
        RAISE EXCEPTION 'cannot roll back project type catalogue while non-novel projects exist';
    END IF;
END $$;

ALTER TABLE projects DROP CONSTRAINT IF EXISTS projects_type_check;
ALTER TABLE projects ADD CONSTRAINT projects_type_check CHECK (type = 'novel');
