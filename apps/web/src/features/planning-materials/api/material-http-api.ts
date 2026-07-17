import { apiRequest, type ApiRequestInit } from "../../../lib/api.ts";
import type { CreateMaterialRequest, Material, MaterialList, MaterialType, ProjectMaterialSort } from "../contracts/materials.ts";

export interface ListGlobalMaterialsQuery {
  scope?: "global";
  q?: string;
  type?: MaterialType;
  sort?: ProjectMaterialSort;
  limit?: number;
  offset?: number;
}

export interface GlobalMaterialReference {
  usage_id: string;
  project_id: string;
  project_name: string;
  project_type: "novel";
}

export interface GlobalMaterialDetail {
  material: Material;
  references: GlobalMaterialReference[];
  reference_count: number;
}

interface MaterialListResponse {
  items: Material[];
  total: number;
  limit: number;
  offset: number;
}

interface MaterialDetailResponse {
  material: Material;
  references: Array<{
    usage_id: string;
    project_id: string;
    project_name: string;
    project_type: "novel";
  }>;
  reference_count: number;
}

export interface UpdateMaterialRequest {
  expected_version: number;
  name?: string;
  summary?: string;
  content_json?: Record<string, unknown>;
  tags_json?: string[];
}
export function materialQuery(query: ListGlobalMaterialsQuery): string {
  const params = new URLSearchParams();
  if (query.scope) params.set("scope", query.scope);
  if (query.q?.trim()) params.set("q", query.q.trim());
  if (query.type) params.set("type", query.type);
  if (query.sort) params.set("sort", query.sort);
  if (query.limit !== undefined) params.set("limit", String(query.limit));
  if (query.offset !== undefined) params.set("offset", String(query.offset));
  const value = params.toString();
  return value ? `?${value}` : "";
}

export async function listMaterialsFromApi(query: ListGlobalMaterialsQuery = {}, init?: ApiRequestInit): Promise<MaterialList> {
  const response = await apiRequest<MaterialListResponse>(`/materials${materialQuery(query)}`, init);
  return { items: response.items, total: response.total, limit: response.limit, offset: response.offset };
}

export async function getMaterialFromApi(materialId: string, init?: ApiRequestInit): Promise<GlobalMaterialDetail> {
  const response = await apiRequest<MaterialDetailResponse>(`/materials/${encodeURIComponent(materialId)}`, init);
  return {
    material: response.material,
    references: response.references.map((reference) => ({
      usage_id: reference.usage_id,
      project_id: reference.project_id,
      project_name: reference.project_name,
      project_type: reference.project_type,
    })),
    reference_count: response.reference_count,
  };
}
export async function createMaterialFromApi(
  request: CreateMaterialRequest,
  idempotencyKey: string,
  init?: ApiRequestInit,
): Promise<Material> {
  return apiRequest<Material>("/materials", {
    ...init,
    method: "POST",
    headers: { ...init?.headers, "Content-Type": "application/json", "Idempotency-Key": idempotencyKey },
    body: JSON.stringify(request),
  });
}

export async function updateMaterialFromApi(
  materialId: string,
  request: UpdateMaterialRequest,
  init?: ApiRequestInit,
): Promise<Material> {
  return apiRequest<Material>(`/materials/${encodeURIComponent(materialId)}`, {
    ...init,
    method: "PATCH",
    headers: { ...init?.headers, "Content-Type": "application/json" },
    body: JSON.stringify(request),
  });
}
