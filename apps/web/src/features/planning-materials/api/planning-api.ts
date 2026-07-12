import { ApiError, getProjectWorkspace, type Project } from "../../../lib/api.ts";
import type { PlanningMockScenario, ProjectPlanning, SaveProjectPlanningRequest } from "../contracts/planning.ts";
import { planningFixtureProject } from "../mock/fixtures.ts";
import { emptyPlanning, planningMatches, readPlanning, writePlanning } from "../mock/planning-repository.ts";

const validProjectId = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
const requestId = () => `req_planning_${crypto.randomUUID().replaceAll("-", "").slice(0, 12)}`;
const pause = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));
const clone = <T,>(value: T): T => structuredClone(value);

function assertProject(projectId: string, scenario: PlanningMockScenario) {
  if (!validProjectId.test(projectId)) throw new ApiError("项目地址无效。", 400, "INVALID_PROJECT_ID", {}, requestId());
  if (scenario === "not-found") throw new ApiError("项目不存在。", 404, "PROJECT_NOT_FOUND", {}, requestId());
}

export function parsePlanningMockScenario(value: string | undefined): PlanningMockScenario {
  return value === "empty" || value === "slow" || value === "load-error" || value === "save-error" || value === "conflict" || value === "no-results" || value === "no-references" || value === "no-current-usage" || value === "bind-error" || value === "idempotency-conflict" || value === "already-bound" || value === "already-unbound" || value === "unbind-error" || value === "binding-not-found" || value === "create-error" || value === "not-found" ? value : "default";
}

export async function getProjectPlanning(projectId: string, scenario: PlanningMockScenario = "default"): Promise<ProjectPlanning> {
  if (scenario === "slow") await pause(900);
  assertProject(projectId, scenario);
  if (scenario === "load-error") throw new ApiError("暂时无法读取策划内容。", 500, "PLANNING_LOAD_FAILED", {}, requestId());
  return clone(scenario === "empty" ? emptyPlanning(projectId) : readPlanning(projectId) ?? emptyPlanning(projectId));
}

export async function getPlanningProject(projectId: string, scenario: PlanningMockScenario = "default"): Promise<Project> {
  assertProject(projectId, scenario);
  if (projectId !== planningFixtureProject.id) return (await getProjectWorkspace(projectId)).project;
  return clone(planningFixtureProject);
}

export async function saveProjectPlanning(projectId: string, request: SaveProjectPlanningRequest, scenario: PlanningMockScenario = "default"): Promise<ProjectPlanning> {
  assertProject(projectId, scenario);
  if (scenario === "save-error") throw new ApiError("保存策划方案失败，请稍后重试。", 500, "PLANNING_SAVE_FAILED", {}, requestId());
  const current = scenario === "empty" ? emptyPlanning(projectId) : readPlanning(projectId) ?? emptyPlanning(projectId);
  if (scenario === "conflict" || request.expected_version !== current.version) throw new ApiError("当前数据已被更新，请重新加载后再保存。", 409, "VERSION_CONFLICT", {}, requestId());
  if (planningMatches(current, request)) return clone(current);
  const now = new Date().toISOString();
  const next: ProjectPlanning = { project_id: projectId, premise: request.premise, audience: request.audience, style: request.style, goals_json: clone(request.goals_json), constraints_json: clone(request.constraints_json), version: current.version + 1, created_at: current.created_at ?? now, updated_at: now };
  writePlanning(next);
  return clone(next);
}
