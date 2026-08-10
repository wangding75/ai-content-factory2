import type { WorkflowPreflightBlockerReason } from "./workflow-preflight-blocker";
import type { WorkflowPreflightRepairTarget } from "./workflow-preflight-repair-route";

export type WorkflowPreflightReasonSource = {
  code: string;
  message: string;
  safeReason?: string;
  retryAction?: string;
  repairAction?: string;
  repairTarget?: WorkflowPreflightRepairTarget | null;
};

export function toWorkflowPreflightReasons(
  reasons: WorkflowPreflightReasonSource[],
): WorkflowPreflightBlockerReason[] {
  return reasons.map((reason) => ({
    code: reason.code,
    message: reason.message,
    ...(reason.safeReason ? { safeReason: reason.safeReason } : {}),
    ...(reason.retryAction ? { retryAction: reason.retryAction } : {}),
    repairAction: reason.repairAction,
    providerId: reason.repairTarget?.providerId,
    connectionId: reason.repairTarget?.connectionId,
    workflowConfigurationId: reason.repairTarget?.workflowConfigurationId,
    projectId: reason.repairTarget?.projectId,
    stage: reason.repairTarget?.stage,
  }));
}
