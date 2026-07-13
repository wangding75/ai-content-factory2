import assert from "node:assert/strict";
import test from "node:test";
import { ApiError } from "../../../lib/api.ts";
import { bindProjectMaterialFromApi, createProjectMaterialFromApi, listProjectMaterialsFromApi, projectMaterialQuery } from "./project-material-http-api.ts";

const originalFetch = global.fetch;
const projectId = "00000000-0000-4000-8000-000000000001";
const materialId = "30000000-0000-4000-8000-000000000001";
const item = { material: { id: materialId, type: "character" as const, name: "Hero", summary: "", content_json: {}, tags_json: ["lead"], version: 3, created_at: "2026-01-01T00:00:00Z", updated_at: "2026-01-02T00:00:00Z" }, usage: { id: "40000000-0000-4000-8000-000000000001", project_id: projectId, material_id: materialId, usage_type: "lead", role_name: "Hero", notes: "Use in opening", start_chapter: 1, end_chapter: null, status: "active" as const, version: 2, created_at: "2026-01-01T00:00:00Z", updated_at: "2026-01-03T00:00:00Z" }, last_updated_at: "2026-01-03T00:00:00Z" };
const typeCounts = { character: 1, worldview: 0, location: 0, organization: 0, item: 0, reference: 0 };
const usage = { usage_type: "lead", role_name: "Hero", notes: "Use in opening", start_chapter: 1, end_chapter: null };
const createRequest = { material: { type: "character" as const, name: "Hero", summary: "", content_json: {}, tags_json: ["lead"] }, usage };

test.after(() => { global.fetch = originalFetch; });

test("maps project material path, query, item usage, and type counts", async () => {
  assert.equal(projectMaterialQuery({ q: " hero ", type: "character", sort: "name_asc", limit: 6, offset: 12 }), "?q=hero&type=character&sort=name_asc&limit=6&offset=12");
  global.fetch = async (input) => { assert.match(String(input), new RegExp(`/projects/${projectId}/materials\\?q=hero&type=character&sort=name_asc&limit=6&offset=12$`)); return new Response(JSON.stringify({ data: { items: [item], total: 13, limit: 6, offset: 12, type_counts: typeCounts }, request_id: "req_list" }), { status: 200 }); };
  assert.deepEqual(await listProjectMaterialsFromApi(projectId, { q: " hero ", type: "character", sort: "name_asc", limit: 6, offset: 12 }), { items: [item], total: 13, limit: 6, offset: 12, type_counts: typeCounts });
});

test("maps empty project material lists and PROJECT_NOT_FOUND", async () => {
  global.fetch = async () => new Response(JSON.stringify({ data: { items: [], total: 0, limit: 6, offset: 0, type_counts: { character: 0, worldview: 0, location: 0, organization: 0, item: 0, reference: 0 } }, request_id: "req_empty" }), { status: 200 });
  assert.deepEqual((await listProjectMaterialsFromApi(projectId)).items, []);
  global.fetch = async () => new Response(JSON.stringify({ error: { code: "PROJECT_NOT_FOUND", message: "missing", details: {} }, request_id: "req_missing" }), { status: 404 });
  await assert.rejects(listProjectMaterialsFromApi(projectId), (error: unknown) => error instanceof ApiError && error.code === "PROJECT_NOT_FOUND");
});

test("creates a material and usage with the required idempotency key", async () => {
  global.fetch = async (input, init) => { assert.match(String(input), new RegExp(`/projects/${projectId}/materials$`)); assert.equal(init?.method, "POST"); assert.equal(new Headers(init?.headers).get("Idempotency-Key"), "create-key"); assert.deepEqual(JSON.parse(String(init?.body)), createRequest); return new Response(JSON.stringify({ data: item, request_id: "req_create" }), { status: 201 }); };
  assert.deepEqual(await createProjectMaterialFromApi(projectId, createRequest, "create-key"), item);
});

test("binds an existing material with the required idempotency key and maps conflicts", async () => {
  global.fetch = async (input, init) => { assert.match(String(input), new RegExp(`/projects/${projectId}/materials/${materialId}/binding$`)); assert.equal(init?.method, "POST"); assert.equal(new Headers(init?.headers).get("Idempotency-Key"), "bind-key"); assert.deepEqual(JSON.parse(String(init?.body)), usage); return new Response(JSON.stringify({ data: item, request_id: "req_bind" }), { status: 201 }); };
  assert.deepEqual(await bindProjectMaterialFromApi(projectId, materialId, usage, "bind-key"), item);
  global.fetch = async () => new Response(JSON.stringify({ error: { code: "MATERIAL_ALREADY_BOUND", message: "bound", details: {} }, request_id: "req_bound" }), { status: 409 });
  await assert.rejects(bindProjectMaterialFromApi(projectId, materialId, usage, "other-key"), (error: unknown) => error instanceof ApiError && error.code === "MATERIAL_ALREADY_BOUND");
});