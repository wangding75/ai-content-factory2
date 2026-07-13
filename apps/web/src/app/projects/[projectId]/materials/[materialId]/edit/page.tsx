import {AppShell} from "@/components/ui/app-shell";
import {MaterialsPage} from "@/features/planning-materials/materials/materials-page";
import {MaterialDetailDrawer} from "@/features/planning-materials/materials/material-detail-drawer";
import {EditMaterialPage} from "@/features/planning-materials/materials/edit-material-page";
export default async function Page({params}:{params:Promise<{projectId:string;materialId:string}>}){const {projectId,materialId}=await params;return <AppShell active="projects"><MaterialsPage projectId={projectId}/><MaterialDetailDrawer projectId={projectId} materialId={materialId}/><EditMaterialPage projectId={projectId} materialId={materialId}/></AppShell>}