[CmdletBinding()]
param(
    [ValidateSet('Apply', 'Verify')]
    [string]$Mode = 'Apply',
    [string]$DatabaseUrl = 'postgres://postgres:postgres@127.0.0.1:15433/ai_content_factory?sslmode=disable'
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repoRoot = [System.IO.Path]::GetFullPath((& git rev-parse --show-toplevel).Trim())

$nodeScript = @'
const { spawnSync } = require('child_process');
const assert = require('assert');

const mode = process.argv[2];
const repoRoot = process.argv[3];

const applySql = `
BEGIN;
DELETE FROM llm_provider_models WHERE provider_id IN ('11000000-0000-4000-8000-000000000001', '11000000-0000-4000-8000-000000000002', '11000000-0000-4000-8000-000000000003', '11000000-0000-4000-8000-000000000004');
DELETE FROM llm_provider_configurations WHERE name IN ('OpenAI 主模型', '内容审核模型', '备用模型', '本地兼容模型') OR id IN ('11000000-0000-4000-8000-000000000001', '11000000-0000-4000-8000-000000000002', '11000000-0000-4000-8000-000000000003', '11000000-0000-4000-8000-000000000004');

INSERT INTO llm_provider_configurations(id, name, provider_type, base_url, default_model, encrypted_secret, secret_fingerprint, timeout_seconds, integration_status, enabled, last_verified_version, last_verified_at, validation_details, version)
VALUES('11000000-0000-4000-8000-000000000001', 'OpenAI 主模型', 'openai_compatible', 'https://api.openai.com/v1', 'gpt-5.2', 'sealed-secret-1', '4e3b9a1c', 60, 'verified', true, 1, '2026-08-06 08:00:00+00', '{"catalog":"safe"}'::jsonb, 1);

INSERT INTO llm_provider_models(id, provider_id, model_key, source, availability) VALUES
('11000000-0000-4000-9000-000000000001', '11000000-0000-4000-8000-000000000001', 'gpt-5.2', 'discovered', 'available'),
('11000000-0000-4000-9000-000000000002', '11000000-0000-4000-8000-000000000001', 'model-1', 'discovered', 'available'),
('11000000-0000-4000-9000-000000000003', '11000000-0000-4000-8000-000000000001', 'model-2', 'discovered', 'available'),
('11000000-0000-4000-9000-000000000004', '11000000-0000-4000-8000-000000000001', 'model-3', 'discovered', 'available'),
('11000000-0000-4000-9000-000000000005', '11000000-0000-4000-8000-000000000001', 'model-4', 'discovered', 'available'),
('11000000-0000-4000-9000-000000000006', '11000000-0000-4000-8000-000000000001', 'model-5', 'discovered', 'available'),
('11000000-0000-4000-9000-000000000007', '11000000-0000-4000-8000-000000000001', 'model-6', 'discovered', 'available'),
('11000000-0000-4000-9000-000000000008', '11000000-0000-4000-8000-000000000001', 'model-7', 'discovered', 'available'),
('11000000-0000-4000-9000-000000000009', '11000000-0000-4000-8000-000000000001', 'model-8', 'discovered', 'available'),
('11000000-0000-4000-9000-000000000010', '11000000-0000-4000-8000-000000000001', 'model-9', 'discovered', 'available'),
('11000000-0000-4000-9000-000000000011', '11000000-0000-4000-8000-000000000001', 'model-10', 'discovered', 'available'),
('11000000-0000-4000-9000-000000000012', '11000000-0000-4000-8000-000000000001', 'model-11', 'discovered', 'available');

INSERT INTO llm_provider_configurations(id, name, provider_type, base_url, default_model, encrypted_secret, secret_fingerprint, timeout_seconds, integration_status, enabled, last_verified_version, last_verified_at, last_error_message, validation_details, version)
VALUES('11000000-0000-4000-8000-000000000002', '内容审核模型', 'openai_compatible', 'https://api.example.com/v1', 'gpt-4.1', 'sealed-secret-2', '4e3b9a1c', 60, 'failed', false, NULL, '2026-08-05 08:00:00+00', '认证失败，请更新 API Key', '{}'::jsonb, 1);

INSERT INTO llm_provider_models(id, provider_id, model_key, source, availability) VALUES
('11000000-0000-4000-9000-000000000021', '11000000-0000-4000-8000-000000000002', 'gpt-4.1', 'discovered', 'available'),
('11000000-0000-4000-9000-000000000022', '11000000-0000-4000-8000-000000000002', 'audit-model-2', 'discovered', 'available'),
('11000000-0000-4000-9000-000000000023', '11000000-0000-4000-8000-000000000002', 'audit-model-3', 'discovered', 'available');

INSERT INTO llm_provider_configurations(id, name, provider_type, base_url, default_model, encrypted_secret, secret_fingerprint, timeout_seconds, integration_status, enabled, last_verified_version, last_verified_at, last_error_message, validation_details, version)
VALUES('11000000-0000-4000-8000-000000000003', '备用模型', 'openai_compatible', 'https://gateway.example.com/v1', 'gpt-4.1-mini', 'sealed-secret-3', '4e3b9a1c', 60, 'stale', true, 1, '2026-08-04 08:00:00+00', '原验证结果已失效', '{}'::jsonb, 2);

INSERT INTO llm_provider_models(id, provider_id, model_key, source, availability) VALUES
('11000000-0000-4000-9000-000000000031', '11000000-0000-4000-8000-000000000003', 'gpt-4.1-mini', 'discovered', 'available'),
('11000000-0000-4000-9000-000000000032', '11000000-0000-4000-8000-000000000003', 'spare-2', 'discovered', 'available'),
('11000000-0000-4000-9000-000000000033', '11000000-0000-4000-8000-000000000003', 'spare-3', 'discovered', 'available'),
('11000000-0000-4000-9000-000000000034', '11000000-0000-4000-8000-000000000003', 'spare-4', 'discovered', 'available'),
('11000000-0000-4000-9000-000000000035', '11000000-0000-4000-8000-000000000003', 'spare-5', 'discovered', 'available');

INSERT INTO llm_provider_configurations(id, name, provider_type, base_url, default_model, timeout_seconds, integration_status, enabled, version)
VALUES('11000000-0000-4000-8000-000000000004', '本地兼容模型', 'openai_compatible', 'http://localhost:11434/v1', '无', 60, 'unverified', false, 1);

COMMIT;
`;

function execQuery(sql) {
  const dockerRes = spawnSync('docker', ['compose', 'exec', '-T', 'postgres', 'psql', '-U', 'postgres', '-d', 'ai_content_factory'], {
    input: Buffer.from(sql, 'utf-8'),
    encoding: 'utf-8',
    cwd: repoRoot
  });
  if (dockerRes.status === 0) {
    return dockerRes.stdout;
  }

  const res = spawnSync('psql', ['-X', '--set=ON_ERROR_STOP=1', 'postgres://postgres:postgres@127.0.0.1:15433/ai_content_factory?sslmode=disable'], {
    input: Buffer.from(sql, 'utf-8'),
    encoding: 'utf-8',
    env: { ...process.env, PGCLIENTENCODING: 'UTF8' }
  });
  if (res.status !== 0) throw new Error('psql execution failed: ' + res.stderr);
  return res.stdout;
}

async function run() {
  if (mode === 'Apply') {
    console.log('Applying UI-001 LLM Provider Acceptance Fixture...');
    execQuery(applySql);
    console.log('Fixture applied successfully.');
  }

  console.log('Verifying UI-001 LLM Provider Acceptance Fixture...');
  
  // 1. SQL check
  const verifySql = `
SELECT json_build_object(
  'verified_exists', EXISTS(SELECT 1 FROM llm_provider_configurations WHERE id = '11000000-0000-4000-8000-000000000001'),
  'failed_exists', EXISTS(SELECT 1 FROM llm_provider_configurations WHERE id = '11000000-0000-4000-8000-000000000002'),
  'stale_exists', EXISTS(SELECT 1 FROM llm_provider_configurations WHERE id = '11000000-0000-4000-8000-000000000003'),
  'unverified_exists', EXISTS(SELECT 1 FROM llm_provider_configurations WHERE id = '11000000-0000-4000-8000-000000000004'),
  'verified_status', (SELECT integration_status FROM llm_provider_configurations WHERE id = '11000000-0000-4000-8000-000000000001'),
  'failed_status', (SELECT integration_status FROM llm_provider_configurations WHERE id = '11000000-0000-4000-8000-000000000002'),
  'stale_status', (SELECT integration_status FROM llm_provider_configurations WHERE id = '11000000-0000-4000-8000-000000000003'),
  'unverified_status', (SELECT integration_status FROM llm_provider_configurations WHERE id = '11000000-0000-4000-8000-000000000004'),
  'verified_models', (SELECT COUNT(*) FROM llm_provider_models WHERE provider_id = '11000000-0000-4000-8000-000000000001'),
  'failed_models', (SELECT COUNT(*) FROM llm_provider_models WHERE provider_id = '11000000-0000-4000-8000-000000000002'),
  'stale_models', (SELECT COUNT(*) FROM llm_provider_models WHERE provider_id = '11000000-0000-4000-8000-000000000003'),
  'unverified_models', (SELECT COUNT(*) FROM llm_provider_models WHERE provider_id = '11000000-0000-4000-8000-000000000004')
);
`;
  const sqlOut = execQuery(verifySql);
  const jsonLine = sqlOut.split('\n').map(s => s.trim()).find(s => s.startsWith('{') && s.endsWith('}'));
  assert(jsonLine, 'Could not parse verification JSON output: ' + sqlOut);
  const dbData = JSON.parse(jsonLine);

  assert.strictEqual(dbData.verified_exists, true, 'verified provider missing in DB');
  assert.strictEqual(dbData.failed_exists, true, 'failed provider missing in DB');
  assert.strictEqual(dbData.stale_exists, true, 'stale provider missing in DB');
  assert.strictEqual(dbData.unverified_exists, true, 'unverified provider missing in DB');

  assert.strictEqual(dbData.verified_status, 'verified', 'verified provider status incorrect');
  assert.strictEqual(dbData.failed_status, 'failed', 'failed provider status incorrect');
  assert.strictEqual(dbData.stale_status, 'stale', 'stale provider status incorrect');
  assert.strictEqual(dbData.unverified_status, 'unverified', 'unverified provider status incorrect');

  assert.strictEqual(dbData.verified_models, 12, 'verified provider models count incorrect');
  assert.strictEqual(dbData.failed_models, 3, 'failed provider models count incorrect');
  assert.strictEqual(dbData.stale_models, 5, 'stale provider models count incorrect');
  assert.strictEqual(dbData.unverified_models, 0, 'unverified provider models count incorrect');

  console.log('Database verification PASS.');

  // 2. REST API verification
  const res = await fetch('http://127.0.0.1:18080/api/v1/llm-providers').then(r => r.json());
  const items = res.data.items;

  const p1 = items.find(i => i.id === '11000000-0000-4000-8000-000000000001');
  const p2 = items.find(i => i.id === '11000000-0000-4000-8000-000000000002');
  const p3 = items.find(i => i.id === '11000000-0000-4000-8000-000000000003');
  const p4 = items.find(i => i.id === '11000000-0000-4000-8000-000000000004');

  assert(p1, 'OpenAI 主模型 missing in API response');
  assert(p2, '内容审核模型 missing in API response');
  assert(p3, '备用模型 missing in API response');
  assert(p4, '本地兼容模型 missing in API response');

  assert.strictEqual(p1.executable, true, 'OpenAI 主模型 executable should be true');
  assert.strictEqual(p2.executable, false, '内容审核模型 executable should be false');
  assert.strictEqual(p3.executable, false, '备用模型 executable should be false');
  assert.strictEqual(p4.executable, false, '本地兼容模型 executable should be false');

  assert.strictEqual(p1.enabled, true, 'OpenAI 主模型 enabled should be true');
  assert.strictEqual(p2.enabled, false, '内容审核模型 enabled should be false');
  assert.strictEqual(p3.enabled, true, '备用模型 enabled should be true');
  assert.strictEqual(p4.enabled, false, '本地兼容模型 enabled should be false');

  items.forEach(item => {
    assert(!('secret' in item), 'secret found in API item');
    assert(!('encryptedSecret' in item), 'encryptedSecret found in API item');
    assert(!('encrypted_secret' in item), 'encrypted_secret found in API item');
  });

  console.log('API verification PASS.');
  console.log('PASS');
}

run().catch(err => {
  console.error(err);
  process.exit(1);
});
'@

$tempRunner = [System.IO.Path]::Combine([System.IO.Path]::GetTempPath(), "ui001_fixture_runner_$([Guid]::NewGuid().ToString('N')).js")
try {
    $utf8NoBom = New-Object System.Text.UTF8Encoding $false
    [System.IO.File]::WriteAllText($tempRunner, $nodeScript, $utf8NoBom)
    & node $tempRunner $Mode $repoRoot
    if ($LASTEXITCODE -ne 0) { throw "Fixture execution failed." }
}
finally {
    if (Test-Path $tempRunner) { Remove-Item $tempRunner -Force }
}
