import { AppShell } from "@/components/ui/app-shell";
import { getProjectWorkspace } from "@/lib/api";
import { ProjectWorkspaceFrame } from "@/features/planning-materials/components/project-workspace-frame";
import { CandidateBatchDetailPage } from "@/features/chapter-plans/candidate-batch-detail-page";

export default async function ProjectCandidateBatchDetailRoute({
  params,
}: {
  params: Promise<{ projectId: string; batchId: string }>;
}) {
  const { projectId, batchId } = await params;
  const project = await getProjectWorkspace(projectId)
    .then(({ project }) => project)
    .catch(() => null);

  return (
    <AppShell active="projects">
      {project ? (
        <ProjectWorkspaceFrame project={project} active="chapter-plans">
          <CandidateBatchDetailPage batchId={batchId} />
        </ProjectWorkspaceFrame>
      ) : (
        <main className="project-works-state error">
          <h1>暂时无法加载项目</h1>
        </main>
      )}
    </AppShell>
  );
}
