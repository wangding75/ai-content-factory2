DROP TABLE IF EXISTS review_recommendations;
DROP TABLE IF EXISTS review_findings;
DROP TABLE IF EXISTS review_reports;
DROP TABLE IF EXISTS workflow_runs;
ALTER TABLE content_items DROP CONSTRAINT IF EXISTS content_items_current_version_same_item;
DROP TABLE IF EXISTS content_versions;
DROP TABLE IF EXISTS content_items;
ALTER TABLE chapter_plans DROP CONSTRAINT IF EXISTS chapter_plans_project_id_id_unique;
