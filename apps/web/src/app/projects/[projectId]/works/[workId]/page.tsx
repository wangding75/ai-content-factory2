import { AppShell } from "@/components/ui/app-shell";
import { ProjectWorkEditorShell } from "@/features/content-review/project-content-review-shell";

export default async function WorkEditorRoute({ params }: { params: Promise<{ projectId: string; workId: string }> }) {
  const { projectId, workId } = await params;
  return <AppShell active="projects"><ProjectWorkEditorShell projectId={projectId} workId={workId} /></AppShell>;
}
