BEGIN;

-- The fixture is deliberately removed by its stable IDs only.  It never relies
-- on TRUNCATE and cannot select ordinary user rows.
DELETE FROM chapter_plan_materials WHERE chapter_plan_id::text LIKE 'f6e00000-0000-4000-8005-%';
DELETE FROM chapter_plan_foreshadowings WHERE chapter_plan_id::text LIKE 'f6e00000-0000-4000-8005-%';
DELETE FROM chapter_plan_storylines WHERE chapter_plan_id::text LIKE 'f6e00000-0000-4000-8005-%';
DELETE FROM projects WHERE id::text LIKE 'f6e00000-0000-4000-8000-%';
DELETE FROM materials WHERE id::text LIKE 'f6e00000-0000-4000-8001-%';

INSERT INTO projects (id,name,type,status,description,current_stage,created_by,created_at,updated_at) VALUES
 ('f6e00000-0000-4000-8000-000000000001','F6E 空白小说项目','novel','planning','统一测试数据：没有策划、素材或章节。','project_setup','acf-test-data-f6e','2026-01-02T08:00:00Z','2026-01-02T08:00:00Z'),
 ('f6e00000-0000-4000-8000-000000000002','F6E 策划短片项目','short_film','planning','统一测试数据：已有项目策划。','project_planning','acf-test-data-f6e','2026-01-03T08:00:00Z','2026-01-03T08:00:00Z'),
 ('f6e00000-0000-4000-8000-000000000003','F6E 星港进行中系列','series','producing','统一测试数据：完整 P0 业务读取样本。','content_production','acf-test-data-f6e','2026-01-04T08:00:00Z','2026-01-04T08:00:00Z'),
 ('f6e00000-0000-4000-8000-000000000004','F6E 已归档图文项目','graphic_text','archived','统一测试数据：归档筛选样本。','completed','acf-test-data-f6e','2026-01-05T08:00:00Z','2026-01-05T08:00:00Z'),
 ('f6e00000-0000-4000-8000-000000000005','F6E 图像策划项目','image','planning','统一测试数据：图像项目类型样本。','materials','acf-test-data-f6e','2026-01-06T08:00:00Z','2026-01-06T08:00:00Z');

INSERT INTO project_plannings (project_id,premise,audience,style,goals_json,constraints_json,created_by,created_at,updated_at) VALUES
 ('f6e00000-0000-4000-8000-000000000002','一名剪辑师在雨夜寻找失踪胶片。','独立短片观众','悬疑克制','{"selling_points":["雨夜胶片","悬疑追寻"],"plot_summary":"剪辑师循着失踪胶片的线索，在雨夜完成最后一次放映。"}','{"emotional_tone":"克制紧张"}','acf-test-data-f6e','2026-01-03T08:00:00Z','2026-01-03T08:00:00Z'),
 ('f6e00000-0000-4000-8000-000000000003','星港管理员阻止失控航线坠入居民区。','科幻冒险读者','明快群像','{"selling_points":["星港危机","导航仪谜团","群像协作"],"plot_summary":"林澈修复旧导航仪，带领伙伴阻止失控航线坠入居民区。"}','{"emotional_tone":"紧张而坚定"}','acf-test-data-f6e','2026-01-04T08:00:00Z','2026-01-04T08:00:00Z');

INSERT INTO materials (id,type,name,summary,content_json,tags_json,created_by,created_at,updated_at) VALUES
 ('f6e00000-0000-4000-8001-000000000001','character','F6E 林澈','星港管理员，擅长修复旧导航仪。','{"role":"protagonist"}','["F6E","角色","星港"]','acf-test-data-f6e','2026-01-01T08:00:00Z','2026-01-01T08:00:00Z'),
 ('f6e00000-0000-4000-8001-000000000002','worldview','F6E 星港航线规则','航线失控时必须先保护居民区。','{"rule":"priority"}','["F6E","世界观"]','acf-test-data-f6e','2026-01-02T08:00:00Z','2026-01-02T08:00:00Z'),
 ('f6e00000-0000-4000-8001-000000000003','location','F6E 北码头','雾气常年不散的货运码头。','{"district":"north"}','["F6E","地点","搜索码头"]','acf-test-data-f6e','2026-01-03T08:00:00Z','2026-01-03T08:00:00Z'),
 ('f6e00000-0000-4000-8001-000000000004','organization','F6E 航线委员会','负责星港航线安全审查。','{"kind":"council"}','["F6E","组织"]','acf-test-data-f6e','2026-01-04T08:00:00Z','2026-01-04T08:00:00Z'),
 ('f6e00000-0000-4000-8001-000000000005','item','F6E 旧导航仪','林澈留下的可修复导航仪。','{"condition":"damaged"}','["F6E","道具","导航"]','acf-test-data-f6e','2026-01-05T08:00:00Z','2026-01-05T08:00:00Z'),
 ('f6e00000-0000-4000-8001-000000000006','reference','F6E 已解除关联参考','仍保留于全局素材库的参考资料。','{"source":"fixture"}','["F6E","已解除","搜索参考"]','acf-test-data-f6e','2026-01-06T08:00:00Z','2026-01-06T08:00:00Z');

INSERT INTO project_material_usages (id,project_id,material_id,usage_type,role_name,notes,start_chapter,end_chapter,status,created_by,created_at,updated_at) VALUES
 ('f6e00000-0000-4000-8002-000000000001','f6e00000-0000-4000-8000-000000000003','f6e00000-0000-4000-8001-000000000001','character','主角','第一至第三章使用',1,3,'active','acf-test-data-f6e','2026-01-04T08:00:00Z','2026-01-04T08:00:00Z'),
 ('f6e00000-0000-4000-8002-000000000002','f6e00000-0000-4000-8000-000000000003','f6e00000-0000-4000-8001-000000000002','world','规则背景','',1,3,'active','acf-test-data-f6e','2026-01-04T08:00:00Z','2026-01-04T08:00:00Z'),
 ('f6e00000-0000-4000-8002-000000000003','f6e00000-0000-4000-8000-000000000002','f6e00000-0000-4000-8001-000000000001','reference','借用角色','同一全局素材被两个项目引用',NULL,NULL,'active','acf-test-data-f6e','2026-01-03T08:00:00Z','2026-01-03T08:00:00Z'),
 ('f6e00000-0000-4000-8002-000000000004','f6e00000-0000-4000-8000-000000000003','f6e00000-0000-4000-8001-000000000003','location','场景','北码头开场',1,1,'active','acf-test-data-f6e','2026-01-04T08:00:00Z','2026-01-04T08:00:00Z');

INSERT INTO storylines (id,project_id,parent_id,type,relation,name,summary,start_chapter,end_chapter,status,sort_order,created_by,created_at,updated_at) VALUES
 ('f6e00000-0000-4000-8003-000000000001','f6e00000-0000-4000-8000-000000000003',NULL,'main','root','F6E 主线：守住星港','阻止失控航线。',1,3,'active',0,'acf-test-data-f6e','2026-01-04T08:00:00Z','2026-01-04T08:00:00Z'),
 ('f6e00000-0000-4000-8003-000000000002','f6e00000-0000-4000-8000-000000000003','f6e00000-0000-4000-8003-000000000001','child','child','F6E 子线：旧导航仪','修复导航仪的来历。',1,2,'active',0,'acf-test-data-f6e','2026-01-04T08:00:00Z','2026-01-04T08:00:00Z'),
 ('f6e00000-0000-4000-8003-000000000003','f6e00000-0000-4000-8000-000000000003','f6e00000-0000-4000-8003-000000000001','child','child','F6E 子线：委员会','审查委员会的选择。',2,3,'completed',1,'acf-test-data-f6e','2026-01-04T08:00:00Z','2026-01-04T08:00:00Z');

INSERT INTO foreshadowings (id,project_id,title,description,priority,status,planted_plot_line_id,payoff_plot_line_id,planned_plant_chapter,planned_payoff_chapter,created_by,created_at,updated_at) VALUES
 ('f6e00000-0000-4000-8004-000000000001','f6e00000-0000-4000-8000-000000000003','F6E 未埋下信号','后续章节的神秘信号。','low','planned',NULL,'f6e00000-0000-4000-8003-000000000001',3,3,'acf-test-data-f6e','2026-01-04T08:00:00Z','2026-01-04T08:00:00Z'),
 ('f6e00000-0000-4000-8004-000000000002','f6e00000-0000-4000-8000-000000000003','F6E 导航仪裂纹','第一章埋下的裂纹。','medium','planted','f6e00000-0000-4000-8003-000000000002','f6e00000-0000-4000-8003-000000000002',1,2,'acf-test-data-f6e','2026-01-04T08:00:00Z','2026-01-04T08:00:00Z'),
 ('f6e00000-0000-4000-8004-000000000003','f6e00000-0000-4000-8000-000000000003','F6E 委员会密令','已在第二章回收。','high','paid_off','f6e00000-0000-4000-8003-000000000003','f6e00000-0000-4000-8003-000000000001',1,2,'acf-test-data-f6e','2026-01-04T08:00:00Z','2026-01-04T08:00:00Z');

INSERT INTO mock_generation_runs (id,project_id,provider_key,workflow_key,status,created_at,updated_at) VALUES ('f6e00000-0000-4000-8005-000000000001','f6e00000-0000-4000-8000-000000000003','mock','chapter_plan_mock_generate','succeeded','2026-01-04T08:00:00Z','2026-01-04T08:00:00Z');
INSERT INTO chapter_plans (id,project_id,chapter_no,title,summary,chapter_goal,creation_notes,status,source,mock_generation_run_id,confirmed_at,created_by,created_at,updated_at) VALUES
 ('f6e00000-0000-4000-8005-000000000011','f6e00000-0000-4000-8000-000000000003',1,'F6E 第一章：雾中警报','北码头收到失控警报。','建立危机','固定测试章节','confirmed','mock_generated','f6e00000-0000-4000-8005-000000000001','2026-01-04T09:00:00Z','acf-test-data-f6e','2026-01-04T08:00:00Z','2026-01-04T08:00:00Z'),
 ('f6e00000-0000-4000-8005-000000000012','f6e00000-0000-4000-8000-000000000003',2,'F6E 第二章：修复导航','林澈修复导航仪。','推进子线',NULL,'confirmed','mock_generated','f6e00000-0000-4000-8005-000000000001','2026-01-04T10:00:00Z','acf-test-data-f6e','2026-01-04T08:00:00Z','2026-01-04T08:00:00Z'),
 ('f6e00000-0000-4000-8005-000000000013','f6e00000-0000-4000-8000-000000000003',3,'F6E 第三章：待确认航线','等待确认的章节计划。','收束主线',NULL,'pending_confirmation','mock_generated','f6e00000-0000-4000-8005-000000000001',NULL,'acf-test-data-f6e','2026-01-04T08:00:00Z','2026-01-04T08:00:00Z');
INSERT INTO chapter_plan_storylines (chapter_plan_id,project_id,storyline_id,relation,position) VALUES
 ('f6e00000-0000-4000-8005-000000000011','f6e00000-0000-4000-8000-000000000003','f6e00000-0000-4000-8003-000000000001','primary',0),
 ('f6e00000-0000-4000-8005-000000000012','f6e00000-0000-4000-8000-000000000003','f6e00000-0000-4000-8003-000000000002','primary',0),
 ('f6e00000-0000-4000-8005-000000000013','f6e00000-0000-4000-8000-000000000003','f6e00000-0000-4000-8003-000000000001','primary',0);
INSERT INTO chapter_plan_materials (chapter_plan_id,project_id,material_id,position) VALUES ('f6e00000-0000-4000-8005-000000000011','f6e00000-0000-4000-8000-000000000003','f6e00000-0000-4000-8001-000000000001',0);
INSERT INTO chapter_plan_foreshadowings (chapter_plan_id,project_id,foreshadowing_id,position) VALUES ('f6e00000-0000-4000-8005-000000000011','f6e00000-0000-4000-8000-000000000003','f6e00000-0000-4000-8004-000000000002',0);

INSERT INTO content_items (id,project_id,chapter_plan_id,title,status,current_version_id,reviewed_at,created_at,updated_at) VALUES ('f6e00000-0000-4000-8006-000000000001','f6e00000-0000-4000-8000-000000000003','f6e00000-0000-4000-8005-000000000011','F6E 雾中警报正文','reviewed','f6e00000-0000-4000-8006-000000000011','2026-01-04T11:00:00Z','2026-01-04T08:00:00Z','2026-01-04T08:00:00Z');
INSERT INTO content_versions (id,content_item_id,version_no,title,content,summary,word_count,source,status,generation_parameters,frozen_at,created_at,updated_at) VALUES ('f6e00000-0000-4000-8006-000000000011','f6e00000-0000-4000-8006-000000000001',1,'F6E 雾中警报正文','固定测试正文内容。','北码头警报。',8,'mock_generated','frozen','{}','2026-01-04T11:00:00Z','2026-01-04T08:00:00Z','2026-01-04T08:00:00Z');
INSERT INTO workflow_runs (id,project_id,content_item_id,content_version_id,provider_key,workflow_key,subject_type,subject_id,status,idempotency_key,request_fingerprint,input_json,output_json,started_at,finished_at,created_at,updated_at) VALUES
 ('f6e00000-0000-4000-8007-000000000001','f6e00000-0000-4000-8000-000000000003','f6e00000-0000-4000-8006-000000000001','f6e00000-0000-4000-8006-000000000011','mock','content_mock_generate','content_item','f6e00000-0000-4000-8006-000000000001','succeeded','f6e-load-generate','aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','{}','{}','2026-01-04T08:00:00Z','2026-01-04T08:01:00Z','2026-01-04T08:00:00Z','2026-01-04T08:01:00Z'),
 ('f6e00000-0000-4000-8007-000000000002','f6e00000-0000-4000-8000-000000000003','f6e00000-0000-4000-8006-000000000001','f6e00000-0000-4000-8006-000000000011','mock','content_mock_review','content_item','f6e00000-0000-4000-8006-000000000001','succeeded','f6e-load-review','bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb','{}','{}','2026-01-04T11:00:00Z','2026-01-04T11:01:00Z','2026-01-04T11:00:00Z','2026-01-04T11:01:00Z');
INSERT INTO review_reports (id,project_id,content_item_id,content_version_id,workflow_run_id,provider_key,status,conclusion,score,summary,created_at,completed_at) VALUES ('f6e00000-0000-4000-8008-000000000001','f6e00000-0000-4000-8000-000000000003','f6e00000-0000-4000-8006-000000000001','f6e00000-0000-4000-8006-000000000011','f6e00000-0000-4000-8007-000000000002','mock','completed','pass',92,'固定测试审核通过。','2026-01-04T11:01:00Z','2026-01-04T11:01:00Z');
INSERT INTO review_findings (id,review_id,category,severity,title,description,sort_order,created_at) VALUES ('f6e00000-0000-4000-8008-000000000011','f6e00000-0000-4000-8008-000000000001','pacing','low','F6E 节奏提示','开场节奏正常。',0,'2026-01-04T11:01:00Z');
INSERT INTO review_recommendations (id,review_id,priority,title,description,sort_order,created_at) VALUES ('f6e00000-0000-4000-8008-000000000021','f6e00000-0000-4000-8008-000000000001','low','F6E 保持航线线索','下一章继续引用导航仪。',0,'2026-01-04T11:01:00Z');
COMMIT;
