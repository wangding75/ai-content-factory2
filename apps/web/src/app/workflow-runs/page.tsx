import { AppShell } from "@/components/ui/app-shell";
import { WorkflowRunsPage } from "@/features/workflow-runs/workflow-runs-page";
import { stageOrder, type WorkflowStage } from "@/features/workflow-bindings/workflow-binding-api";

function initialStage(value?: string): WorkflowStage | undefined {
  return stageOrder.includes(value as WorkflowStage) ? value as WorkflowStage : undefined;
}

export default async function Page({ searchParams }: { searchParams: Promise<{ projectId?: string; stage?: string }> }) {
  const { projectId, stage } = await searchParams;
  return <AppShell active="workflows"><WorkflowRunsPage projectId={projectId?.trim() || undefined} initialStage={initialStage(stage)} /></AppShell>;
}
