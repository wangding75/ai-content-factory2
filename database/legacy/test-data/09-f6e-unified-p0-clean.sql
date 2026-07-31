BEGIN;
DELETE FROM chapter_plan_materials WHERE chapter_plan_id::text LIKE 'f6e00000-0000-4000-8005-%';
DELETE FROM chapter_plan_foreshadowings WHERE chapter_plan_id::text LIKE 'f6e00000-0000-4000-8005-%';
DELETE FROM chapter_plan_storylines WHERE chapter_plan_id::text LIKE 'f6e00000-0000-4000-8005-%';
DELETE FROM projects WHERE id::text LIKE 'f6e00000-0000-4000-8000-%';
DELETE FROM materials WHERE id::text LIKE 'f6e00000-0000-4000-8001-%';
COMMIT;
