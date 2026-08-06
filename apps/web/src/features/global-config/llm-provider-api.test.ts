import assert from "node:assert/strict";
import test from "node:test";
import { ApiError } from "../../lib/api.ts";
import { createLlmProvider, createLlmProviderPayload, mapLlmProvider, updateLlmProvider, updateLlmProviderPayload, listLlmProviders } from "./llm-provider-api.ts";

const provider = { id: "018f0c20-0000-4000-8000-000000000001", name: "主连接", providerType: "openai_compatible" as const, baseUrl: "https://api.example.test/v1", defaultModel: "gpt-4.1-mini", timeoutSeconds: 30, hasSecret: true, secretFingerprint: "4e3b9a1c", integrationStatus: "not_connected" as const, enabled: false, lastVerifiedAt: null, lastErrorCode: null, lastErrorMessage: null, version: 1, createdAt: "2026-07-19T09:00:00Z", updatedAt: "2026-07-19T09:00:00Z" };
test("maps API provider data to a safe card view model", () => { const vm = mapLlmProvider(provider); assert.equal(vm.providerTypeLabel, "OpenAI-compatible"); assert.equal(vm.integrationStatusLabel, "未接入"); assert.equal("secret" in vm, false); assert.equal(vm.secretFingerprint, "4e3b9a1c"); });
test("create sends contract fields and update excludes readonly fields", () => { const input = { name: " 主连接 ", providerType: "openai_compatible" as const, baseUrl: " https://api.example.test/v1 ", defaultModel: " gpt-4.1-mini ", timeoutSeconds: 30, secret: "do-not-display" }; assert.deepEqual(createLlmProviderPayload(input), { name: "主连接", providerType: "openai_compatible", baseUrl: "https://api.example.test/v1", defaultModel: "gpt-4.1-mini", timeoutSeconds: 30, secret: "do-not-display" }); assert.deepEqual(updateLlmProviderPayload({ ...input, expectedVersion: 1 }), { expectedVersion: 1, name: "主连接", baseUrl: "https://api.example.test/v1", defaultModel: "gpt-4.1-mini", timeoutSeconds: 30, secret: "do-not-display" }); });
test("409 is available as an explicit version conflict state", () => { const error = new ApiError("conflict", 409, "version_conflict"); assert.equal(error.status === 409 || error.code === "version_conflict", true); });
test("write requests use frozen create and update fields only", async () => { const originalFetch = global.fetch; const calls: { url: string; init: RequestInit | undefined }[] = []; global.fetch = async (input, init) => { calls.push({ url: String(input), init }); return new Response(JSON.stringify({ data: provider, request_id: "req" }), { status: 200 }); }; try { const input = { name: "主连接", providerType: "openai_compatible" as const, baseUrl: "https://api.example.test/v1", defaultModel: "gpt-4.1-mini", timeoutSeconds: 30, secret: "input-only" }; await createLlmProvider(input, "create-key"); await updateLlmProvider(provider.id, { name: input.name, baseUrl: input.baseUrl, defaultModel: input.defaultModel, timeoutSeconds: input.timeoutSeconds, secret: "", expectedVersion: 1 }, "update-key"); const createBody = JSON.parse(String(calls[0]!.init?.body)); const updateBody = JSON.parse(String(calls[1]!.init?.body)); assert.deepEqual(createBody, { ...input }); assert.deepEqual(updateBody, { expectedVersion: 1, name: input.name, baseUrl: input.baseUrl, defaultModel: input.defaultModel, timeoutSeconds: input.timeoutSeconds }); assert.equal(calls[1]!.url.endsWith(`/llm-providers/${provider.id}`), true); } finally { global.fetch = originalFetch; } });

test("detailed mapLlmProvider verification according to requirements", () => {
  // 1. verified 映射为“验证成功”
  // 2. stale 映射为“配置已变更”
  // 3. raw validationStatus 被保留
  // 4. enabled 被保留
  // 5. checkedAt 和 lastVerifiedAt 被保留
  // 6. lastErrorMessage 被保留
  // 7. executable=false 不会因为 enabled=true 被改成 true
  // 8. ViewModel 不包含 secret
  const providerVerified = {
    ...provider,
    validationStatus: "verified" as const,
    enabled: true,
    executable: true,
    lastVerifiedAt: "2026-08-06T08:00:00Z",
    checkedAt: "2026-08-06T08:05:00Z",
    lastErrorMessage: "no error"
  };
  const vmVerified = mapLlmProvider(providerVerified);
  assert.equal(vmVerified.validationStatusLabel, "验证成功");
  assert.equal(vmVerified.validationStatus, "verified");
  assert.equal(vmVerified.enabled, true);
  assert.equal(vmVerified.lastVerifiedAt, "2026-08-06T08:00:00Z");
  assert.equal(vmVerified.checkedAt, "2026-08-06T08:05:00Z");
  assert.equal(vmVerified.lastErrorMessage, "no error");
  assert.equal(vmVerified.executable, true);
  assert.equal("secret" in vmVerified, false);

  const providerStale = {
    ...provider,
    validationStatus: "stale" as const,
    enabled: true,
    executable: false,
    lastVerifiedAt: "2026-08-04T08:00:00Z",
    checkedAt: null,
    lastErrorMessage: "stale error"
  };
  const vmStale = mapLlmProvider(providerStale);
  assert.equal(vmStale.validationStatusLabel, "配置已变更");
  assert.equal(vmStale.validationStatus, "stale");
  assert.equal(vmStale.enabled, true);
  assert.equal(vmStale.lastVerifiedAt, "2026-08-04T08:00:00Z");
  assert.equal(vmStale.checkedAt, null);
  assert.equal(vmStale.lastErrorMessage, "stale error");
  assert.equal(vmStale.executable, false); // executable=false is preserved as false
  assert.equal("secret" in vmStale, false);
});

test("listLlmProviders query parameter mapping for executable=true/false", async () => {
  const originalFetch = global.fetch;
  const urls: string[] = [];
  global.fetch = async (input) => {
    urls.push(String(input));
    return new Response(JSON.stringify({ data: { items: [], total: 0 }, request_id: "req" }), { status: 200 });
  };
  try {
    // 9. listLlmProviders executable=true 生成正确查询参数
    await listLlmProviders({ executable: true });
    // 10. listLlmProviders executable=false 生成正确查询参数
    await listLlmProviders({ executable: false });
    
    const url1 = new URL(urls[0]!, "http://localhost");
    assert.equal(url1.searchParams.get("executable"), "true");
    
    const url2 = new URL(urls[1]!, "http://localhost");
    assert.equal(url2.searchParams.get("executable"), "false");
  } finally {
    global.fetch = originalFetch;
  }
});
