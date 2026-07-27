import { AppShell } from "@/components/ui/app-shell";
import { ProjectWorkspaceLoader } from "@/features/planning-materials/components/project-workspace-loader";

export default async function StorylinesRoute({ params }: { params: Promise<{ projectId: string }> }) {
  const { projectId } = await params;
  return <AppShell active="projects"><ProjectWorkspaceLoader projectId={projectId} page="storylines" /></AppShell>;
}
