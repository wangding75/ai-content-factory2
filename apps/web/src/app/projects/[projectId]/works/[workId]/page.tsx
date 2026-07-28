import { AppShell } from "@/components/ui/app-shell";
import { ContentEditorWorkspace } from "@/features/content-items/content-editor-workspace";

export default async function WorkEditorRoute({ params }: { params: Promise<{ projectId: string; workId: string }> }) {
  const { projectId, workId } = await params;
  return <AppShell active="projects"><ContentEditorWorkspace projectId={projectId} workId={workId} /></AppShell>;
}
