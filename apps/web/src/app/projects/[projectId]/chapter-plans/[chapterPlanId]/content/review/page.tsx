import { AppShell } from "@/components/ui/app-shell";
import { ContentReviewWorkspace } from "@/features/content-items/content-review-workspace";

export default async function ContentReviewRoute({ params }: { params: Promise<{ projectId: string; chapterPlanId: string }> }) {
  const { projectId, chapterPlanId } = await params;
  return <AppShell active="projects"><ContentReviewWorkspace projectId={projectId} chapterPlanId={chapterPlanId} /></AppShell>;
}
