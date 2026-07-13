import { apiRequest, type ApiRequestInit } from "../../../lib/api.ts";
import type { CreateProjectMaterialRequest, ListProjectMaterialsQuery, ProjectMaterialItem, ProjectMaterialList, ProjectMaterialTypeCounts, ProjectMaterialUsageInput } from "../contracts/materials.ts";

export interface UpdateProjectMaterialUsageRequest extends ProjectMaterialUsageInput {
  expected_version: number;
}

export interface UnbindProjectMaterialResult {
  project_id: string;
  material_id: string;
  unbound: boolean;
  material_retained: true;
}

interface ProjectMaterialListResponse {
  items: ProjectMaterialItem[];
  total: number;
  limit: number;
  offset: number;
  type_counts: ProjectMaterialTypeCounts;
}

function projectMaterialItem(item: ProjectMaterialItem): ProjectMaterialItem {
  return { material: item.material, usage: item.usage, last_updated_at: item.last_updated_at };
}

export function projectMaterialQuery(query: ListProjectMaterialsQuery = {}): string {
  const params = new URLSearchParams();
  if (query.q?.trim()) params.set("q", query.q.trim());
  if (query.type) params.set("type", query.type);
  if (query.sort) params.set("sort", query.sort);
  if (query.limit !== undefined) params.set("limit", String(query.limit));
  if (query.offset !== undefined) params.set("offset", String(query.offset));
  const value = params.toString();
  return value ? `?${value}` : "";
}

export async function listProjectMaterialsFromApi(projectId: string, query: ListProjectMaterialsQuery = {}, init?: ApiRequestInit): Promise<ProjectMaterialList> {
  const response = await apiRequest<ProjectMaterialListResponse>(`/projects/${encodeURIComponent(projectId)}/materials${projectMaterialQuery(query)}`, init);
  return { items: response.items.map(projectMaterialItem), total: response.total, limit: response.limit, offset: response.offset, type_counts: response.type_counts };
}

export async function createProjectMaterialFromApi(projectId: string, request: CreateProjectMaterialRequest, idempotencyKey: string, init?: ApiRequestInit): Promise<ProjectMaterialItem> {
  const response = await apiRequest<ProjectMaterialItem>(`/projects/${encodeURIComponent(projectId)}/materials`, {
    ...init, method: "POST", headers: { ...init?.headers, "Content-Type": "application/json", "Idempotency-Key": idempotencyKey }, body: JSON.stringify(request),
  });
  return projectMaterialItem(response);
}

export async function bindProjectMaterialFromApi(projectId: string, materialId: string, usage: ProjectMaterialUsageInput, idempotencyKey: string, init?: ApiRequestInit): Promise<ProjectMaterialItem> {
  const response = await apiRequest<ProjectMaterialItem>(`/projects/${encodeURIComponent(projectId)}/materials/${encodeURIComponent(materialId)}/binding`, {
    ...init, method: "POST", headers: { ...init?.headers, "Content-Type": "application/json", "Idempotency-Key": idempotencyKey }, body: JSON.stringify(usage),
  });
  return projectMaterialItem(response);
}

export async function updateProjectMaterialUsageFromApi(projectId: string, materialId: string, request: UpdateProjectMaterialUsageRequest, init?: ApiRequestInit): Promise<ProjectMaterialItem> {
  const response = await apiRequest<ProjectMaterialItem>(`/projects/${encodeURIComponent(projectId)}/materials/${encodeURIComponent(materialId)}/usage`, {
    ...init, method: "PATCH", headers: { ...init?.headers, "Content-Type": "application/json" }, body: JSON.stringify(request),
  });
  return projectMaterialItem(response);
}

export async function unbindProjectMaterialFromApi(projectId: string, materialId: string, expectedVersion: number, init?: ApiRequestInit): Promise<UnbindProjectMaterialResult> {
  const params = new URLSearchParams({ expected_version: String(expectedVersion) });
  return apiRequest<UnbindProjectMaterialResult>(`/projects/${encodeURIComponent(projectId)}/materials/${encodeURIComponent(materialId)}/binding?${params}`, {
    ...init, method: "DELETE",
  });
}