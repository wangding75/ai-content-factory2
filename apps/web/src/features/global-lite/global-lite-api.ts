import { apiRequest, type ApiRequestInit } from "@/lib/api";
import { getMaterialFromApi, listMaterialsFromApi } from "@/features/planning-materials/api/material-http-api";
import type { Material, MaterialType } from "@/features/planning-materials/contracts/materials";
import type { ProjectWorkDto, WorkflowRunStatus } from "@/features/project-works/project-work-api";

export type MaterialVm = Material & { references: { project_id: string; project_name: string }[] };
export type WorkVm = { project: { id: string; name: string; status: "planning" | "producing" | "archived" }; work: ProjectWorkDto };
export type BuiltinWorkflowDto = { workflow_key: "chapter_plan_mock_generate" | "content_mock_generate" | "content_mock_review" | "content_mock_rewrite"; provider_key: "mock"; label: string; description: string; status: "enabled"; result_kind: "chapter_plan" | "content" | "review" | "rewrite" };
export type GlobalWorkflowRunDto = { id: string; provider_key: "mock"; workflow_key: BuiltinWorkflowDto["workflow_key"]; status: WorkflowRunStatus; project: { id: string; name: string; status: "planning" | "producing" | "archived" } | null; subject: { type: "chapter_plan" | "content_item" | "review_report" | "content_version"; id: string }; error: { code: string; message: string } | null; started_at: string; finished_at: string | null };
export type CapabilityDto = { key: "mock_content" | "real_ai"; label: string; status: "enabled" | "not_configured"; description: string; workflow_keys: string[] };
export type IntegrationDto = { key: "wechat" | "douyin" | "youtube" | "n8n" | "coze" | "comfyui"; label: string; category: "publish" | "workflow"; status: "not_available"; description: string };
export type BuiltinWorkflowVm = { key: string; provider: string; label: string; description: string; status: string; resultKind: string };
export type WorkflowRunVm = { id: string; workflowKey: string; provider: string; status: WorkflowRunStatus; projectName: string; subject: string; failureSummary: string | null; startedAt: string; finishedAt: string | null; projectWorksHref: string | null };
export type CapabilityVm = { key: string; label: string; status: string; description: string; enabled: boolean; workflowKeys: string[] };
export type IntegrationVm = { key: string; label: string; category: string; status: string; description: string; available: boolean };
type Page<T> = { items: T[]; total: number; limit: number; offset: number };

const pageQuery = (values: Record<string, string | number | undefined>) => {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(values)) if (value !== undefined) params.set(key, String(value));
  return `?${params}`;
};
const mapBuiltinWorkflow = (dto: BuiltinWorkflowDto): BuiltinWorkflowVm => ({ key: dto.workflow_key, provider: dto.provider_key, label: dto.label, description: dto.description, status: dto.status, resultKind: dto.result_kind });
const mapWorkflowRun = (dto: GlobalWorkflowRunDto): WorkflowRunVm => ({ id: dto.id, workflowKey: dto.workflow_key, provider: dto.provider_key, status: dto.status, projectName: dto.project?.name ?? "未关联项目", subject: `${dto.subject.type} · ${dto.subject.id.slice(0, 8)}`, failureSummary: dto.error?.message ?? null, startedAt: dto.started_at, finishedAt: dto.finished_at, projectWorksHref: dto.project ? `/projects/${dto.project.id}/works` : null });
const mapCapability = (dto: CapabilityDto): CapabilityVm => ({ key: dto.key, label: dto.label, status: dto.status, description: dto.description, enabled: dto.status === "enabled", workflowKeys: dto.workflow_keys });
const mapIntegration = (dto: IntegrationDto): IntegrationVm => ({ key: dto.key, label: dto.label, category: dto.category, status: dto.status, description: dto.description, available: false });

export async function listGlobalMaterials(query: { type?: MaterialType; limit: number; offset: number }, init?: ApiRequestInit): Promise<Page<MaterialVm>> {
  const list = await listMaterialsFromApi({ scope: "global", type: query.type, limit: query.limit, offset: query.offset, sort: "updated_at_desc" }, init);
  const items = await Promise.all(list.items.map(async (material) => {
    const detail = await getMaterialFromApi(material.id, init);
    return { ...detail.material, references: detail.references.map(({ project_id, project_name }) => ({ project_id, project_name })) };
  }));
  return { ...list, items };
}
export function listGlobalWorks(query: { limit: number; offset: number }, init?: ApiRequestInit) { return apiRequest<Page<WorkVm>>(`/works${pageQuery({ scope: "global", limit: query.limit, offset: query.offset })}`, init); }
export async function listBuiltinWorkflows(init?: ApiRequestInit) { const response = await apiRequest<{ items: BuiltinWorkflowDto[] }>("/workflows/builtin", init); return response.items.map(mapBuiltinWorkflow); }
export async function listGlobalWorkflowRuns(query: { limit: number; offset: number }, init?: ApiRequestInit): Promise<Page<WorkflowRunVm>> { const response = await apiRequest<Page<GlobalWorkflowRunDto>>(`/workflow-runs${pageQuery(query)}`, init); return { ...response, items: response.items.map(mapWorkflowRun) }; }
export async function listCapabilities(init?: ApiRequestInit) { const response = await apiRequest<{ items: CapabilityDto[] }>("/capabilities", init); return response.items.map(mapCapability); }
export async function listIntegrations(init?: ApiRequestInit) { const response = await apiRequest<{ items: IntegrationDto[] }>("/integrations", init); return response.items.map(mapIntegration); }
export const globalLiteRequestPaths = { materials: "/api/v1/materials?scope=global", works: "/api/v1/works?scope=global", builtinWorkflows: "/api/v1/workflows/builtin", workflowRuns: "/api/v1/workflow-runs", capabilities: "/api/v1/capabilities", integrations: "/api/v1/integrations" } as const;
