DROP TABLE IF EXISTS chapter_plan_foreshadowings;
DROP TABLE IF EXISTS chapter_plan_materials;
DROP TABLE IF EXISTS chapter_plan_storylines;
DROP TABLE IF EXISTS chapter_plans;
DROP TABLE IF EXISTS mock_generation_runs;
ALTER TABLE foreshadowings DROP CONSTRAINT IF EXISTS foreshadowings_project_id_id_unique;
