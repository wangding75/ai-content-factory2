import {AppShell} from "@/components/ui/app-shell";
import {MaterialsPage} from "@/features/planning-materials/materials/materials-page";
export default async function MaterialsRoute({params}:{params:Promise<{projectId:string}>}){const {projectId}=await params;return <AppShell active="projects"><MaterialsPage projectId={projectId}/></AppShell>}