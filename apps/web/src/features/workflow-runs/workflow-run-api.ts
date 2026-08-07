import type { ApiRequestInit } from "../../lib/api.ts";
import { ApiError, apiRequest } from "../../lib/api.ts";
import type { WorkflowStage } from "../workflow-bindings/workflow-binding-api.ts";

export type WorkflowRunStatus = "queued" | "running" | "cancelling" | "succeeded" | "failed" | "cancelled" | "timed_out";
export type WorkflowRunDisplayStatus = WorkflowRunStatus | "output_validation_failed" | "result_consumption_failed";
export type WorkflowRunTriggerSource = "manual" | "retry" | "system" | "api";
export type WorkflowRunRetryMode = "current_configuration" | "original_configuration";
export type WorkflowRunRetryability = "runtime_retry" | "result_consumption_retry" | "not_retryable";
export type WorkflowRunDto = { id: string; runNumber: string; projectId: string; stage: WorkflowStage; subjectType: string | null; subjectId: string | null; workflowConfigurationId: string; workflowName: string | null; workflowConfigurationVersion: number | null; triggerSource: WorkflowRunTriggerSource; retryOfRunId: string | null; status: WorkflowRunStatus; displayStatus: WorkflowRunDisplayStatus; failurePhase: "external_execution" | "output_validation" | "result_consumption" | "cancellation" | null; failureCode: string | null; safeError: { code: string; message: string; details?: Record<string, unknown> } | null; domainImpact: Array<{ resourceType: string; resourceId: string | null; outcome: string }>; connectionSummary: { id: string; name: string; connectionType: string; validationStatus: string; enabled: boolean; executable: boolean } | null; llmPolicySummary: { strategy: string; providerId: string | null; providerName: string | null; providerVersion: number | null; model: string | null; validationStatus: string | null; executable: boolean } | null; retryability: WorkflowRunRetryability; retryMode: WorkflowRunRetryMode | null; externalExecutionId: string | null; cancellationRequestedAt: string | null; timedOutAt: string | null; inputPayload: Record<string, unknown>; outputPayload: Record<string, unknown> | null; errorCode: string | null; errorMessage: string | null; errorDetails: Record<string, unknown> | null; configurationSnapshot: Record<string, unknown>; bindingSnapshot: { bindingId: string; bindingVersion: number; stage: WorkflowStage }; connectionSnapshot: Record<string, unknown>; llmPolicySnapshot: Record<string, unknown>; startedAt: string | null; finishedAt: string | null; cancelledAt: string | null; createdAt: string; updatedAt: string; version: number };
export type WorkflowRunList = { items: WorkflowRunDto[]; total: number; limit: number; offset: number };
export type WorkflowRunListQuery = { projectId?: string; stage?: WorkflowStage; workflowConfigurationId?: string; status?: WorkflowRunStatus; displayStatus?: WorkflowRunDisplayStatus; connectionId?: string; providerId?: string; model?: string; configurationVersion?: number; retryability?: "runtime_retry" | "result_consumption_retry" | "not_retryable"; triggerSource?: WorkflowRunTriggerSource; q?: string; from?: string; to?: string; startTime?: string; endTime?: string; limit?: number; offset?: number };
export type WorkflowRunVm = {
  id: string;
  runNumber: string;
  projectId: string;
  stageLabel: string;
  status: WorkflowRunDisplayStatus | "unknown";
  statusLabel: string;
  triggerSourceLabel: string;
  workflowName: string;
  workflowVersionLabel: string;
  connectionName: string;
  modelStrategyLabel: string;
  startedAtLabel: string;
  durationLabel: string;
  retryLabel: string;
  createdAtLabel: string;
  updatedAtLabel: string;
  subjectLabel: string;
};

const stageLabels: Record<WorkflowStage, string> = { chapter_planning: "章节规划", content_generation: "内容生成", review: "内容审核", rewrite: "内容改写" };
const statusLabels: Record<WorkflowRunDisplayStatus, string> = { queued: "等待执行", running: "运行中", cancelling: "正在取消", succeeded: "已成功", failed: "执行失败", cancelled: "已取消", timed_out: "执行超时", output_validation_failed: "输出校验失败", result_consumption_failed: "结果提交失败" };
const triggerSourceLabels: Record<WorkflowRunTriggerSource, string> = { manual: "手动触发", retry: "重试触发", system: "系统触发", api: "API 触发" };

const isWorkflowStage = (value: string): value is WorkflowStage => value in stageLabels;
const isWorkflowRunStatus = (value: string): value is WorkflowRunDisplayStatus => value in statusLabels;
const isWorkflowRunTriggerSource = (value: string): value is WorkflowRunTriggerSource => value in triggerSourceLabels;

export function formatWorkflowRunTime(value: string | null | undefined) {
  if (!value) return "—";
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "—" : new Intl.DateTimeFormat("zh-CN", { dateStyle: "medium", timeStyle: "short" }).format(date);
}

export function workflowRunQuery(query: WorkflowRunListQuery = {}) {
  const params = new URLSearchParams({ limit: String(query.limit ?? 20), offset: String(query.offset ?? 0) });
  if (query.projectId?.trim()) params.set("projectId", query.projectId.trim());
  if (query.stage) params.set("stage", query.stage);
  if (query.status) params.set("status", query.status);
  if (query.displayStatus) params.set("displayStatus", query.displayStatus);
  if (query.workflowConfigurationId) params.set("workflowConfigurationId", query.workflowConfigurationId);
  if (query.connectionId) params.set("connectionId", query.connectionId);
  if (query.providerId) params.set("providerId", query.providerId);
  if (query.model) params.set("model", query.model);
  if (query.configurationVersion !== undefined) params.set("configurationVersion", String(query.configurationVersion));
  if (query.retryability) params.set("retryability", query.retryability);
  if (query.triggerSource) params.set("triggerSource", query.triggerSource);
  if (query.q?.trim()) params.set("q", query.q.trim());
  if (query.from) params.set("from", query.from);
  if (query.to) params.set("to", query.to);
  if (query.startTime) params.set("startTime", query.startTime);
  if (query.endTime) params.set("endTime", query.endTime);
  return params;
}

function formatDuration(startedAt: string | null, finishedAt: string | null, status: string) {
  if (!startedAt) return "—";
  const start = new Date(startedAt).getTime();
  if (Number.isNaN(start)) return "—";
  const end = finishedAt ? new Date(finishedAt).getTime() : (status === "running" || status === "cancelling" ? Date.now() : NaN);
  if (Number.isNaN(end) || end < start) return "—";
  const totalSec = Math.floor((end - start) / 1000);
  const h = Math.floor(totalSec / 3600);
  const m = Math.floor((totalSec % 3600) / 60);
  const s = totalSec % 60;
  if (h > 0) return `${h}时${m}分`;
  if (m > 0) return `${m}分${s}秒`;
  return `${s}秒`;
}

export const mapWorkflowRun = (item: WorkflowRunDto): WorkflowRunVm => {
  const displayStatus = item.displayStatus ?? item.status;
  const knownStatus = isWorkflowRunStatus(displayStatus) ? displayStatus : undefined;
  const strategy = item.llmPolicySummary?.strategy;
  const strategyLabel =
    strategy === "acf_managed"
      ? [item.llmPolicySummary?.providerName, item.llmPolicySummary?.model].filter(Boolean).join(" / ") || "ACF 托管"
      : strategy === "n8n_managed"
        ? "n8n 内部模型"
        : strategy === "none"
          ? "不使用模型"
          : "—";
  return {
    id: item.id,
    runNumber: item.runNumber || "—",
    projectId: item.projectId,
    stageLabel: isWorkflowStage(item.stage) ? stageLabels[item.stage] : "未知环节",
    status: knownStatus ?? "unknown",
    statusLabel: knownStatus ? statusLabels[knownStatus] : "未知状态",
    triggerSourceLabel: isWorkflowRunTriggerSource(item.triggerSource) ? triggerSourceLabels[item.triggerSource] : "未知来源",
    workflowName: item.workflowName || "工作流配置已不可用",
    workflowVersionLabel: item.workflowConfigurationVersion != null ? `v${item.workflowConfigurationVersion}` : "—",
    connectionName: item.connectionSummary?.name || "—",
    modelStrategyLabel: strategyLabel,
    startedAtLabel: formatWorkflowRunTime(item.startedAt ?? item.createdAt),
    durationLabel: formatDuration(item.startedAt, item.finishedAt, item.status),
    retryLabel: item.retryOfRunId ? "由重试创建" : "首次执行",
    createdAtLabel: formatWorkflowRunTime(item.createdAt),
    updatedAtLabel: formatWorkflowRunTime(item.updatedAt),
    subjectLabel: item.subjectType ? `${item.subjectType}` : "—"
  };
};

export async function listWorkflowRuns(query: WorkflowRunListQuery = {}, init?: ApiRequestInit): Promise<Omit<WorkflowRunList, "items"> & { items: WorkflowRunVm[] }> {
  const response = await apiRequest<WorkflowRunList>(`/workflow-runs?${workflowRunQuery(query)}`, init);
  return { ...response, items: response.items.map(mapWorkflowRun) };
}

export type WorkflowRunEventDto = { id: string; runId: string; eventType: string; status: string; payload: Record<string, unknown> | null; createdAt: string | null };
export type WorkflowRunEventVm = { id: string; eventType: string; title: string; statusLabel: string; createdAtLabel: string; payload: Record<string, unknown> | null };
export type WorkflowRunDetailVm = Omit<WorkflowRunDto, Exclude<keyof WorkflowRunVm, "status">> & Omit<WorkflowRunVm, "status"> & { canCancel: boolean; canRetry: boolean; canRetryResultConsumption: boolean };
const eventLabels: Record<string, string> = { queued: "已创建运行", worker_started: "开始执行", request_sent: "已发送请求", response_received: "已收到响应", output_validated: "已校验输出", succeeded: "运行成功", failed: "运行失败", cancelled: "已取消运行", retry_created: "已创建重试运行" };
const redact = (value: unknown): unknown => {
  if (Array.isArray(value)) return value.map(redact);
  if (value && typeof value === "object") return Object.fromEntries(Object.entries(value).map(([key, item]) => [/token|secret|password|authorization|api.?key/i.test(key) ? [key, "[已隐藏]"] : [key, redact(item)]]));
  return value;
};
export const formatWorkflowRunJson = (value: unknown) => value && typeof value === "object" && Object.keys(value).length ? JSON.stringify(redact(value), null, 2) : "暂无信息";
export const mapWorkflowRunDetail = (item: WorkflowRunDto): WorkflowRunDetailVm => { const consumption = item.retryability === "result_consumption_retry" || item.failurePhase === "result_consumption"; return ({ ...item, ...mapWorkflowRun(item), status: item.status, inputPayload: item.inputPayload ?? {}, configurationSnapshot: item.configurationSnapshot ?? {}, canCancel: item.status === "queued" || item.status === "running" || item.status === "cancelling", canRetry: !consumption && (item.retryability === "runtime_retry" || item.status === "failed" || item.status === "cancelled" || item.status === "timed_out"), canRetryResultConsumption: consumption }); };
export const mapWorkflowRunEvent = (item: WorkflowRunEventDto): WorkflowRunEventVm => ({ id: item.id, eventType: item.eventType, title: eventLabels[item.eventType] ?? "未知运行事件", statusLabel: isWorkflowRunStatus(item.status) ? statusLabels[item.status] : "未知状态", createdAtLabel: formatWorkflowRunTime(item.createdAt), payload: item.payload ?? null });
const runPath = (runId: string) => `/workflow-runs/${encodeURIComponent(runId)}`;
export const getWorkflowRun = async (runId: string, init?: ApiRequestInit) => mapWorkflowRunDetail(await apiRequest<WorkflowRunDto>(runPath(runId), init));
export const listWorkflowRunEvents = async (runId: string, init?: ApiRequestInit) => (await apiRequest<{ items: WorkflowRunEventDto[] }>(`${runPath(runId)}/events`, init)).items.map(mapWorkflowRunEvent);
export const cancelWorkflowRun = async (runId: string, expectedVersion: number, key: string) => mapWorkflowRunDetail(await apiRequest<WorkflowRunDto>(`${runPath(runId)}/cancel`, { method: "POST", headers: { "Content-Type": "application/json", "Idempotency-Key": key }, body: JSON.stringify({ expectedVersion }) }));
export type WorkflowRunRetryOptions = { runId: string; retryability: WorkflowRunRetryability; currentConfiguration: { mode: "current_configuration"; enabled: boolean; reasons: Array<{ code: string; message: string; repairAction?: string }>; configurationDifferences?: Array<{ field: string; changed: boolean; summary: string }> }; originalConfiguration: { mode: "original_configuration"; enabled: boolean; reasons: Array<{ code: string; message: string; repairAction?: string }>; configurationDifferences?: Array<{ field: string; changed: boolean; summary: string }> }; resultConsumptionRetryRequired: boolean };
export const getWorkflowRunRetryOptions = (runId: string, init?: ApiRequestInit) => apiRequest<WorkflowRunRetryOptions>(`${runPath(runId)}/retry-options`, init);
export const retryWorkflowRun = async (runId: string, expectedVersion: number, key: string, mode: WorkflowRunRetryMode, inputOverride?: Record<string, unknown>, reason?: string) => mapWorkflowRunDetail(await apiRequest<WorkflowRunDto>(`${runPath(runId)}/retries`, { method: "POST", headers: { "Content-Type": "application/json", "Idempotency-Key": key }, body: JSON.stringify({ expectedVersion, mode, ...(reason !== undefined ? { reason } : {}), ...(inputOverride ? { inputOverride } : {}) }) }));
export const retryWorkflowRunResultConsumption = async (run: Pick<WorkflowRunDto, "id" | "stage" | "version">, key: string) => {
  const encoded = encodeURIComponent(run.id);
  const path = run.stage === "chapter_planning" ? `/workflow-runs/${encoded}/chapter-planning-result-consumption-retries`
    : run.stage === "content_generation" ? `/content-generation-runs/${encoded}/result-consumption-retries`
      : run.stage === "review" ? `/workflow-runs/${encoded}/review-result-consumption-retries`
        : run.stage === "rewrite" ? `/workflow-runs/${encoded}/rewrite-result-consumption-retries` : "";
  if (!path) throw new ApiError("该阶段不支持结果消费重试。", 409, "result_consumption_not_supported");
  return apiRequest<unknown>(path, { method: "POST", headers: { "Content-Type": "application/json", "Idempotency-Key": key }, body: JSON.stringify({ expectedRunVersion: run.version }) });
};

export type ProjectWorkflowRunSummaryDto = {
  totalRuns: number;
  activeRuns: number;
  recentFailedRuns: number;
  lastRunAt: string | null;
  recentRuns: WorkflowRunDto[];
};
export type ProjectWorkflowRunSummaryVm = {
  totalRuns: number;
  activeRuns: number;
  recentFailedRuns: number;
  lastRunAtLabel: string;
  recentRuns: WorkflowRunVm[];
};

const safeCount = (value: number) => Number.isFinite(value) && value >= 0 ? value : 0;
export const mapProjectWorkflowRunSummary = (summary: ProjectWorkflowRunSummaryDto): ProjectWorkflowRunSummaryVm => ({
  totalRuns: safeCount(summary.totalRuns),
  activeRuns: safeCount(summary.activeRuns),
  recentFailedRuns: safeCount(summary.recentFailedRuns),
  lastRunAtLabel: formatWorkflowRunTime(summary.lastRunAt),
  recentRuns: Array.isArray(summary.recentRuns) ? summary.recentRuns.slice(0, 3).map(mapWorkflowRun) : [],
});
export const getProjectWorkflowRunSummary = async (projectId: string, init?: ApiRequestInit) =>
  mapProjectWorkflowRunSummary(await apiRequest<ProjectWorkflowRunSummaryDto>(`/projects/${encodeURIComponent(projectId)}/workflow-run-summary`, init));
export const createWorkflowRun = async (projectId: string, stage: WorkflowStage, inputPayload: Record<string, unknown>, key: string) =>
  mapWorkflowRunDetail(await apiRequest<WorkflowRunDto>(`/workflow-runs`, { method: "POST", headers: { "Content-Type": "application/json", "Idempotency-Key": key }, body: JSON.stringify({ projectId, stage, inputPayload }) }));
