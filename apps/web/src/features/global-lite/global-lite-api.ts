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
export type BuiltinWorkflowVm = { id: string; label: string; description: string; statusLabel: string; resultLabel: string; steps: string[] };
export type WorkflowRunVm = { id: string; workflowLabel: string; status: WorkflowRunStatus; statusLabel: string; projectName: string; subjectLabel: string; failureSummary: string | null; startedAtLabel: string; finishedAtLabel: string; projectWorksHref: string | null };
export type CapabilityVm = { id: string; label: string; description: string; statusLabel: string; categoryLabel: string; scopeLabel: string; hasWorkflow: boolean };
export type IntegrationVm = { id: string; label: string; description: string; statusLabel: string; categoryLabel: string; scopeLabel: string };
type Page<T> = { items: T[]; total: number; limit: number; offset: number };

const pageQuery = (values: Record<string, string | number | undefined>) => { const params = new URLSearchParams(); for (const [key, value] of Object.entries(values)) if (value !== undefined) params.set(key, String(value)); return `?${params}`; };
const workflowPresentation: Record<BuiltinWorkflowDto["workflow_key"], Omit<BuiltinWorkflowVm, "id" | "description" | "statusLabel">> = {
  chapter_plan_mock_generate: { label: "章节规划生成", resultLabel: "产出章节规划", steps: ["读取项目规划", "生成章节建议", "保存规划结果"] },
  content_mock_generate: { label: "正文内容生成", resultLabel: "产出正文草稿", steps: ["读取章节规划", "生成正文草稿", "保存正文版本"] },
  content_mock_review: { label: "内容质量评审", resultLabel: "产出评审结果", steps: ["读取正文版本", "检查内容质量", "生成评审结果"] },
  content_mock_rewrite: { label: "内容改写", resultLabel: "产出改写版本", steps: ["读取评审建议", "生成改写版本", "保留原始版本"] },
};
const runStatusLabel: Record<WorkflowRunStatus, string> = { running: "运行中", succeeded: "已成功", failed: "失败" };
const subjectLabel: Record<GlobalWorkflowRunDto["subject"]["type"], string> = { chapter_plan: "章节规划", content_item: "内容条目", review_report: "评审报告", content_version: "内容版本" };
const capabilityPresentation: Record<CapabilityDto["key"], Pick<CapabilityVm, "label" | "description" | "categoryLabel">> = { mock_content: { label: "模拟内容生成", description: "使用内置模拟能力完成规划、生成、评审和改写流程。", categoryLabel: "内容生产" }, real_ai: { label: "真实 AI 生成", description: "连接真实 AI 服务的能力将在后续版本开放。", categoryLabel: "智能生成" } };
const integrationPresentation: Record<IntegrationDto["key"], Pick<IntegrationVm, "label" | "description" | "categoryLabel">> = { wechat: { label: "微信公众号", description: "内容发布连接将在后续版本开放。", categoryLabel: "发布渠道" }, douyin: { label: "抖音", description: "内容发布连接将在后续版本开放。", categoryLabel: "发布渠道" }, youtube: { label: "YouTube", description: "内容发布连接将在后续版本开放。", categoryLabel: "发布渠道" }, n8n: { label: "n8n 自动化", description: "外部自动化连接将在后续版本开放。", categoryLabel: "工作流协同" }, coze: { label: "Coze 智能体", description: "外部智能体连接将在后续版本开放。", categoryLabel: "工作流协同" }, comfyui: { label: "ComfyUI 图像工作流", description: "图像工作流连接将在后续版本开放。", categoryLabel: "工作流协同" } };
const formatTime = (value: string | null) => value ? new Intl.DateTimeFormat("zh-CN", { dateStyle: "medium", timeStyle: "short" }).format(new Date(value)) : "进行中";
const mapBuiltinWorkflow = (dto: BuiltinWorkflowDto): BuiltinWorkflowVm => ({ id: dto.workflow_key, ...workflowPresentation[dto.workflow_key], description: dto.description, statusLabel: "已启用" });
const mapWorkflowRun = (dto: GlobalWorkflowRunDto): WorkflowRunVm => ({ id: dto.id, workflowLabel: workflowPresentation[dto.workflow_key].label, status: dto.status, statusLabel: runStatusLabel[dto.status], projectName: dto.project?.name ?? "未关联项目", subjectLabel: subjectLabel[dto.subject.type], failureSummary: dto.error?.message ?? null, startedAtLabel: formatTime(dto.started_at), finishedAtLabel: formatTime(dto.finished_at), projectWorksHref: dto.project ? `/projects/${dto.project.id}/works` : null });
const mapCapability = (dto: CapabilityDto): CapabilityVm => ({ id: dto.key, ...capabilityPresentation[dto.key], statusLabel: dto.status === "enabled" ? "已启用" : "已停用", scopeLabel: dto.workflow_keys.length ? "适用于内容生产流程" : "当前不可用", hasWorkflow: dto.workflow_keys.length > 0 });
const mapIntegration = (dto: IntegrationDto): IntegrationVm => ({ id: dto.key, ...integrationPresentation[dto.key], statusLabel: "暂未开放", scopeLabel: dto.category === "publish" ? "适用于内容发布" : "适用于自动化协同" });

export async function listGlobalMaterials(query: { type?: MaterialType; limit: number; offset: number }, init?: ApiRequestInit): Promise<Page<MaterialVm>> { const list = await listMaterialsFromApi({ scope: "global", type: query.type, limit: query.limit, offset: query.offset, sort: "updated_at_desc" }, init); const items = await Promise.all(list.items.map(async (material) => { const detail = await getMaterialFromApi(material.id, init); return { ...detail.material, references: detail.references.map(({ project_id, project_name }) => ({ project_id, project_name })) }; })); return { ...list, items }; }
export function listGlobalWorks(query: { limit: number; offset: number }, init?: ApiRequestInit) { return apiRequest<Page<WorkVm>>(`/works${pageQuery({ scope: "global", limit: query.limit, offset: query.offset })}`, init); }
export async function listBuiltinWorkflows(init?: ApiRequestInit) { const response = await apiRequest<{ items: BuiltinWorkflowDto[] }>("/workflows/builtin", init); return response.items.map(mapBuiltinWorkflow); }
export async function listGlobalWorkflowRuns(query: { limit: number; offset: number }, init?: ApiRequestInit): Promise<Page<WorkflowRunVm>> { const response = await apiRequest<Page<GlobalWorkflowRunDto>>(`/workflow-runs${pageQuery(query)}`, init); return { ...response, items: response.items.map(mapWorkflowRun) }; }
export async function listCapabilities(init?: ApiRequestInit) { const response = await apiRequest<{ items: CapabilityDto[] }>("/capabilities", init); return response.items.map(mapCapability); }
export async function listIntegrations(init?: ApiRequestInit) { const response = await apiRequest<{ items: IntegrationDto[] }>("/integrations", init); return response.items.map(mapIntegration); }
export const globalLiteRequestPaths = { materials: "/api/v1/materials?scope=global", works: "/api/v1/works?scope=global", builtinWorkflows: "/api/v1/workflows/builtin", workflowRuns: "/api/v1/workflow-runs", capabilities: "/api/v1/capabilities", integrations: "/api/v1/integrations" } as const;
