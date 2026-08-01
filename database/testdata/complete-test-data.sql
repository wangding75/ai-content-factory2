-- ACF deterministic complete test dataset
-- Target schema: Migration 000001..000019
-- Preconditions:
--   1. DATABASE_URL points to ai_content_factory.
--   2. Business tables are empty.
--   3. schema_migrations remains at version 19 and dirty=false.
--
-- This file inserts deterministic, non-sensitive fixture data only.
-- It does not truncate or alter schema.

\set ON_ERROR_STOP on

BEGIN;
SET CONSTRAINTS ALL DEFERRED;
SET LOCAL TIME ZONE 'UTC';

-- ---------------------------------------------------------------------------
-- Deterministic identifiers
-- ---------------------------------------------------------------------------
-- Project A: 10000000-0000-4000-8000-000000000001
-- Project B: 10000000-0000-4000-8000-000000000002
-- Fixture clock starts at 2026-01-01 00:00:00+00.

-- ---------------------------------------------------------------------------
-- Global configuration
-- ---------------------------------------------------------------------------
INSERT INTO llm_provider_configurations (
    id, name, provider_type, base_url, default_model,
    encrypted_secret, secret_fingerprint, timeout_seconds,
    integration_status, enabled, last_verified_at,
    last_verified_version, validation_details, model_catalog_updated_at,
    last_error_code, last_error_message, version, created_at, updated_at
) VALUES
(
    '20000000-0000-4000-8000-000000000001',
    'acf-fixture-llm-enabled',
    'openai_compatible',
    'https://fixture.invalid/v1',
    'fixture-model',
    NULL,
    NULL,
    30,
    'verified',
    TRUE,
    '2026-01-01 00:01:00+00',
    1,
    '{"catalog":"fixture"}'::jsonb,
    '2026-01-01 00:01:00+00',
    NULL,
    NULL,
    1,
    '2026-01-01 00:00:00+00',
    '2026-01-01 00:01:00+00'
),
(
    '20000000-0000-4000-8000-000000000002',
    'acf-fixture-llm-disabled',
    'openai_compatible',
    'https://fixture-disabled.invalid/v1',
    'fixture-disabled-model',
    NULL,
    NULL,
    30,
    'unverified',
    FALSE,
    NULL,
    NULL,
    '{}'::jsonb,
    NULL,
    NULL,
    NULL,
    1,
    '2026-01-01 00:00:10+00',
    '2026-01-01 00:00:10+00'
);

INSERT INTO llm_provider_models (
    id, provider_id, model_key, source, availability,
    last_seen_at, created_at, updated_at
) VALUES
(
    '20500000-0000-4000-8000-000000000001',
    '20000000-0000-4000-8000-000000000001',
    'fixture-model',
    'discovered',
    'available',
    '2026-01-01 00:01:00+00',
    '2026-01-01 00:00:55+00',
    '2026-01-01 00:01:00+00'
),
(
    '20500000-0000-4000-8000-000000000002',
    '20000000-0000-4000-8000-000000000001',
    'fixture-manual-model',
    'manual',
    'unavailable',
    NULL,
    '2026-01-01 00:00:56+00',
    '2026-01-01 00:00:56+00'
);

INSERT INTO workflow_connections (
    id, name, connection_type, base_url, auth_type,
    encrypted_credential, credential_fingerprint, timeout_seconds,
    type_config, integration_status, enabled, last_verified_at,
    last_verified_version, validation_details,
    last_error_code, last_error_message, version, created_at, updated_at
) VALUES
(
    '21000000-0000-4000-8000-000000000001',
    'acf-fixture-n8n-enabled',
    'n8n',
    'https://n8n.fixture.invalid',
    'api_key',
    NULL,
    NULL,
    60,
    '{"webhookMode":"test","fixture":true}'::jsonb,
    'verified',
    TRUE,
    '2026-01-01 00:02:00+00',
    1,
    '{"connectivity":"fixture"}'::jsonb,
    NULL,
    NULL,
    1,
    '2026-01-01 00:00:20+00',
    '2026-01-01 00:02:00+00'
),
(
    '21000000-0000-4000-8000-000000000002',
    'acf-fixture-n8n-disabled',
    'n8n',
    'https://n8n-disabled.fixture.invalid',
    'api_key',
    NULL,
    NULL,
    60,
    '{"webhookMode":"test","fixture":true}'::jsonb,
    'unverified',
    FALSE,
    NULL,
    NULL,
    '{}'::jsonb,
    NULL,
    NULL,
    1,
    '2026-01-01 00:00:30+00',
    '2026-01-01 00:00:30+00'
);

INSERT INTO workflow_configurations (
    id, name, connection_id, applicable_stages, type_config,
    input_contract_version, output_contract_version,
    default_parameters, note, llm_strategy, llm_provider_id, llm_model,
    integration_status, enabled, last_verified_at, last_verified_version,
    validation_details, last_error_code, last_error_message,
    version, created_at, updated_at
) VALUES
(
    '22000000-0000-4000-8000-000000000001',
    'acf-fixture-chapter-planning',
    '21000000-0000-4000-8000-000000000001',
    '["chapter_planning"]'::jsonb,
    '{"workflowId":"fixture-chapter-planning"}'::jsonb,
    'chapter-plan.input.v1',
    'chapter-plan.output.v1',
    '{"temperature":0.2}'::jsonb,
    'Deterministic fixture workflow',
    'none',
    NULL,
    NULL,
    'verified',
    TRUE,
    '2026-01-01 00:02:10+00',
    1,
    '{"workflow":"fixture"}'::jsonb,
    NULL,
    NULL,
    1,
    '2026-01-01 00:00:40+00',
    '2026-01-01 00:02:10+00'
),
(
    '22000000-0000-4000-8000-000000000002',
    'acf-fixture-content-generation',
    '21000000-0000-4000-8000-000000000001',
    '["content_generation"]'::jsonb,
    '{"workflowId":"fixture-content-generation"}'::jsonb,
    'content-generation.input.v1',
    'content-generation.output.v1',
    '{"temperature":0.3}'::jsonb,
    'Deterministic fixture workflow',
    'acf_managed',
    '20000000-0000-4000-8000-000000000001',
    'fixture-model',
    'verified',
    TRUE,
    '2026-01-01 00:02:20+00',
    1,
    '{"workflow":"fixture","provider":"fixture"}'::jsonb,
    NULL,
    NULL,
    1,
    '2026-01-01 00:00:50+00',
    '2026-01-01 00:02:20+00'
),
(
    '22000000-0000-4000-8000-000000000003',
    'acf-fixture-review',
    '21000000-0000-4000-8000-000000000001',
    '["review"]'::jsonb,
    '{"workflowId":"fixture-review"}'::jsonb,
    'review.input.v1',
    'review.output.v1',
    '{"strict":true}'::jsonb,
    'Deterministic fixture workflow',
    'n8n_managed',
    NULL,
    NULL,
    'verified',
    TRUE,
    '2026-01-01 00:02:30+00',
    1,
    '{"workflow":"fixture","n8n":"fixture"}'::jsonb,
    NULL,
    NULL,
    1,
    '2026-01-01 00:01:00+00',
    '2026-01-01 00:02:30+00'
),
(
    '22000000-0000-4000-8000-000000000004',
    'acf-fixture-rewrite',
    '21000000-0000-4000-8000-000000000001',
    '["rewrite"]'::jsonb,
    '{"workflowId":"fixture-rewrite"}'::jsonb,
    'rewrite.input.v1',
    'rewrite.output.v1',
    '{"preserveVoice":true}'::jsonb,
    'Deterministic fixture workflow',
    'none',
    NULL,
    NULL,
    'stale',
    TRUE,
    '2026-01-01 00:02:40+00',
    1,
    '{"reason":"fixture_stale"}'::jsonb,
    NULL,
    NULL,
    2,
    '2026-01-01 00:01:10+00',
    '2026-01-01 00:02:40+00'
);

INSERT INTO distribution_platform_configurations (
    id, name, platform_type, account_identifier, endpoint_url,
    auth_type, encrypted_credential, credential_fingerprint,
    timeout_seconds, type_config, note, integration_status,
    enabled, last_verified_at, last_error_code, last_error_message,
    version, created_at, updated_at
) VALUES
(
    '23000000-0000-4000-8000-000000000001',
    'acf-fixture-custom-platform',
    'custom',
    'fixture-account',
    'https://publisher.fixture.invalid',
    'custom',
    NULL,
    NULL,
    30,
    '{"fixture":true}'::jsonb,
    'Non-production fixture platform',
    'not_connected',
    FALSE,
    NULL,
    NULL,
    NULL,
    1,
    '2026-01-01 00:01:20+00',
    '2026-01-01 00:01:20+00'
);

-- ---------------------------------------------------------------------------
-- Projects, planning and materials
-- ---------------------------------------------------------------------------
INSERT INTO projects (
    id, name, type, status, description, current_stage,
    created_by, created_at, updated_at
) VALUES
(
    '10000000-0000-4000-8000-000000000001',
    'ACF Fixture Novel',
    'novel',
    'producing',
    'Deterministic end-to-end fixture project.',
    'review',
    'fixture-system',
    '2026-01-01 01:00:00+00',
    '2026-01-01 08:00:00+00'
),
(
    '10000000-0000-4000-8000-000000000002',
    'ACF Fixture Short Film',
    'short_film',
    'planning',
    'Secondary project used to verify project isolation.',
    'chapter_planning',
    'fixture-system',
    '2026-01-01 01:05:00+00',
    '2026-01-01 07:00:00+00'
);

INSERT INTO project_plannings (
    project_id, premise, audience, style, goals_json,
    constraints_json, created_by, version, created_at, updated_at
) VALUES
(
    '10000000-0000-4000-8000-000000000001',
    'A city archivist discovers that forgotten stories alter reality.',
    'Adult speculative-fiction readers.',
    'Restrained cinematic prose',
    '{"primary":"complete a coherent first arc"}'::jsonb,
    '{"chapters":12,"rating":"general"}'::jsonb,
    'fixture-system',
    1,
    '2026-01-01 01:10:00+00',
    '2026-01-01 01:10:00+00'
),
(
    '10000000-0000-4000-8000-000000000002',
    'A courier must deliver a message before sunrise.',
    'Short-form drama audience.',
    'Fast visual storytelling',
    '{"primary":"produce a concise short-film outline"}'::jsonb,
    '{"minutes":15}'::jsonb,
    'fixture-system',
    1,
    '2026-01-01 01:11:00+00',
    '2026-01-01 01:11:00+00'
);

INSERT INTO materials (
    id, type, name, summary, content_json, tags_json,
    created_by, version, created_at, updated_at
) VALUES
(
    '30000000-0000-4000-8000-000000000001',
    'character',
    'Lin An',
    'An archivist who notices contradictions in public memory.',
    '{"role":"protagonist","traits":["patient","skeptical"]}'::jsonb,
    '["fixture","character"]'::jsonb,
    'fixture-system',
    1,
    '2026-01-01 01:20:00+00',
    '2026-01-01 01:20:00+00'
),
(
    '30000000-0000-4000-8000-000000000002',
    'worldview',
    'The Memory Archive',
    'A municipal archive whose records affect the physical city.',
    '{"rule":"published records become consensus reality"}'::jsonb,
    '["fixture","worldview"]'::jsonb,
    'fixture-system',
    1,
    '2026-01-01 01:21:00+00',
    '2026-01-01 01:21:00+00'
),
(
    '30000000-0000-4000-8000-000000000003',
    'reference',
    'Short Film Beat Sheet',
    'Reference structure for the secondary project.',
    '{"beats":8}'::jsonb,
    '["fixture","reference"]'::jsonb,
    'fixture-system',
    1,
    '2026-01-01 01:22:00+00',
    '2026-01-01 01:22:00+00'
);

INSERT INTO project_material_usages (
    id, project_id, material_id, usage_type, role_name, notes,
    start_chapter, end_chapter, status, created_by, version,
    created_at, updated_at
) VALUES
(
    '31000000-0000-4000-8000-000000000001',
    '10000000-0000-4000-8000-000000000001',
    '30000000-0000-4000-8000-000000000001',
    'main_character',
    'Protagonist',
    'Primary viewpoint character.',
    1,
    12,
    'active',
    'fixture-system',
    1,
    '2026-01-01 01:30:00+00',
    '2026-01-01 01:30:00+00'
),
(
    '31000000-0000-4000-8000-000000000002',
    '10000000-0000-4000-8000-000000000001',
    '30000000-0000-4000-8000-000000000002',
    'world_rule',
    '',
    'Core setting rule.',
    1,
    12,
    'active',
    'fixture-system',
    1,
    '2026-01-01 01:31:00+00',
    '2026-01-01 01:31:00+00'
),
(
    '31000000-0000-4000-8000-000000000003',
    '10000000-0000-4000-8000-000000000002',
    '30000000-0000-4000-8000-000000000003',
    'structure_reference',
    '',
    'Reference for scene order.',
    1,
    1,
    'active',
    'fixture-system',
    1,
    '2026-01-01 01:32:00+00',
    '2026-01-01 01:32:00+00'
);

-- ---------------------------------------------------------------------------
-- Storylines and foreshadowing
-- ---------------------------------------------------------------------------
INSERT INTO storylines (
    id, project_id, parent_id, type, relation, name, summary,
    start_chapter, end_chapter, status, sort_order,
    created_by, version, created_at, updated_at
) VALUES
(
    '40000000-0000-4000-8000-000000000001',
    '10000000-0000-4000-8000-000000000001',
    NULL,
    'main',
    'root',
    'The Missing District',
    'Lin An traces references to a district erased from every official map.',
    1,
    12,
    'active',
    0,
    'fixture-system',
    1,
    '2026-01-01 02:00:00+00',
    '2026-01-01 02:00:00+00'
),
(
    '40000000-0000-4000-8000-000000000002',
    '10000000-0000-4000-8000-000000000001',
    '40000000-0000-4000-8000-000000000001',
    'child',
    'child',
    'The Red Index Card',
    'A recurring index card points to events that have not happened yet.',
    1,
    6,
    'active',
    1,
    'fixture-system',
    1,
    '2026-01-01 02:01:00+00',
    '2026-01-01 02:01:00+00'
),
(
    '40000000-0000-4000-8000-000000000003',
    '10000000-0000-4000-8000-000000000002',
    NULL,
    'main',
    'root',
    'Before Sunrise',
    'The courier crosses the city before the final bridge closes.',
    1,
    1,
    'active',
    0,
    'fixture-system',
    1,
    '2026-01-01 02:02:00+00',
    '2026-01-01 02:02:00+00'
);

INSERT INTO foreshadowings (
    id, project_id, title, description, priority, status,
    planted_plot_line_id, payoff_plot_line_id,
    planned_plant_chapter, planned_payoff_chapter,
    created_by, version, created_at, updated_at
) VALUES
(
    '41000000-0000-4000-8000-000000000001',
    '10000000-0000-4000-8000-000000000001',
    'The blank shelf',
    'One shelf has catalogue numbers but no physical records.',
    'high',
    'planted',
    '40000000-0000-4000-8000-000000000001',
    '40000000-0000-4000-8000-000000000002',
    1,
    6,
    'fixture-system',
    1,
    '2026-01-01 02:10:00+00',
    '2026-01-01 02:10:00+00'
);

-- ---------------------------------------------------------------------------
-- Workflow bindings
-- ---------------------------------------------------------------------------
INSERT INTO project_workflow_bindings (
    id, project_id, stage, workflow_configuration_id,
    version, created_at, updated_at
) VALUES
(
    '24000000-0000-4000-8000-000000000001',
    '10000000-0000-4000-8000-000000000001',
    'chapter_planning',
    '22000000-0000-4000-8000-000000000001',
    1,
    '2026-01-01 02:20:00+00',
    '2026-01-01 02:20:00+00'
),
(
    '24000000-0000-4000-8000-000000000002',
    '10000000-0000-4000-8000-000000000001',
    'content_generation',
    '22000000-0000-4000-8000-000000000002',
    1,
    '2026-01-01 02:21:00+00',
    '2026-01-01 02:21:00+00'
),
(
    '24000000-0000-4000-8000-000000000003',
    '10000000-0000-4000-8000-000000000001',
    'review',
    '22000000-0000-4000-8000-000000000003',
    1,
    '2026-01-01 02:22:00+00',
    '2026-01-01 02:22:00+00'
),
(
    '24000000-0000-4000-8000-000000000004',
    '10000000-0000-4000-8000-000000000001',
    'rewrite',
    '22000000-0000-4000-8000-000000000004',
    1,
    '2026-01-01 02:23:00+00',
    '2026-01-01 02:23:00+00'
),
(
    '24000000-0000-4000-8000-000000000005',
    '10000000-0000-4000-8000-000000000002',
    'chapter_planning',
    '22000000-0000-4000-8000-000000000001',
    1,
    '2026-01-01 02:24:00+00',
    '2026-01-01 02:24:00+00'
),
(
    '24000000-0000-4000-8000-000000000006',
    '10000000-0000-4000-8000-000000000002',
    'content_generation',
    '22000000-0000-4000-8000-000000000002',
    1,
    '2026-01-01 02:25:00+00',
    '2026-01-01 02:25:00+00'
),
(
    '24000000-0000-4000-8000-000000000007',
    '10000000-0000-4000-8000-000000000002',
    'review',
    '22000000-0000-4000-8000-000000000003',
    1,
    '2026-01-01 02:26:00+00',
    '2026-01-01 02:26:00+00'
);

-- ---------------------------------------------------------------------------
-- Runtime run used by chapter planning candidate generation
-- ---------------------------------------------------------------------------
INSERT INTO workflow_run_records (
    id, run_number, project_id, stage, workflow_configuration_id,
    trigger_source, status, configuration_snapshot, input_payload,
    output_payload, error_code, error_message, error_details,
    retry_of_run_id, started_at, finished_at, cancelled_at,
    created_at, updated_at, version, subject_type, subject_id
) VALUES
(
    '50000000-0000-4000-8000-000000000001',
    'ACF-FIX-CP-0001',
    '10000000-0000-4000-8000-000000000001',
    'chapter_planning',
    '22000000-0000-4000-8000-000000000001',
    'manual',
    'succeeded',
    '{"stage":"chapter_planning","workflowConfiguration":{"inputContractVersion":"chapter-plan.input.v1","outputContractVersion":"chapter-plan.output.v1"}}'::jsonb,
    '{"schemaVersion":"chapter-plan.input.v1","projectId":"10000000-0000-4000-8000-000000000001"}'::jsonb,
    '{"schemaVersion":"chapter-plan.output.v1","candidateCount":1}'::jsonb,
    NULL,
    NULL,
    NULL,
    NULL,
    '2026-01-01 03:00:10+00',
    '2026-01-01 03:00:50+00',
    NULL,
    '2026-01-01 03:00:00+00',
    '2026-01-01 03:00:50+00',
    1,
    NULL,
    NULL
);

INSERT INTO workflow_run_events (
    id, run_id, event_type, status, payload, created_at
) VALUES
(
    '51000000-0000-4000-8000-000000000001',
    '50000000-0000-4000-8000-000000000001',
    'queued',
    'queued',
    '{"fixture":true}'::jsonb,
    '2026-01-01 03:00:00+00'
),
(
    '51000000-0000-4000-8000-000000000002',
    '50000000-0000-4000-8000-000000000001',
    'worker_started',
    'running',
    '{"fixture":true}'::jsonb,
    '2026-01-01 03:00:10+00'
),
(
    '51000000-0000-4000-8000-000000000003',
    '50000000-0000-4000-8000-000000000001',
    'output_validated',
    'running',
    '{"fixture":true}'::jsonb,
    '2026-01-01 03:00:40+00'
),
(
    '51000000-0000-4000-8000-000000000004',
    '50000000-0000-4000-8000-000000000001',
    'succeeded',
    'succeeded',
    '{"fixture":true}'::jsonb,
    '2026-01-01 03:00:50+00'
);

-- ---------------------------------------------------------------------------
-- Chapter plans and revisions
-- ---------------------------------------------------------------------------
INSERT INTO mock_generation_runs (
    id, project_id, provider_key, workflow_key, status,
    created_at, updated_at
) VALUES
(
    '52000000-0000-4000-8000-000000000001',
    '10000000-0000-4000-8000-000000000001',
    'mock',
    'chapter_plan_mock_generate',
    'succeeded',
    '2026-01-01 03:05:00+00',
    '2026-01-01 03:05:30+00'
);

INSERT INTO chapter_plans (
    id, project_id, chapter_no, title, summary, chapter_goal,
    creation_notes, status, source, mock_generation_run_id,
    confirmed_at, created_by, version, created_at, updated_at,
    current_revision_id, source_candidate_id,
    source_candidate_batch_id, source_workflow_run_id
) VALUES
(
    '60000000-0000-4000-8000-000000000001',
    '10000000-0000-4000-8000-000000000001',
    1,
    'The Catalogue Error',
    'Lin An finds a catalogue number that points to a missing district.',
    'Introduce the archive rule and the central mystery.',
    'Fixture chapter created from the legacy mock path.',
    'confirmed',
    'mock_generated',
    '52000000-0000-4000-8000-000000000001',
    '2026-01-01 03:11:00+00',
    'fixture-system',
    1,
    '2026-01-01 03:10:00+00',
    '2026-01-01 03:11:00+00',
    NULL,
    NULL,
    NULL,
    NULL
),
(
    '60000000-0000-4000-8000-000000000003',
    '10000000-0000-4000-8000-000000000002',
    1,
    'The Last Delivery',
    'The courier receives an envelope that must arrive before sunrise.',
    'Establish urgency and destination.',
    'Fixture manual chapter.',
    'confirmed',
    'manual',
    NULL,
    '2026-01-01 03:13:00+00',
    'fixture-system',
    1,
    '2026-01-01 03:12:00+00',
    '2026-01-01 03:13:00+00',
    NULL,
    NULL,
    NULL,
    NULL
);

INSERT INTO chapter_plan_revisions (
    id, chapter_plan_id, project_id, revision_no, snapshot,
    change_type, source_candidate_id, source_candidate_batch_id,
    source_workflow_run_id, created_by, created_at
) VALUES
(
    '61000000-0000-4000-8000-000000000001',
    '60000000-0000-4000-8000-000000000001',
    '10000000-0000-4000-8000-000000000001',
    1,
    '{"chapterNo":1,"title":"The Catalogue Error","summary":"Lin An finds a catalogue number that points to a missing district.","chapterPurpose":"setup","storylineRefs":[{"id":"40000000-0000-4000-8000-000000000001","relation":"primary","position":0}],"materialRefs":[{"id":"30000000-0000-4000-8000-000000000001","position":0}],"foreshadowingRefs":[{"id":"41000000-0000-4000-8000-000000000001","position":0}],"generationBasis":{"contextSummary":"fixture","additionalInstructions":null}}'::jsonb,
    'legacy_backfill',
    NULL,
    NULL,
    NULL,
    'fixture-system',
    '2026-01-01 03:10:30+00'
),
(
    '61000000-0000-4000-8000-000000000003',
    '60000000-0000-4000-8000-000000000003',
    '10000000-0000-4000-8000-000000000002',
    1,
    '{"chapterNo":1,"title":"The Last Delivery","summary":"The courier receives an envelope that must arrive before sunrise.","chapterPurpose":"setup","storylineRefs":[{"id":"40000000-0000-4000-8000-000000000003","relation":"primary","position":0}],"materialRefs":[{"id":"30000000-0000-4000-8000-000000000003","position":0}],"foreshadowingRefs":[],"generationBasis":{"contextSummary":"fixture","additionalInstructions":null}}'::jsonb,
    'manual_create',
    NULL,
    NULL,
    NULL,
    'fixture-system',
    '2026-01-01 03:12:30+00'
);

UPDATE chapter_plans
SET current_revision_id = CASE id
    WHEN '60000000-0000-4000-8000-000000000001'::uuid
        THEN '61000000-0000-4000-8000-000000000001'::uuid
    WHEN '60000000-0000-4000-8000-000000000003'::uuid
        THEN '61000000-0000-4000-8000-000000000003'::uuid
END
WHERE id IN (
    '60000000-0000-4000-8000-000000000001',
    '60000000-0000-4000-8000-000000000003'
);

INSERT INTO chapter_plan_storylines (
    chapter_plan_id, project_id, storyline_id, relation, position
) VALUES
(
    '60000000-0000-4000-8000-000000000001',
    '10000000-0000-4000-8000-000000000001',
    '40000000-0000-4000-8000-000000000001',
    'primary',
    0
),
(
    '60000000-0000-4000-8000-000000000003',
    '10000000-0000-4000-8000-000000000002',
    '40000000-0000-4000-8000-000000000003',
    'primary',
    0
);

INSERT INTO chapter_plan_materials (
    chapter_plan_id, project_id, material_id, position
) VALUES
(
    '60000000-0000-4000-8000-000000000001',
    '10000000-0000-4000-8000-000000000001',
    '30000000-0000-4000-8000-000000000001',
    0
),
(
    '60000000-0000-4000-8000-000000000003',
    '10000000-0000-4000-8000-000000000002',
    '30000000-0000-4000-8000-000000000003',
    0
);

INSERT INTO chapter_plan_foreshadowings (
    chapter_plan_id, project_id, foreshadowing_id, position
) VALUES
(
    '60000000-0000-4000-8000-000000000001',
    '10000000-0000-4000-8000-000000000001',
    '41000000-0000-4000-8000-000000000001',
    0
);

-- ---------------------------------------------------------------------------
-- Candidate batch, candidate adoption and adopted chapter plan
-- ---------------------------------------------------------------------------
INSERT INTO chapter_plan_candidate_batches (
    id, project_id, source_workflow_run_id, generation_mode,
    range_start, range_end, requested_chapter_count, input_digest,
    input_snapshot, storyline_selection_snapshot, context_options,
    additional_instructions, workflow_binding_snapshot, status,
    candidate_count, pending_count, stale_count, adopted_count,
    discarded_count, completed_at, abandoned_at, abandon_reason,
    created_by, updated_by, version, created_at, updated_at
) VALUES
(
    '62000000-0000-4000-8000-000000000001',
    '10000000-0000-4000-8000-000000000001',
    '50000000-0000-4000-8000-000000000001',
    'append',
    2,
    2,
    1,
    repeat('a', 64),
    '{"projectId":"10000000-0000-4000-8000-000000000001","mode":"append"}'::jsonb,
    '{"storylineIds":["40000000-0000-4000-8000-000000000001"]}'::jsonb,
    '{"includeMaterials":true}'::jsonb,
    'Continue the archive mystery.',
    '{"workflowConfigurationId":"22000000-0000-4000-8000-000000000001"}'::jsonb,
    'adopted',
    1,
    0,
    0,
    1,
    0,
    '2026-01-01 03:21:00+00',
    NULL,
    NULL,
    'fixture-system',
    'fixture-system',
    1,
    '2026-01-01 03:20:00+00',
    '2026-01-01 03:21:00+00'
);

INSERT INTO chapter_plan_candidates (
    id, batch_id, project_id, chapter_no, sort_order,
    base_chapter_plan_id, base_revision_id, base_chapter_version,
    base_snapshot, generated_snapshot, current_snapshot, diff_type,
    status, adopted_chapter_plan_id, adopted_revision_id,
    adopted_at, discarded_at, discard_reason, created_by, updated_by,
    last_edited_by, last_edited_at, version, created_at, updated_at
) VALUES
(
    '63000000-0000-4000-8000-000000000001',
    '62000000-0000-4000-8000-000000000001',
    '10000000-0000-4000-8000-000000000001',
    2,
    0,
    NULL,
    NULL,
    NULL,
    NULL,
    '{"chapterNo":2,"title":"The Red Index Card","summary":"A red card predicts tomorrow''s archive incident.","chapterPurpose":"escalation","storylineRefs":[{"id":"40000000-0000-4000-8000-000000000001","relation":"primary","position":0}],"materialRefs":[{"id":"30000000-0000-4000-8000-000000000002","position":0}],"foreshadowingRefs":[{"id":"41000000-0000-4000-8000-000000000001","position":0}],"generationBasis":{"contextSummary":"fixture","additionalInstructions":"Continue the archive mystery."}}'::jsonb,
    '{"chapterNo":2,"title":"The Red Index Card","summary":"A red card predicts tomorrow''s archive incident.","chapterPurpose":"escalation","storylineRefs":[{"id":"40000000-0000-4000-8000-000000000001","relation":"primary","position":0}],"materialRefs":[{"id":"30000000-0000-4000-8000-000000000002","position":0}],"foreshadowingRefs":[{"id":"41000000-0000-4000-8000-000000000001","position":0}],"generationBasis":{"contextSummary":"fixture","additionalInstructions":"Continue the archive mystery."}}'::jsonb,
    'new',
    'pending',
    NULL,
    NULL,
    NULL,
    NULL,
    NULL,
    'fixture-system',
    'fixture-system',
    NULL,
    NULL,
    1,
    '2026-01-01 03:20:10+00',
    '2026-01-01 03:20:10+00'
);

INSERT INTO chapter_plans (
    id, project_id, chapter_no, title, summary, chapter_goal,
    creation_notes, status, source, mock_generation_run_id,
    confirmed_at, created_by, version, created_at, updated_at,
    current_revision_id, source_candidate_id,
    source_candidate_batch_id, source_workflow_run_id
) VALUES
(
    '60000000-0000-4000-8000-000000000002',
    '10000000-0000-4000-8000-000000000001',
    2,
    'The Red Index Card',
    'A red card predicts tomorrow''s archive incident.',
    'Escalate the mystery and connect the child storyline.',
    'Adopted from a deterministic candidate.',
    'confirmed',
    'candidate_adopted',
    NULL,
    '2026-01-01 03:21:00+00',
    'fixture-system',
    1,
    '2026-01-01 03:20:30+00',
    '2026-01-01 03:21:00+00',
    NULL,
    '63000000-0000-4000-8000-000000000001',
    '62000000-0000-4000-8000-000000000001',
    '50000000-0000-4000-8000-000000000001'
);

INSERT INTO chapter_plan_revisions (
    id, chapter_plan_id, project_id, revision_no, snapshot,
    change_type, source_candidate_id, source_candidate_batch_id,
    source_workflow_run_id, created_by, created_at
) VALUES
(
    '61000000-0000-4000-8000-000000000002',
    '60000000-0000-4000-8000-000000000002',
    '10000000-0000-4000-8000-000000000001',
    1,
    '{"chapterNo":2,"title":"The Red Index Card","summary":"A red card predicts tomorrow''s archive incident.","chapterPurpose":"escalation","storylineRefs":[{"id":"40000000-0000-4000-8000-000000000001","relation":"primary","position":0}],"materialRefs":[{"id":"30000000-0000-4000-8000-000000000002","position":0}],"foreshadowingRefs":[{"id":"41000000-0000-4000-8000-000000000001","position":0}],"generationBasis":{"contextSummary":"fixture","additionalInstructions":"Continue the archive mystery."}}'::jsonb,
    'candidate_adopt',
    '63000000-0000-4000-8000-000000000001',
    '62000000-0000-4000-8000-000000000001',
    '50000000-0000-4000-8000-000000000001',
    'fixture-system',
    '2026-01-01 03:20:40+00'
);

UPDATE chapter_plans
SET current_revision_id = '61000000-0000-4000-8000-000000000002'
WHERE id = '60000000-0000-4000-8000-000000000002';

UPDATE chapter_plan_candidates
SET status = 'adopted',
    adopted_chapter_plan_id = '60000000-0000-4000-8000-000000000002',
    adopted_revision_id = '61000000-0000-4000-8000-000000000002',
    adopted_at = '2026-01-01 03:21:00+00',
    updated_at = '2026-01-01 03:21:00+00',
    version = 2
WHERE id = '63000000-0000-4000-8000-000000000001';

INSERT INTO chapter_plan_storylines (
    chapter_plan_id, project_id, storyline_id, relation, position
) VALUES
(
    '60000000-0000-4000-8000-000000000002',
    '10000000-0000-4000-8000-000000000001',
    '40000000-0000-4000-8000-000000000001',
    'primary',
    0
),
(
    '60000000-0000-4000-8000-000000000002',
    '10000000-0000-4000-8000-000000000001',
    '40000000-0000-4000-8000-000000000002',
    'secondary',
    1
);

INSERT INTO chapter_plan_materials (
    chapter_plan_id, project_id, material_id, position
) VALUES
(
    '60000000-0000-4000-8000-000000000002',
    '10000000-0000-4000-8000-000000000001',
    '30000000-0000-4000-8000-000000000002',
    0
);

INSERT INTO chapter_plan_foreshadowings (
    chapter_plan_id, project_id, foreshadowing_id, position
) VALUES
(
    '60000000-0000-4000-8000-000000000002',
    '10000000-0000-4000-8000-000000000001',
    '41000000-0000-4000-8000-000000000001',
    0
);

INSERT INTO chapter_plan_result_consumptions (
    workflow_run_id, project_id, status, failure_code, safe_reason,
    retry_action, candidate_batch_id, consumed_at, version,
    created_at, updated_at
) VALUES
(
    '50000000-0000-4000-8000-000000000001',
    '10000000-0000-4000-8000-000000000001',
    'consumed',
    NULL,
    NULL,
    NULL,
    '62000000-0000-4000-8000-000000000001',
    '2026-01-01 03:21:00+00',
    1,
    '2026-01-01 03:20:55+00',
    '2026-01-01 03:21:00+00'
);

-- ---------------------------------------------------------------------------
-- Content item A and workflow-generated versions
-- ---------------------------------------------------------------------------
INSERT INTO content_items (
    id, project_id, chapter_plan_id, title, status,
    current_version_id, version, reviewed_at, created_at, updated_at
) VALUES
(
    '70000000-0000-4000-8000-000000000001',
    '10000000-0000-4000-8000-000000000001',
    '60000000-0000-4000-8000-000000000002',
    'The Red Index Card',
    'reviewed',
    '71000000-0000-4000-8000-000000000003',
    3,
    '2026-01-01 06:10:00+00',
    '2026-01-01 04:00:00+00',
    '2026-01-01 06:20:00+00'
),
(
    '70000000-0000-4000-8000-000000000002',
    '10000000-0000-4000-8000-000000000002',
    '60000000-0000-4000-8000-000000000003',
    'The Last Delivery',
    'draft',
    '71000000-0000-4000-8000-000000000004',
    1,
    NULL,
    '2026-01-01 04:01:00+00',
    '2026-01-01 04:01:00+00'
);

INSERT INTO content_versions (
    id, content_item_id, version_no, title, content, summary,
    word_count, source, status, generation_parameters, version,
    frozen_at, created_at, updated_at, source_content_version_id,
    source_content_version_version, source_workflow_run_id
) VALUES
(
    '71000000-0000-4000-8000-000000000001',
    '70000000-0000-4000-8000-000000000001',
    1,
    'The Red Index Card',
    'Lin An opened the archive drawer and found a red index card dated tomorrow.',
    'Manual baseline draft.',
    14,
    'manual_created',
    'frozen',
    '{"fixture":true}'::jsonb,
    1,
    '2026-01-01 04:05:00+00',
    '2026-01-01 04:00:10+00',
    '2026-01-01 04:05:00+00',
    NULL,
    NULL,
    NULL
),
(
    '71000000-0000-4000-8000-000000000004',
    '70000000-0000-4000-8000-000000000002',
    1,
    'The Last Delivery',
    'The courier checked the sealed envelope and started the motorcycle.',
    'Secondary project baseline.',
    10,
    'manual_created',
    'editable_draft',
    '{"fixture":true}'::jsonb,
    1,
    NULL,
    '2026-01-01 04:01:10+00',
    '2026-01-01 04:01:10+00',
    NULL,
    NULL,
    NULL
);

INSERT INTO workflow_run_records (
    id, run_number, project_id, stage, workflow_configuration_id,
    trigger_source, status, configuration_snapshot, input_payload,
    output_payload, error_code, error_message, error_details,
    retry_of_run_id, started_at, finished_at, cancelled_at,
    created_at, updated_at, version, subject_type, subject_id
) VALUES
(
    '50000000-0000-4000-8000-000000000002',
    'ACF-FIX-CG-0001',
    '10000000-0000-4000-8000-000000000001',
    'content_generation',
    '22000000-0000-4000-8000-000000000002',
    'manual',
    'succeeded',
    '{"stage":"content_generation","workflowConfiguration":{"inputContractVersion":"content-generation.input.v1","outputContractVersion":"content-generation.output.v1"}}'::jsonb,
    '{"schemaVersion":"content-generation.input.v1","projectId":"10000000-0000-4000-8000-000000000001","contentItemId":"70000000-0000-4000-8000-000000000001","sourceContentVersionId":"71000000-0000-4000-8000-000000000001"}'::jsonb,
    '{"schemaVersion":"content-generation.output.v1","title":"The Red Index Card"}'::jsonb,
    NULL,
    NULL,
    NULL,
    NULL,
    '2026-01-01 04:10:10+00',
    '2026-01-01 04:10:50+00',
    NULL,
    '2026-01-01 04:10:00+00',
    '2026-01-01 04:10:50+00',
    1,
    'content_item',
    '70000000-0000-4000-8000-000000000001'
);

INSERT INTO workflow_run_events (
    id, run_id, event_type, status, payload, created_at
) VALUES
(
    '51000000-0000-4000-8000-000000000005',
    '50000000-0000-4000-8000-000000000002',
    'queued',
    'queued',
    '{"fixture":true}'::jsonb,
    '2026-01-01 04:10:00+00'
),
(
    '51000000-0000-4000-8000-000000000006',
    '50000000-0000-4000-8000-000000000002',
    'worker_started',
    'running',
    '{"fixture":true}'::jsonb,
    '2026-01-01 04:10:10+00'
),
(
    '51000000-0000-4000-8000-000000000007',
    '50000000-0000-4000-8000-000000000002',
    'result_consumed',
    'running',
    '{"contentVersionId":"71000000-0000-4000-8000-000000000002"}'::jsonb,
    '2026-01-01 04:10:45+00'
),
(
    '51000000-0000-4000-8000-000000000008',
    '50000000-0000-4000-8000-000000000002',
    'succeeded',
    'succeeded',
    '{"fixture":true}'::jsonb,
    '2026-01-01 04:10:50+00'
);

INSERT INTO content_versions (
    id, content_item_id, version_no, title, content, summary,
    word_count, source, status, generation_parameters, version,
    frozen_at, created_at, updated_at, source_content_version_id,
    source_content_version_version, source_workflow_run_id
) VALUES
(
    '71000000-0000-4000-8000-000000000002',
    '70000000-0000-4000-8000-000000000001',
    2,
    'The Red Index Card',
    'Lin An opened the archive drawer. A red index card carried tomorrow''s date and described a fire that had not yet begun.',
    'Workflow-generated draft.',
    23,
    'workflow_generated',
    'editable_draft',
    '{"fixture":true,"temperature":0.3}'::jsonb,
    1,
    NULL,
    '2026-01-01 04:10:45+00',
    '2026-01-01 04:10:45+00',
    '71000000-0000-4000-8000-000000000001',
    1,
    '50000000-0000-4000-8000-000000000002'
);

-- ---------------------------------------------------------------------------
-- Runtime review
-- ---------------------------------------------------------------------------
INSERT INTO workflow_run_records (
    id, run_number, project_id, stage, workflow_configuration_id,
    trigger_source, status, configuration_snapshot, input_payload,
    output_payload, error_code, error_message, error_details,
    retry_of_run_id, started_at, finished_at, cancelled_at,
    created_at, updated_at, version, subject_type, subject_id
) VALUES
(
    '50000000-0000-4000-8000-000000000003',
    'ACF-FIX-RV-0001',
    '10000000-0000-4000-8000-000000000001',
    'review',
    '22000000-0000-4000-8000-000000000003',
    'manual',
    'succeeded',
    '{"stage":"review","workflowConfiguration":{"inputContractVersion":"review.input.v1","outputContractVersion":"review.output.v1"}}'::jsonb,
    '{"schemaVersion":"review.input.v1","projectId":"10000000-0000-4000-8000-000000000001","contentItemId":"70000000-0000-4000-8000-000000000001","contentVersionId":"71000000-0000-4000-8000-000000000002"}'::jsonb,
    '{"schemaVersion":"review.output.v1","conclusion":"needs_changes","issueCount":1}'::jsonb,
    NULL,
    NULL,
    NULL,
    NULL,
    '2026-01-01 05:00:10+00',
    '2026-01-01 05:00:50+00',
    NULL,
    '2026-01-01 05:00:00+00',
    '2026-01-01 05:00:50+00',
    1,
    'content_version',
    '71000000-0000-4000-8000-000000000002'
);

INSERT INTO workflow_run_events (
    id, run_id, event_type, status, payload, created_at
) VALUES
(
    '51000000-0000-4000-8000-000000000009',
    '50000000-0000-4000-8000-000000000003',
    'queued',
    'queued',
    '{"fixture":true}'::jsonb,
    '2026-01-01 05:00:00+00'
),
(
    '51000000-0000-4000-8000-000000000010',
    '50000000-0000-4000-8000-000000000003',
    'worker_started',
    'running',
    '{"fixture":true}'::jsonb,
    '2026-01-01 05:00:10+00'
),
(
    '51000000-0000-4000-8000-000000000011',
    '50000000-0000-4000-8000-000000000003',
    'output_validated',
    'running',
    '{"fixture":true}'::jsonb,
    '2026-01-01 05:00:40+00'
),
(
    '51000000-0000-4000-8000-000000000012',
    '50000000-0000-4000-8000-000000000003',
    'succeeded',
    'succeeded',
    '{"fixture":true}'::jsonb,
    '2026-01-01 05:00:50+00'
);

INSERT INTO review_reports (
    id, project_id, content_item_id, content_version_id,
    workflow_run_id, provider_key, status, conclusion, score,
    summary, created_at, completed_at, schema_version,
    source_content_version_version, source_content_hash
) VALUES
(
    '80000000-0000-4000-8000-000000000001',
    '10000000-0000-4000-8000-000000000001',
    '70000000-0000-4000-8000-000000000001',
    '71000000-0000-4000-8000-000000000002',
    '50000000-0000-4000-8000-000000000003',
    'runtime',
    'completed',
    'needs_changes',
    NULL,
    'The reveal arrives too quickly and needs one additional beat.',
    '2026-01-01 05:00:45+00',
    '2026-01-01 05:00:50+00',
    'review.output.v1',
    1,
    repeat('b', 64)
);

INSERT INTO review_findings (
    id, review_id, category, severity, title, description,
    location_json, sort_order, created_at, issue_key,
    category_label, evidence_json, suggestion, disposition,
    version, ignored_at, ignored_by, updated_at
) VALUES
(
    '81000000-0000-4000-8000-000000000001',
    '80000000-0000-4000-8000-000000000001',
    'pacing',
    'warning',
    'Reveal needs one more beat',
    'The card''s prediction is accepted before Lin An tests its authenticity.',
    '{"paragraphStart":1,"paragraphEnd":1}'::jsonb,
    1,
    '2026-01-01 05:00:46+00',
    'ISSUE-001',
    'Pacing',
    '{"quote":"A red index card carried tomorrow''s date."}'::jsonb,
    'Add a short verification action before the prediction is trusted.',
    'open',
    1,
    NULL,
    NULL,
    '2026-01-01 05:00:46+00'
);

INSERT INTO review_recommendations (
    id, review_id, priority, title, description,
    sort_order, created_at
) VALUES
(
    '82000000-0000-4000-8000-000000000001',
    '80000000-0000-4000-8000-000000000001',
    'high',
    'Add a verification beat',
    'Have Lin An compare the card with the sealed incident ledger.',
    0,
    '2026-01-01 05:00:47+00'
);

-- Legacy mock review path remains covered.
INSERT INTO workflow_runs (
    id, project_id, content_item_id, content_version_id,
    provider_key, workflow_key, subject_type, subject_id,
    status, idempotency_key, request_fingerprint,
    input_json, output_json, error_code, error_summary,
    started_at, finished_at, created_at, updated_at,
    target_content_version_id, source_review_report_id
) VALUES
(
    '83000000-0000-4000-8000-000000000001',
    '10000000-0000-4000-8000-000000000001',
    '70000000-0000-4000-8000-000000000001',
    '71000000-0000-4000-8000-000000000001',
    'mock',
    'content_mock_review',
    'content_item',
    '70000000-0000-4000-8000-000000000001',
    'succeeded',
    'acf-fixture-mock-review',
    repeat('c', 64),
    '{"fixture":true}'::jsonb,
    '{"conclusion":"pass"}'::jsonb,
    NULL,
    NULL,
    '2026-01-01 05:10:10+00',
    '2026-01-01 05:10:20+00',
    '2026-01-01 05:10:00+00',
    '2026-01-01 05:10:20+00',
    NULL,
    NULL
);

INSERT INTO review_reports (
    id, project_id, content_item_id, content_version_id,
    workflow_run_id, provider_key, status, conclusion, score,
    summary, created_at, completed_at, schema_version,
    source_content_version_version, source_content_hash
) VALUES
(
    '80000000-0000-4000-8000-000000000002',
    '10000000-0000-4000-8000-000000000001',
    '70000000-0000-4000-8000-000000000001',
    '71000000-0000-4000-8000-000000000001',
    '83000000-0000-4000-8000-000000000001',
    'mock',
    'completed',
    'pass',
    92,
    'Legacy mock review fixture.',
    '2026-01-01 05:10:15+00',
    '2026-01-01 05:10:20+00',
    NULL,
    NULL,
    NULL
);

INSERT INTO review_findings (
    id, review_id, category, severity, title, description,
    location_json, sort_order, created_at, issue_key,
    category_label, evidence_json, suggestion, disposition,
    version, ignored_at, ignored_by, updated_at
) VALUES
(
    '81000000-0000-4000-8000-000000000002',
    '80000000-0000-4000-8000-000000000002',
    'world_consistency',
    'low',
    'Minor terminology consistency',
    'Use the full archive name on first mention.',
    NULL,
    0,
    '2026-01-01 05:10:16+00',
    NULL,
    NULL,
    NULL,
    NULL,
    'open',
    1,
    NULL,
    NULL,
    '2026-01-01 05:10:16+00'
);

-- ---------------------------------------------------------------------------
-- Runtime rewrite and workflow_rewrite candidate
-- ---------------------------------------------------------------------------
INSERT INTO workflow_run_records (
    id, run_number, project_id, stage, workflow_configuration_id,
    trigger_source, status, configuration_snapshot, input_payload,
    output_payload, error_code, error_message, error_details,
    retry_of_run_id, started_at, finished_at, cancelled_at,
    created_at, updated_at, version, subject_type, subject_id
) VALUES
(
    '50000000-0000-4000-8000-000000000004',
    'ACF-FIX-RW-0001',
    '10000000-0000-4000-8000-000000000001',
    'rewrite',
    '22000000-0000-4000-8000-000000000004',
    'manual',
    'succeeded',
    '{"stage":"rewrite","workflowConfiguration":{"inputContractVersion":"rewrite.input.v1","outputContractVersion":"rewrite.output.v1"}}'::jsonb,
    '{"schemaVersion":"rewrite.input.v1","projectId":"10000000-0000-4000-8000-000000000001","contentItemId":"70000000-0000-4000-8000-000000000001","sourceContentVersionId":"71000000-0000-4000-8000-000000000002","reviewReportId":"80000000-0000-4000-8000-000000000001","selectedIssues":[{"reviewIssueId":"81000000-0000-4000-8000-000000000001","reviewReportId":"80000000-0000-4000-8000-000000000001","disposition":"open"}]}'::jsonb,
    '{"schemaVersion":"rewrite.output.v1","candidateCount":1}'::jsonb,
    NULL,
    NULL,
    NULL,
    NULL,
    '2026-01-01 06:00:10+00',
    '2026-01-01 06:00:50+00',
    NULL,
    '2026-01-01 06:00:00+00',
    '2026-01-01 06:00:50+00',
    1,
    'review_report',
    '80000000-0000-4000-8000-000000000001'
);

INSERT INTO workflow_run_events (
    id, run_id, event_type, status, payload, created_at
) VALUES
(
    '51000000-0000-4000-8000-000000000013',
    '50000000-0000-4000-8000-000000000004',
    'queued',
    'queued',
    '{"fixture":true}'::jsonb,
    '2026-01-01 06:00:00+00'
),
(
    '51000000-0000-4000-8000-000000000014',
    '50000000-0000-4000-8000-000000000004',
    'worker_started',
    'running',
    '{"fixture":true}'::jsonb,
    '2026-01-01 06:00:10+00'
),
(
    '51000000-0000-4000-8000-000000000015',
    '50000000-0000-4000-8000-000000000004',
    'result_consumed',
    'running',
    '{"contentVersionId":"71000000-0000-4000-8000-000000000003"}'::jsonb,
    '2026-01-01 06:00:45+00'
),
(
    '51000000-0000-4000-8000-000000000016',
    '50000000-0000-4000-8000-000000000004',
    'succeeded',
    'succeeded',
    '{"fixture":true}'::jsonb,
    '2026-01-01 06:00:50+00'
);

INSERT INTO content_versions (
    id, content_item_id, version_no, title, content, summary,
    word_count, source, status, generation_parameters, version,
    frozen_at, created_at, updated_at, source_content_version_id,
    source_content_version_version, source_workflow_run_id
) VALUES
(
    '71000000-0000-4000-8000-000000000003',
    '70000000-0000-4000-8000-000000000001',
    3,
    'The Red Index Card',
    'Lin An opened the archive drawer and found a red index card dated tomorrow. Before trusting it, she matched its seal against the locked incident ledger.',
    'Workflow rewrite candidate addressing the pacing issue.',
    27,
    'workflow_rewrite',
    'editable_draft',
    '{"fixture":true,"selectedIssueIds":["81000000-0000-4000-8000-000000000001"]}'::jsonb,
    1,
    NULL,
    '2026-01-01 06:00:45+00',
    '2026-01-01 06:00:45+00',
    '71000000-0000-4000-8000-000000000002',
    1,
    '50000000-0000-4000-8000-000000000004'
);

-- ---------------------------------------------------------------------------
-- Additional runtime statuses on Project B
-- ---------------------------------------------------------------------------
INSERT INTO workflow_run_records (
    id, run_number, project_id, stage, workflow_configuration_id,
    trigger_source, status, configuration_snapshot, input_payload,
    output_payload, error_code, error_message, error_details,
    retry_of_run_id, started_at, finished_at, cancelled_at,
    created_at, updated_at, version, subject_type, subject_id
) VALUES
(
    '50000000-0000-4000-8000-000000000005',
    'ACF-FIX-QUEUED-0001',
    '10000000-0000-4000-8000-000000000002',
    'chapter_planning',
    '22000000-0000-4000-8000-000000000001',
    'manual',
    'queued',
    '{"stage":"chapter_planning"}'::jsonb,
    '{"fixture":true}'::jsonb,
    NULL,
    NULL,
    NULL,
    NULL,
    NULL,
    NULL,
    NULL,
    NULL,
    '2026-01-01 07:00:00+00',
    '2026-01-01 07:00:00+00',
    1,
    NULL,
    NULL
),
(
    '50000000-0000-4000-8000-000000000006',
    'ACF-FIX-RUNNING-0001',
    '10000000-0000-4000-8000-000000000002',
    'review',
    '22000000-0000-4000-8000-000000000003',
    'manual',
    'running',
    '{"stage":"review"}'::jsonb,
    '{"fixture":true}'::jsonb,
    NULL,
    NULL,
    NULL,
    NULL,
    NULL,
    '2026-01-01 07:10:10+00',
    NULL,
    NULL,
    '2026-01-01 07:10:00+00',
    '2026-01-01 07:10:10+00',
    1,
    'content_version',
    '71000000-0000-4000-8000-000000000004'
),
(
    '50000000-0000-4000-8000-000000000007',
    'ACF-FIX-FAILED-0001',
    '10000000-0000-4000-8000-000000000002',
    'content_generation',
    '22000000-0000-4000-8000-000000000002',
    'manual',
    'failed',
    '{"stage":"content_generation"}'::jsonb,
    '{"fixture":true}'::jsonb,
    NULL,
    'fixture_failure',
    'Intentional deterministic fixture failure.',
    '{"retryable":true}'::jsonb,
    NULL,
    '2026-01-01 07:20:10+00',
    '2026-01-01 07:20:20+00',
    NULL,
    '2026-01-01 07:20:00+00',
    '2026-01-01 07:20:20+00',
    1,
    'content_item',
    '70000000-0000-4000-8000-000000000002'
),
(
    '50000000-0000-4000-8000-000000000008',
    'ACF-FIX-CANCELLED-0001',
    '10000000-0000-4000-8000-000000000002',
    'chapter_planning',
    '22000000-0000-4000-8000-000000000001',
    'manual',
    'cancelled',
    '{"stage":"chapter_planning"}'::jsonb,
    '{"fixture":true}'::jsonb,
    NULL,
    NULL,
    NULL,
    NULL,
    NULL,
    '2026-01-01 07:30:10+00',
    NULL,
    '2026-01-01 07:30:20+00',
    '2026-01-01 07:30:00+00',
    '2026-01-01 07:30:20+00',
    1,
    NULL,
    NULL
);

INSERT INTO workflow_run_records (
    id, run_number, project_id, stage, workflow_configuration_id,
    trigger_source, status, configuration_snapshot, input_payload,
    output_payload, retry_of_run_id, retry_mode, retryability,
    external_execution_id, started_at, finished_at,
    created_at, updated_at, version, subject_type, subject_id
) VALUES (
    '50000000-0000-4000-8000-000000000009',
    'ACF-FIX-CG-RETRY-0001',
    '10000000-0000-4000-8000-000000000002',
    'content_generation',
    '22000000-0000-4000-8000-000000000002',
    'retry',
    'succeeded',
    '{"id":"22000000-0000-4000-8000-000000000002","version":1,"connection":{"id":"21000000-0000-4000-8000-000000000001"},"inputContractVersion":"content-generation.input.v1","outputContractVersion":"content-generation.output.v1"}'::jsonb,
    '{"schemaVersion":"content-generation.input.v1","projectId":"10000000-0000-4000-8000-000000000002","retryOfRunId":"50000000-0000-4000-8000-000000000007"}'::jsonb,
    '{"schemaVersion":"content-generation.output.v1","title":"Recovered fixture"}'::jsonb,
    '50000000-0000-4000-8000-000000000007',
    'original_configuration',
    'not_retryable',
    'fixture-execution-retry-0001',
    '2026-01-01 07:40:10+00',
    '2026-01-01 07:40:20+00',
    '2026-01-01 07:40:00+00',
    '2026-01-01 07:40:20+00',
    1,
    'content_item',
    '70000000-0000-4000-8000-000000000002'
);

UPDATE workflow_run_records
SET failure_phase = 'external_execution',
    failure_code = 'fixture_failure',
    safe_error_message = 'Intentional deterministic fixture failure.',
    retryability = 'runtime_retry'
WHERE id = '50000000-0000-4000-8000-000000000007';

UPDATE workflow_run_records run
SET configuration_snapshot = jsonb_build_object(
        'stage', run.stage,
        'workflowConnection', jsonb_build_object(
            'id', connection.id,
            'name', connection.name,
            'version', connection.version,
            'type', connection.connection_type
        ),
        'workflowConfiguration', jsonb_build_object(
            'id', workflow.id,
            'name', workflow.name,
            'version', workflow.version,
            'reference', workflow.type_config,
            'inputContractVersion', workflow.input_contract_version,
            'outputContractVersion', workflow.output_contract_version,
            'defaultParameters', workflow.default_parameters
        )
    ),
    binding_snapshot = jsonb_build_object(
        'bindingId', binding.id,
        'bindingVersion', binding.version,
        'stage', run.stage
    ),
    connection_snapshot = jsonb_build_object(
        'id', connection.id,
        'name', connection.name,
        'version', connection.version,
        'type', connection.connection_type,
        'baseUrl', connection.base_url,
        'authType', connection.auth_type,
        'credentialFingerprint', connection.credential_fingerprint
    ),
    llm_policy_snapshot = jsonb_strip_nulls(jsonb_build_object(
        'strategy', workflow.llm_strategy,
        'providerId', provider.id,
        'providerName', provider.name,
        'providerVersion', provider.version,
        'model', workflow.llm_model,
        'secretFingerprint', provider.secret_fingerprint
    ))
FROM workflow_configurations workflow
JOIN workflow_connections connection ON connection.id = workflow.connection_id
LEFT JOIN llm_provider_configurations provider ON provider.id = workflow.llm_provider_id
CROSS JOIN project_workflow_bindings binding
WHERE workflow.id = run.workflow_configuration_id
  AND binding.project_id = run.project_id
  AND binding.stage = run.stage
  AND binding.workflow_configuration_id = workflow.id;

INSERT INTO workflow_run_events (
    id, run_id, event_type, status, payload, created_at
) VALUES
(
    '51000000-0000-4000-8000-000000000017',
    '50000000-0000-4000-8000-000000000005',
    'queued',
    'queued',
    '{"fixture":true}'::jsonb,
    '2026-01-01 07:00:00+00'
),
(
    '51000000-0000-4000-8000-000000000018',
    '50000000-0000-4000-8000-000000000006',
    'queued',
    'queued',
    '{"fixture":true}'::jsonb,
    '2026-01-01 07:10:00+00'
),
(
    '51000000-0000-4000-8000-000000000019',
    '50000000-0000-4000-8000-000000000006',
    'worker_started',
    'running',
    '{"fixture":true}'::jsonb,
    '2026-01-01 07:10:10+00'
),
(
    '51000000-0000-4000-8000-000000000020',
    '50000000-0000-4000-8000-000000000007',
    'queued',
    'queued',
    '{"fixture":true}'::jsonb,
    '2026-01-01 07:20:00+00'
),
(
    '51000000-0000-4000-8000-000000000021',
    '50000000-0000-4000-8000-000000000007',
    'worker_started',
    'running',
    '{"fixture":true}'::jsonb,
    '2026-01-01 07:20:10+00'
),
(
    '51000000-0000-4000-8000-000000000022',
    '50000000-0000-4000-8000-000000000007',
    'failed',
    'failed',
    '{"errorCode":"fixture_failure"}'::jsonb,
    '2026-01-01 07:20:20+00'
),
(
    '51000000-0000-4000-8000-000000000023',
    '50000000-0000-4000-8000-000000000008',
    'queued',
    'queued',
    '{"fixture":true}'::jsonb,
    '2026-01-01 07:30:00+00'
),
(
    '51000000-0000-4000-8000-000000000024',
    '50000000-0000-4000-8000-000000000008',
    'worker_started',
    'running',
    '{"fixture":true}'::jsonb,
    '2026-01-01 07:30:10+00'
),
(
    '51000000-0000-4000-8000-000000000025',
    '50000000-0000-4000-8000-000000000008',
    'cancelled',
    'cancelled',
    '{"fixture":true}'::jsonb,
    '2026-01-01 07:30:20+00'
),
(
    '51000000-0000-4000-8000-000000000026',
    '50000000-0000-4000-8000-000000000009',
    'retry_created',
    'queued',
    '{"retryOfRunId":"50000000-0000-4000-8000-000000000007","retryMode":"original_configuration"}'::jsonb,
    '2026-01-01 07:40:00+00'
),
(
    '51000000-0000-4000-8000-000000000027',
    '50000000-0000-4000-8000-000000000009',
    'execution_started',
    'running',
    '{"externalExecutionId":"fixture-execution-retry-0001"}'::jsonb,
    '2026-01-01 07:40:10+00'
),
(
    '51000000-0000-4000-8000-000000000028',
    '50000000-0000-4000-8000-000000000009',
    'succeeded',
    'succeeded',
    '{"fixture":true}'::jsonb,
    '2026-01-01 07:40:20+00'
);

-- ---------------------------------------------------------------------------
-- Idempotency and audit
-- ---------------------------------------------------------------------------
INSERT INTO idempotency_records (
    id, scope, idempotency_key, request_hash,
    response_status, response_body, created_at, expires_at
) VALUES
(
    '90000000-0000-4000-8000-000000000001',
    'fixture:create-content-run:10000000-0000-4000-8000-000000000001',
    'acf-fixture-success',
    repeat('d', 64),
    201,
    '{"runId":"50000000-0000-4000-8000-000000000002"}'::jsonb,
    '2026-01-01 08:00:00+00',
    '2026-02-01 08:00:00+00'
),
(
    '90000000-0000-4000-8000-000000000002',
    'fixture:create-content-run:10000000-0000-4000-8000-000000000002',
    'acf-fixture-failure',
    repeat('e', 64),
    422,
    '{"error":{"code":"fixture_validation_error"}}'::jsonb,
    '2026-01-01 08:01:00+00',
    '2026-02-01 08:01:00+00'
);

INSERT INTO audit_logs (
    id, actor_id, action, subject_type, subject_id, payload, created_at
) VALUES
(
    '91000000-0000-4000-8000-000000000001',
    'fixture-system',
    'fixture.project.created',
    'project',
    '10000000-0000-4000-8000-000000000001',
    '{"fixture":true}'::jsonb,
    '2026-01-01 08:10:00+00'
),
(
    '91000000-0000-4000-8000-000000000002',
    'fixture-system',
    'fixture.rewrite.completed',
    'workflow_run',
    '50000000-0000-4000-8000-000000000004',
    '{"fixture":true}'::jsonb,
    '2026-01-01 08:11:00+00'
);

-- ---------------------------------------------------------------------------
-- Deterministic integrity assertions
-- ---------------------------------------------------------------------------
DO $$
DECLARE
    anomaly_count BIGINT;
    fixture_table TEXT;
    fixture_row_count BIGINT;
BEGIN
    SELECT COUNT(*)
    INTO anomaly_count
    FROM chapter_plans
    WHERE current_revision_id IS NULL;

    IF anomaly_count <> 0 THEN
        RAISE EXCEPTION 'Fixture assertion failed: chapter_plans with NULL current_revision_id = %', anomaly_count;
    END IF;

    SELECT COUNT(*)
    INTO anomaly_count
    FROM (
        SELECT adopted_chapter_plan_id
        FROM chapter_plan_candidates
        WHERE adopted_chapter_plan_id IS NOT NULL
        GROUP BY adopted_chapter_plan_id
        HAVING COUNT(*) > 1
    ) duplicate_adoptions;

    IF anomaly_count <> 0 THEN
        RAISE EXCEPTION 'Fixture assertion failed: duplicate adopted_chapter_plan_id groups = %', anomaly_count;
    END IF;

    SELECT COUNT(*)
    INTO anomaly_count
    FROM workflow_run_records
    WHERE finished_at IS NOT NULL
      AND finished_at < created_at;

    IF anomaly_count <> 0 THEN
        RAISE EXCEPTION 'Fixture assertion failed: workflow runs finished before creation = %', anomaly_count;
    END IF;

    SELECT COUNT(*)
    INTO anomaly_count
    FROM workflow_run_events event
    JOIN workflow_run_records run ON run.id = event.run_id
    WHERE event.created_at < run.created_at;

    IF anomaly_count <> 0 THEN
        RAISE EXCEPTION 'Fixture assertion failed: workflow events before run creation = %', anomaly_count;
    END IF;

    SELECT COUNT(*)
    INTO anomaly_count
    FROM workflow_configurations
    WHERE llm_strategy NOT IN ('acf_managed', 'n8n_managed', 'none')
       OR (llm_strategy = 'acf_managed' AND (llm_provider_id IS NULL OR llm_model IS NULL))
       OR (llm_strategy IN ('n8n_managed', 'none') AND (llm_provider_id IS NOT NULL OR llm_model IS NOT NULL));

    IF anomaly_count <> 0 THEN
        RAISE EXCEPTION 'Fixture assertion failed: invalid workflow LLM strategies = %', anomaly_count;
    END IF;

    SELECT COUNT(*)
    INTO anomaly_count
    FROM workflow_run_records
    WHERE retry_of_run_id = id
       OR jsonb_typeof(binding_snapshot) <> 'object'
       OR jsonb_typeof(connection_snapshot) <> 'object'
       OR jsonb_typeof(llm_policy_snapshot) <> 'object';

    IF anomaly_count <> 0 THEN
        RAISE EXCEPTION 'Fixture assertion failed: invalid retry links or snapshots = %', anomaly_count;
    END IF;

    FOREACH fixture_table IN ARRAY ARRAY[
        'audit_logs',
        'projects',
        'project_plannings',
        'materials',
        'project_material_usages',
        'idempotency_records',
        'storylines',
        'foreshadowings',
        'mock_generation_runs',
        'chapter_plans',
        'chapter_plan_storylines',
        'chapter_plan_materials',
        'chapter_plan_foreshadowings',
        'content_items',
        'content_versions',
        'workflow_runs',
        'review_reports',
        'review_findings',
        'review_recommendations',
        'llm_provider_configurations',
        'llm_provider_models',
        'workflow_connections',
        'workflow_configurations',
        'distribution_platform_configurations',
        'project_workflow_bindings',
        'workflow_run_records',
        'workflow_run_events',
        'chapter_plan_candidate_batches',
        'chapter_plan_candidates',
        'chapter_plan_revisions',
        'chapter_plan_result_consumptions'
    ] LOOP
        EXECUTE format('SELECT COUNT(*) FROM public.%I', fixture_table)
        INTO fixture_row_count;

        IF fixture_row_count = 0 THEN
            RAISE EXCEPTION 'Fixture assertion failed: table % is empty', fixture_table;
        END IF;
    END LOOP;
END
$$;

COMMIT;
