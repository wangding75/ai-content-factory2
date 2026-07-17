import { AppShell } from "@/components/ui/app-shell";
import { ProjectWorkspaceFrame } from "@/features/planning-materials/components/project-workspace-frame";
import { StorylinesWorkspace } from "@/features/storylines-workspace";
import { getProjectWorkspace } from "@/lib/api";
export default async function StorylinesRoute({params}:{params:Promise<{projectId:string}>}){const {projectId}=await params;const project=await getProjectWorkspace(projectId).then(({project})=>project).catch(()=>null);return <AppShell active="projects">{project?<ProjectWorkspaceFrame project={project} active="storylines"><StorylinesWorkspace projectId={projectId}/></ProjectWorkspaceFrame>:<main className="project-works-state error"><h1>暂时无法加载项目</h1></main>}</AppShell>}
