DO $$
DECLARE n integer;
BEGIN
 SELECT count(*) INTO n FROM projects WHERE created_by='acf-test-data-f6e'; IF n<>5 THEN RAISE EXCEPTION 'expected 5 test projects, got %',n; END IF;
 SELECT count(*) INTO n FROM materials WHERE created_by='acf-test-data-f6e'; IF n<>6 THEN RAISE EXCEPTION 'expected 6 test materials, got %',n; END IF;
 SELECT count(*) INTO n FROM project_material_usages WHERE id::text LIKE 'f6e00000-0000-4000-8002-%'; IF n<>4 THEN RAISE EXCEPTION 'expected 4 usages, got %',n; END IF;
 SELECT count(*) INTO n FROM storylines WHERE project_id='f6e00000-0000-4000-8000-000000000003'; IF n<>3 THEN RAISE EXCEPTION 'expected 3 storylines, got %',n; END IF;
 SELECT count(*) INTO n FROM foreshadowings WHERE project_id='f6e00000-0000-4000-8000-000000000003'; IF n<>3 THEN RAISE EXCEPTION 'expected 3 foreshadowings, got %',n; END IF;
 SELECT count(*) INTO n FROM chapter_plans WHERE project_id='f6e00000-0000-4000-8000-000000000003'; IF n<>3 THEN RAISE EXCEPTION 'expected 3 chapter plans, got %',n; END IF;
 SELECT count(*) INTO n FROM content_items WHERE project_id='f6e00000-0000-4000-8000-000000000003'; IF n<>1 THEN RAISE EXCEPTION 'expected 1 content item, got %',n; END IF;
 IF NOT EXISTS (SELECT 1 FROM materials m WHERE m.id='f6e00000-0000-4000-8001-000000000006' AND NOT EXISTS (SELECT 1 FROM project_material_usages u WHERE u.material_id=m.id)) THEN RAISE EXCEPTION 'unbound global material missing'; END IF;
 IF (SELECT count(*) FROM project_material_usages WHERE material_id='f6e00000-0000-4000-8001-000000000001')<>2 THEN RAISE EXCEPTION 'shared material relation incorrect'; END IF;
END $$;
