export type WorkflowPreflightRepairTarget = {
  providerId?: string;
  connectionId?: string;
  workflowConfigurationId?: string;
  projectId?: string;
  stage?: string;
};

export type WorkflowPreflightRepairLink = { href: string; known: boolean };

const withQuery = (path: string, query: Record<string, string | undefined>) => {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value?.trim()) params.set(key, value.trim());
  }
  return params.size ? `${path}?${params}` : path;
};

export function workflowPreflightRepairLink(
  action: string | undefined,
  target: WorkflowPreflightRepairTarget = {},
): WorkflowPreflightRepairLink {
  switch (action) {
    case "provider:verify":
    case "provider:enable":
    case "provider:models":
    case "provider:edit":
      return { href: withQuery("/settings", { providerId: target.providerId }), known: true };
    case "connection:verify":
    case "connection:enable":
    case "connection:edit":
      return { href: withQuery("/settings", { tab: "connections", connectionId: target.connectionId }), known: true };
    case "workflow_configuration:verify":
    case "workflow_configuration:enable":
    case "workflow_configuration:edit":
      return { href: withQuery("/settings", { tab: "workflows", workflowConfigurationId: target.workflowConfigurationId }), known: true };
    case "workflow_binding:configure":
    case "workflow_binding:repair":
      return target.projectId && target.stage
        ? { href: withQuery(`/projects/${encodeURIComponent(target.projectId)}/settings`, { tab: "workflow-bindings", stage: target.stage }), known: true }
        : { href: "/settings", known: false };
    default:
      return { href: "/settings", known: false };
  }
}
