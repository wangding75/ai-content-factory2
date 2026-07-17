import {AppShell} from "@/components/ui/app-shell";
import {ProjectWorksWorkspace} from "@/features/project-works/project-works-workspace";
export default async function ProjectWorksRoute({params,searchParams}:{params:Promise<{projectId:string}>;searchParams:Promise<{view?:string}>}) { const {projectId}=await params; const {view}=await searchParams; return <AppShell active="projects"><ProjectWorksWorkspace projectId={projectId} view={view==="review"?"review":"works"}/></AppShell>; }
