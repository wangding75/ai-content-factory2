import { apiRequest, type ApiRequestInit } from "../../../lib/api.ts";
import type { ProjectPlanning, SaveProjectPlanningRequest } from "../contracts/planning.ts";

interface PlanningResponse {
  project_id: string;
  premise: string;
  audience: string;
  style: string;
  goals_json: { selling_points: string[]; plot_summary: string };
  constraints_json: { emotional_tone: string };
  version: number;
  created_at: string | null;
  updated_at: string | null;
}

function toPlanning(value: PlanningResponse): ProjectPlanning {
  return {
    project_id: value.project_id,
    premise: value.premise,
    audience: value.audience,
    style: value.style,
    goals_json: {
      selling_points: [...value.goals_json.selling_points],
      plot_summary: value.goals_json.plot_summary,
    },
    constraints_json: { emotional_tone: value.constraints_json.emotional_tone },
    version: value.version,
    created_at: value.created_at,
    updated_at: value.updated_at,
  };
}

export async function getProjectPlanningFromApi(projectId: string, init?: ApiRequestInit): Promise<ProjectPlanning> {
  const response = await apiRequest<PlanningResponse>(`/projects/${encodeURIComponent(projectId)}/planning`, init);
  return toPlanning(response);
}

export async function saveProjectPlanningToApi(projectId: string, request: SaveProjectPlanningRequest, init?: ApiRequestInit): Promise<ProjectPlanning> {
  const response = await apiRequest<PlanningResponse>(`/projects/${encodeURIComponent(projectId)}/planning`, {
    ...init,
    method: "PUT",
    headers: { "Content-Type": "application/json", ...init?.headers },
    body: JSON.stringify(request),
  });
  return toPlanning(response);
}
