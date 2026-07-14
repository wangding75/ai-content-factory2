import { AppShell } from "@/components/ui/app-shell";
import { ProjectWorkspaceNav } from "@/features/planning-materials/components/project-workspace-nav";
import { StorylinesWorkspace } from "@/features/storylines-workspace";
export default async function StorylinesRoute({params}:{params:Promise<{projectId:string}>}){const {projectId}=await params;return <AppShell active="projects"><ProjectWorkspaceNav projectId={projectId} active="storylines"/><StorylinesWorkspace projectId={projectId}/></AppShell>}