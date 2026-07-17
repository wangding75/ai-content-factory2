import { AppShell } from "@/components/ui/app-shell";
import { getProjectWorkspace } from "@/lib/api";
import { ProjectWorksWorkspace } from "@/features/project-works/project-works-workspace";
export default async function ProjectWorksRoute({
  params,
  searchParams,
}: {
  params: Promise<{ projectId: string }>;
  searchParams: Promise<{ view?: string }>;
}) {
  const { projectId } = await params;
  const { view } = await searchParams;
  const project = await getProjectWorkspace(projectId).then(({ project }) => project).catch(() => null);
  return (
    <AppShell active="projects">
      {project ? <ProjectWorksWorkspace
        projectId={projectId}
        project={project}
        view={view === "review" ? "review" : "works"}
      /> : <main className="project-works-state error"><h1>暂时无法加载项目</h1></main>}
    </AppShell>
  );
}
