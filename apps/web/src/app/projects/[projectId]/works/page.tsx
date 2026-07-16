import {AppShell} from "@/components/ui/app-shell";
import {ProjectWorksWorkspace} from "@/features/project-works/project-works-workspace";
export default async function ProjectWorksRoute({params,searchParams}:{params:Promise<{projectId:string}>;searchParams:Promise<{mock?:string}>}) { const {projectId}=await params; const {mock}=await searchParams; return <AppShell active="projects"><ProjectWorksWorkspace projectId={projectId} initialMockState={mock??null}/></AppShell>; }
