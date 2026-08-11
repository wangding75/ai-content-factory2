import { apiRequest, type ApiRequestInit } from "@/lib/api";

export type WorkflowStage = "chapter_planning" | "content_generation" | "review" | "rewrite";
export type WorkflowConfiguration = { id:string; name:string; note?:string; connectionId:string; connectionName:string; connectionType:string; workflowType:string; applicableStages:WorkflowStage[]; integrationStatus:"connected"|"not_connected"|"connection_error"|string; connectionStatus?:"connected"|"disconnected"|"error"|string; enabled:boolean; version:number; updatedAt:string; lastErrorMessage:string|null };
export type NonExecutableReason = { code: string; message: string; repairAction?: string; repairTarget?: { providerId?: string; connectionId?: string; workflowConfigurationId?: string; projectId?: string; stage?: WorkflowStage } };
export type ConnectionSummary = { id:string; name:string; connectionType:string; validationStatus:string; enabled:boolean; executable:boolean };
export type LlmPolicySummary = { strategy:"none"|"n8n_managed"|"acf_managed"|string; providerId:string|null; providerName:string|null; providerVersion:number|null; model:string|null; validationStatus:string|null; executable:boolean };
export type WorkflowBindingCandidate = { stage: WorkflowStage; selectable: boolean; executable: boolean; ineligibilityReasons: NonExecutableReason[]; workflowConfiguration: WorkflowConfiguration; connectionSummary: ConnectionSummary; llmPolicySummary: LlmPolicySummary };
const ineligibilityReasonLabels:Record<string,string>={
  connection_verification_failed:"连接验证未通过。",
  workflow_configuration_verification_failed:"工作流配置验证未通过。",
  connection_disabled:"连接已停用。",
  workflow_configuration_disabled:"工作流配置已停用。",
  connection_credential_unavailable:"连接凭据不可用。",
  provider_verification_failed:"模型提供方验证未通过。",
  provider_disabled:"模型提供方已停用。",
  model_unavailable:"所选模型不可用。",
  llm_strategy_incomplete:"LLM 策略尚未配置完整。",
  workflow_stage_mismatch:"工作流不适用于当前环节。",
  input_contract_incompatible:"工作流输入契约不兼容。",
  output_contract_incompatible:"工作流输出契约不兼容。",
};
export const formatIneligibilityReason=(reason:NonExecutableReason)=>ineligibilityReasonLabels[reason.code]??reason.message;
export type Binding = { id:string; projectId:string; stage:WorkflowStage; workflowConfigurationId:string; version:number; createdAt:string; updatedAt:string };
export type BindingStage = { stage:WorkflowStage; bound:boolean; binding:Binding|null; workflowConfigurationSummary:WorkflowConfiguration|null };
export const stageOrder:WorkflowStage[]=["chapter_planning","content_generation","review","rewrite"];
export const stageLabels:Record<WorkflowStage,string>={chapter_planning:"章节规划",content_generation:"内容生成",review:"审核",rewrite:"改写"};
export const stageDescriptions:Record<WorkflowStage,string>={
  chapter_planning:"自动化拆解大纲并生成章节详细规划。",
  content_generation:"基于章节规划进行多模态内容初稿生成。",
  review:"针对生成内容进行合规性、一致性与质量审查。",
  rewrite:"根据反馈或风格需求对现有内容进行重组与优化。"
};
export const formatWorkflowNote=(note:string)=>Object.entries(stageLabels).reduce((value,[stage,label])=>value.replaceAll(stage,label),note);

export type WorkflowExceptionType = "none" | "disabled" | "integration_error" | "connection_error";

export function getWorkflowExceptionType(workflow: WorkflowConfiguration | null): WorkflowExceptionType {
  if (!workflow) return "none";
  if (!workflow.enabled) return "disabled";
  if (workflow.integrationStatus === "not_connected") return "integration_error";
  if (
    workflow.integrationStatus === "connection_error" ||
    workflow.integrationStatus === "error" ||
    workflow.connectionStatus === "disconnected" ||
    workflow.connectionStatus === "error" ||
    Boolean(workflow.lastErrorMessage) ||
    workflow.connectionName === "连接异常"
  ) {
    return "connection_error";
  }
  return "none";
}

export const bindingCopy = (item: BindingStage) => {
  const workflow = item.workflowConfigurationSummary;
  const exc = getWorkflowExceptionType(workflow);
  let statusText = "已绑定 · 可执行";
  if (!item.bound) {
    statusText = "尚未绑定工作流";
  } else if (exc === "disabled") {
    statusText = "已绑定 · 已停用";
  } else if (exc === "integration_error") {
    statusText = "已绑定 · 依赖失效";
  } else if (exc === "connection_error") {
    statusText = "已绑定 · 连接异常";
  }

  return {
    bound: item.bound ? "已绑定" : "未绑定",
    statusText,
    exceptionType: exc,
    enabled: workflow ? (workflow.enabled ? "已启用" : "已停用") : "--",
    integration: workflow ? (workflow.integrationStatus === "not_connected" ? "未接入" : exc === "connection_error" ? "连接异常" : "已集成") : "--",
    connection: workflow ? (workflow.connectionName || "无关联连接") : "无关联连接",
  };
};
export function listProjectWorkflowBindings(projectId:string,init?:ApiRequestInit){return apiRequest<{items:BindingStage[]}>(`/projects/${encodeURIComponent(projectId)}/workflow-bindings`,init)}
export function listApplicableWorkflows(stage:WorkflowStage,query:string,init?:ApiRequestInit){const p=new URLSearchParams({applicableStage:stage,limit:"100",offset:"0"});if(query.trim())p.set("q",query.trim());return apiRequest<{items:WorkflowConfiguration[];total:number}>(`/workflow-configurations?${p}`,init)}
export function listWorkflowBindingCandidates(projectId:string,stage:WorkflowStage,query:string,init?:ApiRequestInit){const p=new URLSearchParams({limit:"100",offset:"0"});if(query.trim())p.set("q",query.trim());return apiRequest<{items:WorkflowBindingCandidate[];total:number}>(`/projects/${encodeURIComponent(projectId)}/workflow-bindings/${stage}/candidates?${p}`,init)}
const headers=(key:string)=>({"Content-Type":"application/json","Idempotency-Key":key});
export function bindWorkflow(projectId:string,stage:WorkflowStage,workflowConfigurationId:string,expectedVersion:number|undefined,key:string){const body={workflowConfigurationId,...(expectedVersion===undefined?{}:{expectedVersion})};return apiRequest<BindingStage>(`/projects/${encodeURIComponent(projectId)}/workflow-bindings/${stage}`,{method:"PUT",headers:headers(key),body:JSON.stringify(body)})}
export function unbindWorkflow(projectId:string,stage:WorkflowStage,expectedVersion:number,key:string){const p=new URLSearchParams({expected_version:String(expectedVersion)});return apiRequest<{projectId:string;stage:WorkflowStage;unbound:true;workflowConfigurationRetained:true}>(`/projects/${encodeURIComponent(projectId)}/workflow-bindings/${stage}?${p}`,{method:"DELETE",headers:{"Idempotency-Key":key}})}
export const newIdempotencyKey=()=>globalThis.crypto?.randomUUID?.()??`${Date.now()}-${Math.random()}`;
