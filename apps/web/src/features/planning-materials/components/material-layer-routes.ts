export type MaterialLayer = "create" | "pick" | "detail" | "edit" | "usage" | "unbind";

export function projectMaterialsRoute(projectId: string) {
  return `/projects/${projectId}/materials`;
}

export function materialDetailRoute(projectId: string, materialId: string) {
  return `${projectMaterialsRoute(projectId)}/${materialId}`;
}

export function closeMaterialLayer(layer: MaterialLayer, projectId: string, materialId?: string) {
  return layer === "create" || layer === "pick" || layer === "detail"
    ? projectMaterialsRoute(projectId)
    : materialDetailRoute(projectId, materialId!);
}
