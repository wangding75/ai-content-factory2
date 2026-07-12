import { AppShell } from "@/components/ui/app-shell";
import { parsePlanningMockScenario } from "@/features/planning-materials/api/planning-api";
import { PlanningPage } from "@/features/planning-materials/planning/planning-page";

export default async function ProjectPlanningRoute({ params, searchParams }: { params: Promise<{ projectId: string }>; searchParams: Promise<{ mockScenario?: string }> }) {
  const { projectId } = await params;
  const { mockScenario } = await searchParams;
  return <AppShell active="projects"><PlanningPage projectId={projectId} scenario={parsePlanningMockScenario(mockScenario)} /></AppShell>;
}
