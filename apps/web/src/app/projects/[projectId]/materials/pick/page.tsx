import {AppShell} from "@/components/ui/app-shell";
import {MaterialsPage} from "@/features/planning-materials/materials/materials-page";
import {PickMaterialPage} from "@/features/planning-materials/materials/pick-material-page";
export default async function Page({params}:{params:Promise<{projectId:string}>}){const {projectId}=await params;return <AppShell active="projects"><MaterialsPage projectId={projectId}/><PickMaterialPage projectId={projectId}/></AppShell>}