import type { ApiRequestInit } from "@/lib/api";
import { apiRequest } from "@/lib/api";
import type { WorkflowStage } from "@/features/workflow-bindings/workflow-binding-api";

export type WorkflowRunStatus = "queued" | "running" | "succeeded" | "failed" | "cancelled";
export type WorkflowRunTriggerSource = "manual" | "retry" | "system" | "api";
export type WorkflowRunDto = { id: string; runNumber: string; projectId: string; stage: string; workflowConfigurationId: string; triggerSource: string; status: string; inputPayload: Record<string, unknown>; outputPayload: Record<string, unknown> | null; errorCode: string | null; errorMessage: string | null; errorDetails: Record<string, unknown> | null; configurationSnapshot: Record<string, unknown>; startedAt: string | null; finishedAt: string | null; cancelledAt: string | null; createdAt: string | null; updatedAt: string | null; version: number };
export type WorkflowRunList = { items: WorkflowRunDto[]; total: number; limit: number; offset: number };
export type WorkflowRunListQuery = { projectId?: string; stage?: WorkflowStage; status?: WorkflowRunStatus; q?: string; startTime?: string; endTime?: string; limit?: number; offset?: number };
export type WorkflowRunVm = { id: string; runNumber: string; projectId: string; stageLabel: string; status: WorkflowRunStatus | "unknown"; statusLabel: string; triggerSourceLabel: string; createdAtLabel: string; updatedAtLabel: string };

const stageLabels: Record<WorkflowStage, string> = { chapter_planning: "章节规划", content_generation: "内容生成", review: "内容审核", rewrite: "内容改写" };
const statusLabels: Record<WorkflowRunStatus, string> = { queued: "等待执行", running: "运行中", succeeded: "已成功", failed: "失败", cancelled: "已取消" };
const triggerSourceLabels: Record<WorkflowRunTriggerSource, string> = { manual: "手动触发", retry: "重试触发", system: "系统触发", api: "API 触发" };

const isWorkflowStage = (value: string): value is WorkflowStage => value in stageLabels;
const isWorkflowRunStatus = (value: string): value is WorkflowRunStatus => value in statusLabels;
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
  if (query.q?.trim()) params.set("q", query.q.trim());
  if (query.startTime) params.set("startTime", query.startTime);
  if (query.endTime) params.set("endTime", query.endTime);
  return params;
}

export const mapWorkflowRun = (item: WorkflowRunDto): WorkflowRunVm => ({
  id: item.id,
  runNumber: item.runNumber || "—",
  projectId: item.projectId,
  stageLabel: isWorkflowStage(item.stage) ? stageLabels[item.stage] : "未知环节",
  status: isWorkflowRunStatus(item.status) ? item.status : "unknown",
  statusLabel: isWorkflowRunStatus(item.status) ? statusLabels[item.status] : "未知状态",
  triggerSourceLabel: isWorkflowRunTriggerSource(item.triggerSource) ? triggerSourceLabels[item.triggerSource] : "未知来源",
  createdAtLabel: formatWorkflowRunTime(item.createdAt),
  updatedAtLabel: formatWorkflowRunTime(item.updatedAt),
});

export async function listWorkflowRuns(query: WorkflowRunListQuery = {}, init?: ApiRequestInit): Promise<Omit<WorkflowRunList, "items"> & { items: WorkflowRunVm[] }> {
  const response = await apiRequest<WorkflowRunList>(`/workflow-runs?${workflowRunQuery(query)}`, init);
  return { ...response, items: response.items.map(mapWorkflowRun) };
}

export type WorkflowRunEventDto = { id: string; runId: string; eventType: string; status: string; payload: Record<string, unknown> | null; createdAt: string | null };
export type WorkflowRunEventVm = { id: string; title: string; statusLabel: string; createdAtLabel: string; payload: Record<string, unknown> | null };
export type WorkflowRunDetailVm = WorkflowRunVm & { version: number; inputPayload: Record<string, unknown>; outputPayload: Record<string, unknown> | null; errorCode: string | null; errorMessage: string | null; errorDetails: Record<string, unknown> | null; configurationSnapshot: Record<string, unknown>; canCancel: boolean; canRetry: boolean };
const eventLabels: Record<string, string> = { queued: "已创建运行", worker_started: "开始执行", request_sent: "已发送请求", response_received: "已收到响应", output_validated: "已校验输出", succeeded: "运行成功", failed: "运行失败", cancelled: "已取消运行", retry_created: "已创建重试运行" };
const redact = (value: unknown): unknown => {
  if (Array.isArray(value)) return value.map(redact);
  if (value && typeof value === "object") return Object.fromEntries(Object.entries(value).map(([key, item]) => [/token|secret|password|authorization|api.?key/i.test(key) ? [key, "[已隐藏]"] : [key, redact(item)]]));
  return value;
};
export const formatWorkflowRunJson = (value: unknown) => value && typeof value === "object" && Object.keys(value).length ? JSON.stringify(redact(value), null, 2) : "暂无信息";
export const mapWorkflowRunDetail = (item: WorkflowRunDto): WorkflowRunDetailVm => ({ ...mapWorkflowRun(item), version: item.version, inputPayload: item.inputPayload ?? {}, outputPayload: item.outputPayload, errorCode: item.errorCode, errorMessage: item.errorMessage, errorDetails: item.errorDetails, configurationSnapshot: item.configurationSnapshot ?? {}, canCancel: item.status === "queued" || item.status === "running", canRetry: item.status === "failed" || item.status === "cancelled" });
export const mapWorkflowRunEvent = (item: WorkflowRunEventDto): WorkflowRunEventVm => ({ id: item.id, title: eventLabels[item.eventType] ?? "未知运行事件", statusLabel: isWorkflowRunStatus(item.status) ? statusLabels[item.status] : "未知状态", createdAtLabel: formatWorkflowRunTime(item.createdAt), payload: item.payload ?? null });
const runPath = (runId: string) => `/workflow-runs/${encodeURIComponent(runId)}`;
export const getWorkflowRun = async (runId: string, init?: ApiRequestInit) => mapWorkflowRunDetail(await apiRequest<WorkflowRunDto>(runPath(runId), init));
export const listWorkflowRunEvents = async (runId: string, init?: ApiRequestInit) => (await apiRequest<{ items: WorkflowRunEventDto[] }>(`${runPath(runId)}/events`, init)).items.map(mapWorkflowRunEvent);
export const cancelWorkflowRun = async (runId: string, expectedVersion: number, key: string) => mapWorkflowRunDetail(await apiRequest<WorkflowRunDto>(`${runPath(runId)}/cancel`, { method: "POST", headers: { "Content-Type": "application/json", "Idempotency-Key": key }, body: JSON.stringify({ expectedVersion }) }));
export const retryWorkflowRun = async (runId: string, expectedVersion: number, key: string, inputOverride?: Record<string, unknown>) => mapWorkflowRunDetail(await apiRequest<WorkflowRunDto>(`${runPath(runId)}/retries`, { method: "POST", headers: { "Content-Type": "application/json", "Idempotency-Key": key }, body: JSON.stringify({ expectedVersion, useCurrentConfiguration: false, ...(inputOverride ? { inputOverride } : {}) }) }));
