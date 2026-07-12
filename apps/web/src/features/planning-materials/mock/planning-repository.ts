import type { ProjectPlanning, SaveProjectPlanningRequest } from "../contracts/planning.ts";
import { planningFixture } from "./fixtures.ts";

const clone = <T,>(value: T): T => structuredClone(value);

type PlanningStore = Map<string, ProjectPlanning>;
declare global { var __planningMockStore: PlanningStore | undefined; }

function store(): PlanningStore {
  if (!globalThis.__planningMockStore) globalThis.__planningMockStore = new Map([[planningFixture.project_id, clone(planningFixture)]]);
  return globalThis.__planningMockStore;
}

export function readPlanning(projectId: string) { return store().get(projectId); }
export function writePlanning(planning: ProjectPlanning) { store().set(planning.project_id, clone(planning)); }
export function resetPlanningRepository() { globalThis.__planningMockStore = new Map([[planningFixture.project_id, clone(planningFixture)]]); }
export function clearPlanning(projectId: string) { store().delete(projectId); }

export function emptyPlanning(projectId: string): ProjectPlanning {
  return { project_id: projectId, premise: "", audience: "", style: "", goals_json: { selling_points: [], plot_summary: "" }, constraints_json: { emotional_tone: "" }, version: 0, created_at: null, updated_at: null };
}

export function planningMatches(current: ProjectPlanning, request: SaveProjectPlanningRequest) {
  return current.premise === request.premise && current.audience === request.audience && current.style === request.style && current.goals_json.plot_summary === request.goals_json.plot_summary && current.constraints_json.emotional_tone === request.constraints_json.emotional_tone && current.goals_json.selling_points.length === request.goals_json.selling_points.length && current.goals_json.selling_points.every((item, index) => item === request.goals_json.selling_points[index]);
}
