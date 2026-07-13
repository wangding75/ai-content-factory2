import assert from "node:assert/strict";
import test from "node:test";
import { ApiError } from "../../../lib/api.ts";
import { createMaterialFromApi, getMaterialFromApi, listMaterialsFromApi, materialQuery, updateMaterialFromApi } from "./material-http-api.ts";

const originalFetch = global.fetch;
const material = {
  id: "30000000-0000-4000-8000-000000000001",
  type: "item" as const,
  name: "Compass",
  summary: "A real material",
  content_json: {},
  tags_json: ["tool"],
  version: 1,
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-02T00:00:00Z",
};

test.after(() => { global.fetch = originalFetch; });

test("maps global material list query parameters and response", async () => {
  assert.equal(materialQuery({ q: " compass ", type: "item", sort: "name_asc", limit: 10, offset: 20 }), "?q=compass&type=item&sort=name_asc&limit=10&offset=20");
  global.fetch = async (input) => {
    assert.match(String(input), /\/materials\?q=compass&type=item&sort=name_asc&limit=10&offset=20$/);
    return new Response(JSON.stringify({ data: { items: [material], total: 1, limit: 10, offset: 20 }, request_id: "req_list" }), { status: 200 });
  };
  assert.deepEqual(await listMaterialsFromApi({ q: " compass ", type: "item", sort: "name_asc", limit: 10, offset: 20 }), { items: [material], total: 1, limit: 10, offset: 20 });
});

test("maps empty global material lists", async () => {
  global.fetch = async () => new Response(JSON.stringify({ data: { items: [], total: 0, limit: 20, offset: 0 }, request_id: "req_empty" }), { status: 200 });
  assert.deepEqual(await listMaterialsFromApi(), { items: [], total: 0, limit: 20, offset: 0 });
});

test("maps material detail references without usage private fields", async () => {
  global.fetch = async () => new Response(JSON.stringify({
    data: {
      material,
      reference_count: 1,
      references: [{ usage_id: "40000000-0000-4000-8000-000000000001", project_id: "00000000-0000-4000-8000-000000000001", project_name: "Novel", project_type: "novel", usage_type: "private", notes: "private" }],
    },
    request_id: "req_detail",
  }), { status: 200 });
  const detail = await getMaterialFromApi(material.id);
  assert.equal(detail.reference_count, 1);
  assert.deepEqual(detail.references[0], { usage_id: "40000000-0000-4000-8000-000000000001", project_id: "00000000-0000-4000-8000-000000000001", project_name: "Novel", project_type: "novel" });
  assert.equal("usage_type" in detail.references[0], false);
});

test("maps not found envelopes and cancellation", async () => {
  global.fetch = async () => new Response(JSON.stringify({ error: { code: "MATERIAL_NOT_FOUND", message: "missing", details: {} }, request_id: "req_missing" }), { status: 404 });
  await assert.rejects(getMaterialFromApi(material.id), (error: unknown) => error instanceof ApiError && error.code === "MATERIAL_NOT_FOUND");
  global.fetch = async (_input, init) => new Promise<Response>((_resolve, reject) => init?.signal?.addEventListener("abort", () => reject(new DOMException("aborted", "AbortError"))));
  const controller = new AbortController();
  const pending = listMaterialsFromApi({}, { signal: controller.signal });
  controller.abort();
  await assert.rejects(pending, (error: unknown) => error instanceof ApiError && error.code === "cancelled");
});
test("creates a global material with its idempotency key", async () => {
  global.fetch = async (_input, init) => {
    assert.equal(init?.method, "POST");
    assert.equal(new Headers(init?.headers).get("Idempotency-Key"), "create-key-1");
    assert.deepEqual(JSON.parse(String(init?.body)), { type: "item", name: "Compass", summary: "A real material", content_json: {}, tags_json: ["tool"] });
    return new Response(JSON.stringify({ data: material, request_id: "req_create" }), { status: 201 });
  };
  assert.deepEqual(await createMaterialFromApi({ type: "item", name: "Compass", summary: "A real material", content_json: {}, tags_json: ["tool"] }, "create-key-1"), material);
});

test("maps idempotency key reuse envelopes", async () => {
  global.fetch = async () => new Response(JSON.stringify({ error: { code: "IDEMPOTENCY_KEY_REUSED", message: "key reused", details: {} }, request_id: "req_key" }), { status: 409 });
  await assert.rejects(createMaterialFromApi({ type: "item", name: "Compass", summary: "", content_json: {}, tags_json: [] }, "reused-key"), (error: unknown) => error instanceof ApiError && error.code === "IDEMPOTENCY_KEY_REUSED");
});

test("updates a global material with expected version", async () => {
  global.fetch = async (input, init) => {
    assert.match(String(input), new RegExp(`/materials/${material.id}$`));
    assert.equal(init?.method, "PATCH");
    assert.deepEqual(JSON.parse(String(init?.body)), { expected_version: 1, name: "Updated Compass" });
    return new Response(JSON.stringify({ data: { ...material, name: "Updated Compass", version: 2 }, request_id: "req_update" }), { status: 200 });
  };
  const updated = await updateMaterialFromApi(material.id, { expected_version: 1, name: "Updated Compass" });
  assert.equal(updated.version, 2);
  assert.equal(updated.name, "Updated Compass");
});

test("maps update conflict and missing material envelopes", async () => {
  global.fetch = async () => new Response(JSON.stringify({ error: { code: "VERSION_CONFLICT", message: "conflict", details: {} }, request_id: "req_conflict" }), { status: 409 });
  await assert.rejects(updateMaterialFromApi(material.id, { expected_version: 1, summary: "new" }), (error: unknown) => error instanceof ApiError && error.code === "VERSION_CONFLICT");
  global.fetch = async () => new Response(JSON.stringify({ error: { code: "MATERIAL_NOT_FOUND", message: "missing", details: {} }, request_id: "req_missing_update" }), { status: 404 });
  await assert.rejects(updateMaterialFromApi(material.id, { expected_version: 1, summary: "new" }), (error: unknown) => error instanceof ApiError && error.code === "MATERIAL_NOT_FOUND");
});
