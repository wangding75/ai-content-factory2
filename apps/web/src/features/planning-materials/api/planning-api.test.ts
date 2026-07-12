import assert from "node:assert/strict";
import test from "node:test";
import { ApiError } from "../../../lib/api.ts";
import { getProjectPlanning, saveProjectPlanning } from "./planning-api.ts";
import { planningFixtureProject } from "../mock/fixtures.ts";
import { clearPlanning, resetPlanningRepository } from "../mock/planning-repository.ts";

const projectId = planningFixtureProject.id;
const emptyRequest = { premise: "", audience: "", style: "", goals_json: { selling_points: [], plot_summary: "" }, constraints_json: { emotional_tone: "" } };

test.beforeEach(() => resetPlanningRepository());

test("returns the contract empty planning state", async () => {
  const planning = await getProjectPlanning(projectId, "empty");
  assert.deepEqual(planning, { project_id: projectId, ...emptyRequest, version: 0, created_at: null, updated_at: null });
});

test("first save changes version zero to one", async () => {
  clearPlanning(projectId);
  const planning = await saveProjectPlanning(projectId, { ...emptyRequest, premise: "新主题", expected_version: 0 });
  assert.equal(planning.version, 1);
});

test("saving identical content preserves the version", async () => {
  clearPlanning(projectId);
  const first = await saveProjectPlanning(projectId, { ...emptyRequest, premise: "新主题", expected_version: 0 });
  const second = await saveProjectPlanning(projectId, { ...emptyRequest, premise: "新主题", expected_version: first.version });
  assert.equal(second.version, first.version);
});

test("rejects an outdated expected version", async () => {
  await assert.rejects(saveProjectPlanning(projectId, { ...emptyRequest, expected_version: 0 }), (error: unknown) => error instanceof ApiError && error.code === "VERSION_CONFLICT");
});
