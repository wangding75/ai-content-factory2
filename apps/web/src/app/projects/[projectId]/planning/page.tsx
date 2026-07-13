import { AppShell } from "@/components/ui/app-shell";
import { PlanningPage } from "@/features/planning-materials/planning/planning-page";

export default async function ProjectPlanningRoute({ params }: { params: Promise<{ projectId: string }> }) {
  const { projectId } = await params;
  return <AppShell active="projects"><PlanningPage projectId={projectId} /></AppShell>;
}
