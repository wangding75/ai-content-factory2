export interface ProjectPlanningGoals {
  selling_points: string[];
  plot_summary: string;
}

export interface ProjectPlanningConstraints {
  emotional_tone: string;
}

export interface ProjectPlanning {
  project_id: string;
  premise: string;
  audience: string;
  style: string;
  goals_json: ProjectPlanningGoals;
  constraints_json: ProjectPlanningConstraints;
  version: number;
  created_at: string | null;
  updated_at: string | null;
}

export interface SaveProjectPlanningRequest {
  premise: string;
  audience: string;
  style: string;
  goals_json: ProjectPlanningGoals;
  constraints_json: ProjectPlanningConstraints;
  expected_version: number;
}

export interface ProjectPlanningEnvelope {
  data: ProjectPlanning;
  request_id: string;
}

export interface ErrorEnvelope {
  error: { code: string; message: string; details: Record<string, unknown> };
  request_id: string;
}

export type PlanningMockScenario = "default" | "empty" | "slow" | "load-error" | "save-error" | "conflict" | "no-results" | "no-references" | "no-current-usage" | "bind-error" | "idempotency-conflict" | "already-bound" | "already-unbound" | "unbind-error" | "binding-not-found" | "create-error" | "not-found";
