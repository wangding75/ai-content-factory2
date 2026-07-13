import {AppShell} from "@/components/ui/app-shell";
import {MaterialsPage} from "@/features/planning-materials/materials/materials-page";
import {CreateMaterialPage} from "@/features/planning-materials/materials/create-material-page";
export default async function Page({params}:{params:Promise<{projectId:string}>}){const {projectId}=await params;return <AppShell active="projects"><MaterialsPage projectId={projectId}/><CreateMaterialPage projectId={projectId}/></AppShell>}