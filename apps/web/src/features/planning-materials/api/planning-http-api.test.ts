import assert from "node:assert/strict";
import test from "node:test";
import { ApiError } from "../../../lib/api.ts";
import { getProjectPlanningFromApi, saveProjectPlanningToApi } from "./planning-http-api.ts";

const originalFetch = global.fetch;
const projectId = "00000000-0000-4000-8000-000000000001";
const planning = {
  project_id: projectId,
  premise: "Premise",
  audience: "Readers",
  style: "Clear",
  goals_json: { selling_points: ["One"], plot_summary: "Summary" },
  constraints_json: { emotional_tone: "Warm" },
  version: 1,
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
};

test.after(() => { global.fetch = originalFetch; });

test("gets planning from the API envelope and maps the response", async () => {
  global.fetch = async (input, init) => {
    assert.match(String(input), new RegExp(`/projects/${projectId}/planning$`));
    assert.equal(init?.method, undefined);
    return new Response(JSON.stringify({ data: planning, request_id: "req_get" }), { status: 200 });
  };
  const result = await getProjectPlanningFromApi(projectId);
  assert.deepEqual(result, planning);
  assert.notEqual(result.goals_json.selling_points, planning.goals_json.selling_points);
});

test("puts planning with expected_version and uses the returned version", async () => {
  global.fetch = async (input, init) => {
    assert.match(String(input), /\/planning$/);
    assert.equal(init?.method, "PUT");
    assert.deepEqual(JSON.parse(String(init?.body)), { ...planning, expected_version: 1 });
    return new Response(JSON.stringify({ data: { ...planning, version: 2 }, request_id: "req_put" }), { status: 200 });
  };
  const result = await saveProjectPlanningToApi(projectId, { ...planning, expected_version: 1 });
  assert.equal(result.version, 2);
});

test("maps error envelopes without falling back to mock data", async () => {
  global.fetch = async () => new Response(JSON.stringify({ error: { code: "VERSION_CONFLICT", message: "conflict", details: {} }, request_id: "req_conflict" }), { status: 409 });
  await assert.rejects(getProjectPlanningFromApi(projectId), (error: unknown) => error instanceof ApiError && error.code === "VERSION_CONFLICT" && error.status === 409);
});
