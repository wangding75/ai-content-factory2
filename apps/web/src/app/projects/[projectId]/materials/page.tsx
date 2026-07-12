import { AppShell } from "@/components/ui/app-shell";
import { parsePlanningMockScenario } from "@/features/planning-materials/api/planning-api";
import { MaterialsPage } from "@/features/planning-materials/materials/materials-page";
export default async function MaterialsRoute({params,searchParams}:{params:Promise<{projectId:string}>;searchParams:Promise<{mockScenario?:string}>}){const {projectId}=await params;const {mockScenario}=await searchParams;return <AppShell active="projects"><MaterialsPage projectId={projectId} scenario={parsePlanningMockScenario(mockScenario)}/></AppShell>}
