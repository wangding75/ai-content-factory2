import { apiRequest, type ApiRequestInit } from "../../../lib/api.ts";
import type { ListProjectMaterialsQuery, ProjectMaterialItem, ProjectMaterialList, ProjectMaterialTypeCounts } from "../contracts/materials.ts";

interface ProjectMaterialListResponse {
  items: ProjectMaterialItem[];
  total: number;
  limit: number;
  offset: number;
  type_counts: ProjectMaterialTypeCounts;
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

export async function listProjectMaterialsFromApi(
  projectId: string,
  query: ListProjectMaterialsQuery = {},
  init?: ApiRequestInit,
): Promise<ProjectMaterialList> {
  const response = await apiRequest<ProjectMaterialListResponse>(
    `/projects/${encodeURIComponent(projectId)}/materials${projectMaterialQuery(query)}`,
    init,
  );
  return {
    items: response.items.map((item) => ({
      material: item.material,
      usage: item.usage,
      last_updated_at: item.last_updated_at,
    })),
    total: response.total,
    limit: response.limit,
    offset: response.offset,
    type_counts: response.type_counts,
  };
}
