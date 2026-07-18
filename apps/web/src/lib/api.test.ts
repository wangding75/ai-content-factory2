import assert from "node:assert/strict";
import test from "node:test";
import { ApiError, createProject, listProjectTypes, listProjects, validateProjectInput } from "./api.ts";

const originalFetch = global.fetch;
test.after(() => { global.fetch = originalFetch; });

test("validates project input before submission", () => {
  assert.equal(validateProjectInput("", ""), "Project name is required.");
  assert.equal(validateProjectInput("x".repeat(121), ""), "Project name must be 120 characters or fewer.");
  assert.equal(validateProjectInput("Novel", "x".repeat(5001)), "Description must be 5,000 characters or fewer.");
  assert.equal(validateProjectInput("Novel", ""), "Project type is required.");
  assert.equal(validateProjectInput(" Novel ", "", "novel"), undefined);
});

test("loads project types from the formal catalogue endpoint", async () => {
  global.fetch = async (input) => {
    assert.match(String(input), /\/project-types$/);
    return new Response(JSON.stringify({ data: { items: [{ code: "novel", name: "小说", description: "小说创作", enabled: true, sort_order: 10 }] }, request_id: "req_types" }), { status: 200 });
  };
  assert.equal((await listProjectTypes()).items[0]?.name, "小说");
});

test("lists projects with contract pagination and status", async () => {
  global.fetch = async (input) => {
    assert.match(String(input), /limit=20/);
    assert.match(String(input), /offset=0/);
    assert.match(String(input), /status=planning/);
    return new Response(JSON.stringify({ data: { items: [], total: 0, limit: 20, offset: 0 }, request_id: "req_1" }), { status: 200 });
  };
  assert.deepEqual(await listProjects({ status: "planning" }), { items: [], total: 0, limit: 20, offset: 0 });
});

test("creates a novel project using the API contract", async () => {
  global.fetch = async (_input, init) => {
    assert.equal(init?.method, "POST");
    assert.deepEqual(JSON.parse(String(init?.body)), { name: "Novel", description: "Draft", type: "novel" });
    return new Response(JSON.stringify({ data: { id: "00000000-0000-4000-8000-000000000001", name: "Novel", description: "Draft", type: "novel", status: "planning", current_stage: "project_setup", created_at: "2026-01-01T00:00:00Z", updated_at: "2026-01-01T00:00:00Z" }, request_id: "req_2" }), { status: 201 });
  };
  assert.equal((await createProject({ name: "Novel", description: "Draft", type: "novel" })).name, "Novel");
});

test("surfaces the API error message", async () => {
  global.fetch = async () => new Response(JSON.stringify({ error: { code: "validation_error", message: "invalid project", details: {} }, request_id: "req_3" }), { status: 400 });
  await assert.rejects(createProject({ name: "Novel", type: "novel" }), (error: unknown) => error instanceof ApiError && error.message === "invalid project" && error.code === "validation_error");
});

test("rejects invalid JSON responses", async () => {
  global.fetch = async () => new Response("not json", { status: 200 });
  await assert.rejects(listProjects(), (error: unknown) => error instanceof ApiError && error.code === "invalid_json");
});

