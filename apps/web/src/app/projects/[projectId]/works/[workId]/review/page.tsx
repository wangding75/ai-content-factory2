import { AppShell } from "@/components/ui/app-shell";
import { ProjectReviewShell } from "@/features/content-review/project-content-review-shell";

export default async function WorkReviewRoute({
  params,
  searchParams,
}: {
  params: Promise<{ projectId: string; workId: string }>;
  searchParams: Promise<{
    reportId?: string;
    issueId?: string;
    view?: string;
  }>;
}) {
  const { projectId, workId } = await params;
  const query = await searchParams;
  return (
    <AppShell active="projects">
      <ProjectReviewShell
        projectId={projectId}
        workId={workId}
        reportId={query.reportId}
        issueId={query.view === "source" ? query.issueId : undefined}
      />
    </AppShell>
  );
}
