export type ProjectType = "novel";
export type ProjectStatus = "planning" | "producing" | "archived";
export type ProjectStage =
  | "project_setup"
  | "project_planning"
  | "materials"
  | "storylines"
  | "chapter_planning"
  | "content_production"
  | "review"
  | "completed";

export interface Project {
  id: string;
  name: string;
  type: ProjectType;
  status: ProjectStatus;
  description: string;
  current_stage: ProjectStage;
  created_at: string;
  updated_at: string;
}

export interface ProjectList {
  items: Project[];
  total: number;
  limit: number;
  offset: number;
}

export interface ProjectWorkspace {
  project: Project;
  progress: {
    material_count: number;
    storyline_count: number;
    confirmed_chapter_count: number;
    work_count: number;
  };
}

interface Envelope<T> {
  data: T;
  request_id: string;
}

interface ErrorEnvelope {
  error: { code: string; message: string; details: Record<string, unknown> };
  request_id: string;
}

export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly details: Record<string, unknown>;
  readonly requestId?: string;

  constructor(message: string, status: number, code = "api_error", details: Record<string, unknown> = {}, requestId?: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
    this.details = details;
    this.requestId = requestId;
  }
}

export function apiBaseUrl() {
  if (typeof window !== "undefined") return "/api/v1";
  return process.env.API_BASE_URL ?? process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://localhost:18080/api/v1";
}

export interface ApiRequestInit extends RequestInit {
  timeoutMs?: number;
}

export async function apiRequest<T>(path: string, init?: ApiRequestInit): Promise<T> {
  const controller = new AbortController();
  const timeoutMs = init?.timeoutMs ?? 10_000;
  const timeout = setTimeout(() => controller.abort("timeout"), timeoutMs);
  const cancel = () => controller.abort("cancelled");
  init?.signal?.addEventListener("abort", cancel, { once: true });

  let response: Response;
  try {
    response = await fetch(`${apiBaseUrl()}${path}`, {
      ...init,
      headers: { Accept: "application/json", ...init?.headers },
      cache: "no-store",
      signal: controller.signal,
    });
  } catch {
    if (controller.signal.aborted) {
      const timedOut = controller.signal.reason === "timeout";
      throw new ApiError(timedOut ? "The API request timed out." : "The API request was cancelled.", 0, timedOut ? "timeout" : "cancelled");
    }
    throw new ApiError("Unable to reach the API. Please try again.", 0, "network_error");
  } finally {
    clearTimeout(timeout);
    init?.signal?.removeEventListener("abort", cancel);
  }

  let body: unknown;
  try {
    body = await response.json();
  } catch {
    throw new ApiError("The API returned an invalid response.", response.status, "invalid_json");
  }

  if (!response.ok) {
    const error = body as Partial<ErrorEnvelope>;
    if (error.error && typeof error.error.message === "string") {
      throw new ApiError(error.error.message, response.status, error.error.code, error.error.details ?? {}, error.request_id);
    }
    throw new ApiError("The API returned an unexpected error.", response.status, "invalid_error_response");
  }

  const envelope = body as Partial<Envelope<T>>;
  if (!("data" in envelope) || typeof envelope.request_id !== "string") {
    throw new ApiError("The API returned an invalid response.", response.status, "invalid_envelope");
  }
  return envelope.data as T;
}

export function listProjects(options: { status?: ProjectStatus; limit?: number; offset?: number } = {}) {
  const params = new URLSearchParams({ limit: String(options.limit ?? 20), offset: String(options.offset ?? 0) });
  if (options.status) params.set("status", options.status);
  return apiRequest<ProjectList>(`/projects?${params}`);
}

export function getProjectWorkspace(projectId: string) {
  return apiRequest<ProjectWorkspace>(`/projects/${encodeURIComponent(projectId)}/workspace`);
}

export function createProject(input: { name: string; description?: string; type: ProjectType }) {
  return apiRequest<Project>("/projects", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
}

export function validateProjectInput(name: string, description: string) {
  const trimmedName = name.trim();
  if (!trimmedName) return "Project name is required.";
  if (trimmedName.length > 120) return "Project name must be 120 characters or fewer.";
  if (description.length > 5000) return "Description must be 5,000 characters or fewer.";
  return undefined;
}
