import { apiRequest, type ApiRequestInit } from "@/lib/api";

export type WorkflowStage = "chapter_planning" | "content_generation" | "review" | "rewrite";
export type WorkflowConfiguration = { id:string; name:string; note?:string; connectionId:string; connectionName:string; connectionType:string; workflowType:string; applicableStages:WorkflowStage[]; integrationStatus:"connected"|"not_connected"|"connection_error"|string; connectionStatus?:"connected"|"disconnected"|"error"|string; enabled:boolean; version:number; updatedAt:string; lastErrorMessage:string|null };
export type Binding = { id:string; projectId:string; stage:WorkflowStage; workflowConfigurationId:string; version:number; createdAt:string; updatedAt:string };
export type BindingStage = { stage:WorkflowStage; bound:boolean; binding:Binding|null; workflowConfigurationSummary:WorkflowConfiguration|null };
export const stageOrder:WorkflowStage[]=["chapter_planning","content_generation","review","rewrite"];
export const stageLabels:Record<WorkflowStage,string>={chapter_planning:"章节规划",content_generation:"内容生成",review:"审核",rewrite:"改写"};
export const stageDescriptions:Record<WorkflowStage,string>={chapter_planning:"为章节结构与创作目标选择工作流。",content_generation:"为正文内容生成选择工作流。",review:"为内容审核选择工作流。",rewrite:"为审核后的改写选择工作流。"};
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
  let statusText = "已绑定 · 已集成";
  if (!item.bound) {
    statusText = "未绑定";
  } else if (exc === "disabled") {
    statusText = "已绑定 · 已停用";
  } else if (exc === "integration_error") {
    statusText = "已绑定 · 未接入";
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
const headers=(key:string)=>({"Content-Type":"application/json","Idempotency-Key":key});
export function bindWorkflow(projectId:string,stage:WorkflowStage,workflowConfigurationId:string,expectedVersion:number|undefined,key:string){const body={workflowConfigurationId,...(expectedVersion===undefined?{}:{expectedVersion})};return apiRequest<BindingStage>(`/projects/${encodeURIComponent(projectId)}/workflow-bindings/${stage}`,{method:"PUT",headers:headers(key),body:JSON.stringify(body)})}
export function unbindWorkflow(projectId:string,stage:WorkflowStage,expectedVersion:number,key:string){const p=new URLSearchParams({expected_version:String(expectedVersion)});return apiRequest<{projectId:string;stage:WorkflowStage;unbound:true;workflowConfigurationRetained:true}>(`/projects/${encodeURIComponent(projectId)}/workflow-bindings/${stage}?${p}`,{method:"DELETE",headers:{"Idempotency-Key":key}})}
export const newIdempotencyKey=()=>globalThis.crypto?.randomUUID?.()??`${Date.now()}-${Math.random()}`;
