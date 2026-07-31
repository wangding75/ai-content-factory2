param()

$ErrorActionPreference = 'Stop'

$sql = @'
BEGIN;
SET CONSTRAINTS ALL DEFERRED;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM schema_migrations WHERE version = 15) THEN
        RAISE EXCEPTION 'Migration 000015 is not recorded';
    END IF;
END $$;

INSERT INTO projects(id,name,type,created_by) VALUES
    ('00000000-0000-0000-0000-000000001001','iteration16-data-foundation','novel','iteration16-validation');
INSERT INTO workflow_connections(id,name,connection_type,base_url,auth_type,timeout_seconds,type_config) VALUES
    ('00000000-0000-0000-0000-000000001002','iteration16-data-foundation','n8n','http://iteration16.invalid','api_key',30,'{}');
INSERT INTO workflow_configurations(id,name,connection_id,applicable_stages,type_config,input_contract_version,output_contract_version) VALUES
    ('00000000-0000-0000-0000-000000001003','iteration16-data-foundation','00000000-0000-0000-0000-000000001002','["content_generation"]','{}','v1','v1');
INSERT INTO chapter_plans(id,project_id,chapter_no,title,status,source,confirmed_at,created_by) VALUES
    ('00000000-0000-0000-0000-000000001004','00000000-0000-0000-0000-000000001001',1,'Iteration 16 validation','confirmed','manual',NOW(),'iteration16-validation');
INSERT INTO content_items(id,project_id,chapter_plan_id,title,current_version_id) VALUES
    ('00000000-0000-0000-0000-000000001005','00000000-0000-0000-0000-000000001001','00000000-0000-0000-0000-000000001004','Iteration 16 validation','00000000-0000-0000-0000-000000001006');
INSERT INTO content_versions(id,content_item_id,version_no,title,content,word_count,source,status) VALUES
    ('00000000-0000-0000-0000-000000001006','00000000-0000-0000-0000-000000001005',1,'Iteration 16 validation','source',1,'manual_created','editable_draft');

INSERT INTO workflow_run_records(id,run_number,project_id,stage,subject_type,subject_id,workflow_configuration_id,trigger_source,status,configuration_snapshot,input_payload) VALUES
    ('00000000-0000-0000-0000-000000001007','I16-DATA-FOUNDATION-1','00000000-0000-0000-0000-000000001001','content_generation','content_item','00000000-0000-0000-0000-000000001005','00000000-0000-0000-0000-000000001003','manual','queued','{}','{}'),
    ('00000000-0000-0000-0000-000000001008','I16-DATA-FOUNDATION-2','00000000-0000-0000-0000-000000001001','content_generation','content_item','00000000-0000-0000-0000-000000001006','00000000-0000-0000-0000-000000001003','manual','queued','{}','{}');

DO $$
BEGIN
    BEGIN
        INSERT INTO workflow_run_records(id,run_number,project_id,stage,subject_type,subject_id,workflow_configuration_id,trigger_source,status,configuration_snapshot,input_payload) VALUES
            ('00000000-0000-0000-0000-000000001013','I16-DATA-FOUNDATION-DUP','00000000-0000-0000-0000-000000001001','content_generation','content_item','00000000-0000-0000-0000-000000001005','00000000-0000-0000-0000-000000001003','manual','queued','{}','{}');
        RAISE EXCEPTION 'active content generation uniqueness was not enforced';
    EXCEPTION WHEN unique_violation THEN
        NULL;
    END;
END $$;

UPDATE workflow_run_records SET status='succeeded',output_payload='{}',started_at=NOW(),finished_at=NOW() WHERE id='00000000-0000-0000-0000-000000001007';
INSERT INTO workflow_run_records(id,run_number,project_id,stage,subject_type,subject_id,workflow_configuration_id,trigger_source,status,configuration_snapshot,input_payload) VALUES
    ('00000000-0000-0000-0000-000000001009','I16-DATA-FOUNDATION-3','00000000-0000-0000-0000-000000001001','content_generation','content_item','00000000-0000-0000-0000-000000001005','00000000-0000-0000-0000-000000001003','manual','queued','{}','{}');

INSERT INTO content_versions(id,content_item_id,version_no,source_content_version_id,source_content_version_version,source_workflow_run_id,title,content,word_count,source,status,version) VALUES
    ('00000000-0000-0000-0000-000000001010','00000000-0000-0000-0000-000000001005',2,'00000000-0000-0000-0000-000000001006',1,'00000000-0000-0000-0000-000000001007','Iteration 16 candidate','candidate',1,'workflow_generated','editable_draft',1);
INSERT INTO workflow_run_events(id,run_id,event_type,status,payload) VALUES
    ('00000000-0000-0000-0000-000000001011','00000000-0000-0000-0000-000000001007','result_consumed','succeeded','{}'),
    ('00000000-0000-0000-0000-000000001012','00000000-0000-0000-0000-000000001007','result_consumption_failed','succeeded','{}');

DO $$
BEGIN
    BEGIN
        INSERT INTO content_versions(id,content_item_id,version_no,source_content_version_id,source_content_version_version,source_workflow_run_id,title,content,word_count,source,status,version) VALUES
            ('00000000-0000-0000-0000-000000001014','00000000-0000-0000-0000-000000001005',3,'00000000-0000-0000-0000-000000001006',1,'00000000-0000-0000-0000-000000001007','Iteration 16 duplicate','candidate',1,'workflow_generated','editable_draft',1);
        RAISE EXCEPTION 'workflow run candidate uniqueness was not enforced';
    EXCEPTION WHEN unique_violation THEN
        NULL;
    END;
END $$;

DO $$
BEGIN
    IF (SELECT current_version_id FROM content_items WHERE id='00000000-0000-0000-0000-000000001005') <> '00000000-0000-0000-0000-000000001006'::uuid THEN
        RAISE EXCEPTION 'candidate changed the current content version';
    END IF;
    IF (SELECT count(*) FROM workflow_run_records WHERE project_id='00000000-0000-0000-0000-000000001001' AND stage='content_generation' AND status IN ('queued','running')) <> 2 THEN
        RAISE EXCEPTION 'different content items did not run in parallel';
    END IF;
    IF (SELECT count(*) FROM workflow_run_events WHERE run_id='00000000-0000-0000-0000-000000001007' AND event_type IN ('result_consumed','result_consumption_failed')) <> 2 THEN
        RAISE EXCEPTION 'result consumption events were not persisted';
    END IF;
END $$;

SELECT 'iteration16 data foundation transaction validation passed' AS result;
ROLLBACK;
'@

$sql | docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U postgres -d ai_content_factory
if ($LASTEXITCODE -ne 0) { throw "Iteration 16 data foundation validation failed with exit code $LASTEXITCODE." }

Write-Host '[PASS] Iteration 16 data foundation validation completed in a rolled-back transaction.' -ForegroundColor Green
