import type { WorkflowPreflightBlockerReason } from "./workflow-preflight-blocker";
import type { WorkflowPreflightRepairTarget } from "./workflow-preflight-repair-route";

export type WorkflowPreflightReasonSource = {
  code: string;
  message: string;
  repairAction?: string;
  repairTarget?: WorkflowPreflightRepairTarget | null;
};

export function toWorkflowPreflightReasons(
  reasons: WorkflowPreflightReasonSource[],
): WorkflowPreflightBlockerReason[] {
  return reasons.map((reason) => ({
    code: reason.code,
    message: reason.message,
    repairAction: reason.repairAction,
    providerId: reason.repairTarget?.providerId,
    connectionId: reason.repairTarget?.connectionId,
    workflowConfigurationId: reason.repairTarget?.workflowConfigurationId,
    projectId: reason.repairTarget?.projectId,
    stage: reason.repairTarget?.stage,
  }));
}
