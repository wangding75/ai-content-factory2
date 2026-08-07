import { apiRequest, type ApiRequestInit } from "../../lib/api.ts";
import type { SafeError, ValidationCheck, ValidationStatus } from "./llm-provider-api.ts";

export type ApplicableStage = "chapter_planning" | "content_generation" | "review" | "rewrite";
export type LlmStrategy = "acf_managed" | "n8n_managed" | "none";
export type NonExecutableReason = { code: string; message: string; repairAction?: string };
export type WorkflowDto = { id:string; name:string; connectionId:string; connectionName:string; connectionType:string; workflowType:string; applicableStages:ApplicableStage[]; typeConfig:{referenceType:"workflow_id"|"webhook_path";referenceValue:string}; inputContractVersion:string; outputContractVersion:string; defaultParameters:Record<string,unknown>; note:string|null; llmStrategy?:LlmStrategy; llmProviderId?:string|null; llmModel?:string|null; validationStatus?:ValidationStatus; enabled:boolean; executable?:boolean; ineligibilityReasons?:NonExecutableReason[]; safeError?:SafeError|null; checks?:ValidationCheck[]; llmPolicySummary?:{strategy:LlmStrategy;providerName:string|null;model:string|null;validationStatus:ValidationStatus|null;executable:boolean}; integrationStatus:"not_connected"|"connected"|"unverified"|"verified"|"failed"; lastVerifiedAt:string|null; lastErrorCode:string|null; lastErrorMessage:string|null; version:number; createdAt:string; updatedAt:string };
export type WorkflowVm = {
  id: string;
  name: string;
  shortId: string;
  connectionName: string;
  connectionTypeLabel: string;
  stagesLabel: string;
  strategyLabel: string;
  strategyDetail: string | null;
  validationStatus: ValidationStatus;
  validationLabel: string;
  enabled: boolean;
  enabledLabel: string;
  statusLabel: string;
  executable: boolean;
  reasons: string[];
  version: number;
};
export type WorkflowForm = { name:string; connectionId:string; applicableStages:ApplicableStage[]; referenceType:"workflow_id"|"webhook_path"; referenceValue:string; inputContractVersion:string; outputContractVersion:string; defaultParametersJson:string; note:string; llmStrategy:LlmStrategy; llmProviderId:string; llmModel:string };
export type WorkflowListQuery = { q?:string; connectionType?:string; applicableStage?:ApplicableStage; integrationStatus?: "not_connected" | "connected" | "unverified" | "verified" | "failed"; validationStatus?:ValidationStatus; llmStrategy?:LlmStrategy; enabled?:boolean; executable?:boolean; limit?:number; offset?:number };

const stageLabels:Record<ApplicableStage,string>={chapter_planning:"章节规划",content_generation:"内容生成",review:"审核",rewrite:"重写"};
const object = (value:string) => { const parsed:unknown=JSON.parse(value); if (!parsed || Array.isArray(parsed) || typeof parsed !== "object") throw new Error("not_object"); return parsed as Record<string,unknown>; };
const strategyLabels: Record<LlmStrategy,string> = { acf_managed:"ACF 托管",n8n_managed:"n8n 内部模型",none:"不使用模型" };
const validationLabels: Record<ValidationStatus, string> = {
  unverified: "未验证",
  verifying: "验证中",
  verified: "验证成功",
  failed: "验证失败",
  stale: "配置已变更"
};
export const mapWorkflow = (item: WorkflowDto): WorkflowVm => {
  const validationStatus = item.validationStatus ?? "unverified";
  const strategy = item.llmStrategy ?? "none";
  const providerName = item.llmPolicySummary?.providerName ?? null;
  const model = item.llmModel ?? item.llmPolicySummary?.model ?? null;
  let strategyDetail: string | null = null;
  if (strategy === "acf_managed") {
    strategyDetail = [providerName, model].filter(Boolean).join(" / ") || model;
  } else if (strategy === "n8n_managed") {
    strategyDetail = model;
  }
  const executable = item.executable === true;
  return {
    id: item.id,
    name: item.name,
    shortId: item.id.length > 12 ? `${item.id.slice(0, 8)}…` : item.id,
    connectionName: item.connectionName,
    connectionTypeLabel: item.connectionType === "n8n" ? "n8n" : "未知连接类型",
    stagesLabel: item.applicableStages.map(stage => stageLabels[stage] ?? "未知环节").join("、"),
    strategyLabel: strategyLabels[strategy] ?? "未知策略",
    strategyDetail,
    validationStatus,
    validationLabel: validationLabels[validationStatus] ?? "未验证",
    enabled: item.enabled,
    enabledLabel: item.enabled ? "已启用" : "未启用",
    statusLabel: executable ? "可执行" : validationStatus === "stale" ? "需重新验证" : item.enabled ? "不可执行" : "未启用",
    executable,
    reasons: (item.ineligibilityReasons ?? []).map(reason => reason.message),
    version: item.version
  };
};
export function validateWorkflow(form:WorkflowForm) { if (!form.name.trim() || !form.connectionId || !form.applicableStages.length || !form.referenceValue.trim() || !form.inputContractVersion.trim() || !form.outputContractVersion.trim()) return "请完整填写必填字段。"; if (form.llmStrategy === "acf_managed" && (!form.llmProviderId || !form.llmModel.trim())) return "平台托管模型需要选择 Provider 和模型。"; if (form.llmStrategy !== "acf_managed" && (form.llmProviderId || form.llmModel)) return "当前模型策略不能填写 Provider 或模型。"; try { object(form.defaultParametersJson); } catch { return "默认参数必须是 JSON 对象。"; } return undefined; }
export const createWorkflowPayload=(form:WorkflowForm)=>({name:form.name.trim(),connectionId:form.connectionId,applicableStages:form.applicableStages,typeConfig:{referenceType:form.referenceType,referenceValue:form.referenceValue.trim()},inputContractVersion:form.inputContractVersion.trim(),outputContractVersion:form.outputContractVersion.trim(),defaultParameters:object(form.defaultParametersJson),note:form.note.trim()||null,llmStrategy:form.llmStrategy,llmProviderId:form.llmStrategy==="acf_managed"?form.llmProviderId:null,llmModel:form.llmStrategy==="acf_managed"?form.llmModel.trim():null});
export const updateWorkflowPayload=(form:WorkflowForm,version:number)=>({expectedVersion:version,...createWorkflowPayload(form)});
const headers=(key:string)=>({"Content-Type":"application/json","Idempotency-Key":key});
export const listWorkflows=(query:WorkflowListQuery={},init?:ApiRequestInit)=>{const params=new URLSearchParams({limit:String(query.limit??20),offset:String(query.offset??0)}); if(query.q?.trim())params.set("q",query.q.trim()); if(query.connectionType)params.set("connectionType",query.connectionType); if(query.applicableStage)params.set("applicableStage",query.applicableStage); if(query.integrationStatus)params.set("integrationStatus",query.integrationStatus); if(query.validationStatus)params.set("validationStatus",query.validationStatus); if(query.llmStrategy)params.set("llmStrategy",query.llmStrategy); if(query.enabled!==undefined)params.set("enabled",String(query.enabled)); if(query.executable!==undefined)params.set("executable",String(query.executable)); return apiRequest<{items:WorkflowDto[];total:number;limit:number;offset:number}>(`/workflow-configurations?${params}`,init);};
export const getWorkflow=(id:string,init?:ApiRequestInit)=>apiRequest<WorkflowDto>(`/workflow-configurations/${encodeURIComponent(id)}`,init);
export const createWorkflow=(form:WorkflowForm,key:string)=>apiRequest<WorkflowDto>("/workflow-configurations",{method:"POST",headers:headers(key),body:JSON.stringify(createWorkflowPayload(form))});
export const updateWorkflow=(id:string,form:WorkflowForm,version:number,key:string)=>apiRequest<WorkflowDto>(`/workflow-configurations/${encodeURIComponent(id)}`,{method:"PATCH",headers:headers(key),body:JSON.stringify(updateWorkflowPayload(form,version))});
export const verifyWorkflow=(id:string,expectedVersion:number,key:string)=>apiRequest<{validationStatus:ValidationStatus;executable:boolean;ineligibilityReasons:NonExecutableReason[];safeError:SafeError|null;checks:ValidationCheck[]}>(`/workflow-configurations/${encodeURIComponent(id)}/verify`,{method:"POST",headers:headers(key),body:JSON.stringify({expectedVersion})});
export const setWorkflowEnabled=(id:string,expectedVersion:number,enabled:boolean,key:string)=>apiRequest<WorkflowDto>(`/workflow-configurations/${encodeURIComponent(id)}/${enabled?"enable":"disable"}`,{method:"POST",headers:headers(key),body:JSON.stringify({expectedVersion})});
