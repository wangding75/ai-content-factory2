import { AppShell } from "@/components/ui/app-shell";
import { ProjectWorkspaceLoader } from "@/features/planning-materials/components/project-workspace-loader";

export default async function ProjectCandidateBatchDetailRoute({
  params,
}: {
  params: Promise<{ projectId: string; batchId: string }>;
}) {
  const { projectId, batchId } = await params;
  return (
    <AppShell active="projects">
      <ProjectWorkspaceLoader projectId={projectId} page="candidate-batch-detail" batchId={batchId} />
    </AppShell>
  );
}
