CREATE TABLE llm_provider_configurations (
 id UUID PRIMARY KEY, name VARCHAR(120) NOT NULL UNIQUE, provider_type VARCHAR(40) NOT NULL,
 base_url VARCHAR(512) NOT NULL, default_model VARCHAR(160) NOT NULL, encrypted_secret TEXT,
 secret_fingerprint VARCHAR(32), timeout_seconds INTEGER NOT NULL CHECK(timeout_seconds BETWEEN 5 AND 300),
 integration_status VARCHAR(20) NOT NULL DEFAULT 'not_connected', enabled BOOLEAN NOT NULL DEFAULT FALSE,
 last_verified_at TIMESTAMPTZ, last_error_code VARCHAR(80), last_error_message VARCHAR(300), version INTEGER NOT NULL DEFAULT 1,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 CHECK(provider_type='openai_compatible')
);
CREATE TABLE workflow_connections (
 id UUID PRIMARY KEY, name VARCHAR(120) NOT NULL UNIQUE, connection_type VARCHAR(40) NOT NULL,
 base_url VARCHAR(512) NOT NULL, auth_type VARCHAR(40) NOT NULL, encrypted_credential TEXT,
 credential_fingerprint VARCHAR(32), timeout_seconds INTEGER NOT NULL CHECK(timeout_seconds BETWEEN 5 AND 300), type_config JSONB NOT NULL,
 integration_status VARCHAR(20) NOT NULL DEFAULT 'not_connected', enabled BOOLEAN NOT NULL DEFAULT FALSE,
 last_verified_at TIMESTAMPTZ, last_error_code VARCHAR(80), last_error_message VARCHAR(300), version INTEGER NOT NULL DEFAULT 1,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 CHECK(connection_type='n8n'), CHECK(auth_type='api_key')
);
CREATE TABLE workflow_configurations (
 id UUID PRIMARY KEY, name VARCHAR(160) NOT NULL UNIQUE, connection_id UUID NOT NULL REFERENCES workflow_connections(id),
 applicable_stages JSONB NOT NULL, type_config JSONB NOT NULL, input_contract_version VARCHAR(40) NOT NULL,
 output_contract_version VARCHAR(40) NOT NULL, default_parameters JSONB NOT NULL DEFAULT '{}'::jsonb, note TEXT,
 integration_status VARCHAR(20) NOT NULL DEFAULT 'not_connected', enabled BOOLEAN NOT NULL DEFAULT FALSE,
 last_verified_at TIMESTAMPTZ, last_error_code VARCHAR(80), last_error_message VARCHAR(300), version INTEGER NOT NULL DEFAULT 1,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE distribution_platform_configurations (
 id UUID PRIMARY KEY, name VARCHAR(120) NOT NULL UNIQUE, platform_type VARCHAR(60) NOT NULL,
 account_identifier VARCHAR(240) NOT NULL, endpoint_url VARCHAR(512), auth_type VARCHAR(40) NOT NULL,
 encrypted_credential TEXT, credential_fingerprint VARCHAR(32), timeout_seconds INTEGER NOT NULL CHECK(timeout_seconds BETWEEN 5 AND 300),
 type_config JSONB NOT NULL, note TEXT, integration_status VARCHAR(20) NOT NULL DEFAULT 'not_connected', enabled BOOLEAN NOT NULL DEFAULT FALSE,
 last_verified_at TIMESTAMPTZ, last_error_code VARCHAR(80), last_error_message VARCHAR(300), version INTEGER NOT NULL DEFAULT 1,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 CHECK(platform_type IN ('wechat_official_account','douyin','youtube','custom')),
 CHECK(auth_type IN ('api_key','oauth','access_token','custom')),
 CHECK(platform_type <> 'custom' OR endpoint_url IS NOT NULL)
);
CREATE INDEX workflow_configurations_connection_id_idx ON workflow_configurations(connection_id);
