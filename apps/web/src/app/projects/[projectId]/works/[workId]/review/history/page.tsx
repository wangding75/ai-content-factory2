import { AppShell } from "@/components/ui/app-shell";
import { ProjectReviewHistoryShell } from "@/features/content-review/project-content-review-shell";

export default async function WorkReviewHistoryRoute({
  params,
}: {
  params: Promise<{ projectId: string; workId: string }>;
}) {
  const { projectId, workId } = await params;
  return (
    <AppShell active="projects">
      <ProjectReviewHistoryShell projectId={projectId} workId={workId} />
    </AppShell>
  );
}
