import { AppShell } from "@/components/ui/app-shell";
import { getProjectWorkspace } from "@/lib/api";
import { ProjectWorkspaceFrame } from "@/features/planning-materials/components/project-workspace-frame";
import { CandidateBatchListPage } from "@/features/chapter-plans/candidate-batch-list-page";

export default async function ProjectCandidateBatchesRoute({
  params,
}: {
  params: Promise<{ projectId: string }>;
}) {
  const { projectId } = await params;
  const project = await getProjectWorkspace(projectId)
    .then(({ project }) => project)
    .catch(() => null);

  return (
    <AppShell active="projects">
      {project ? (
        <ProjectWorkspaceFrame project={project} active="chapter-plans">
          <CandidateBatchListPage projectId={projectId} />
        </ProjectWorkspaceFrame>
      ) : (
        <main className="project-works-state error">
          <h1>暂时无法加载项目</h1>
        </main>
      )}
    </AppShell>
  );
}
