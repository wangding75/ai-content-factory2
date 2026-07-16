import {AppShell} from "@/components/ui/app-shell";
import {ProjectWorksWorkspace} from "@/features/project-works/project-works-workspace";
export default async function ProjectWorksRoute({params}:{params:Promise<{projectId:string}>}) { const {projectId}=await params; return <AppShell active="projects"><ProjectWorksWorkspace projectId={projectId}/></AppShell>; }
