import { AppShell } from "@/components/ui/app-shell";
import { RewriteWorkspace } from "@/features/project-works/rewrite-workspace";

export default async function RewriteRoute({
  params,
  searchParams,
}: {
  params: Promise<{ projectId: string; workId: string }>;
  searchParams: Promise<{
    reportId?: string;
    issueIds?: string;
    workflowRunId?: string;
    history?: string;
  }>;
}) {
  const { projectId, workId } = await params;
  const search = await searchParams;
  const issueIds = search.issueIds?.split(",").filter(Boolean);
  return (
    <AppShell active="projects">
      <RewriteWorkspace
        projectId={projectId}
        workId={workId}
        reportId={search.reportId}
        issueIds={issueIds}
        workflowRunId={search.workflowRunId}
        showHistory={search.history === "1"}
      />
    </AppShell>
  );
}
