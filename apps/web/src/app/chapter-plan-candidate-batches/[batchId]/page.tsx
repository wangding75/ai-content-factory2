import { AppShell } from "@/components/ui/app-shell";
import { CandidateBatchDetailPage } from "@/features/chapter-plans/candidate-batch-detail-page";

export default async function CandidateBatchDetailRoute({
  params,
}: {
  params: Promise<{ batchId: string }>;
}) {
  const { batchId } = await params;

  return (
    <AppShell active="projects">
      <div style={{ padding: "24px 32px" }}>
        <CandidateBatchDetailPage batchId={batchId} />
      </div>
    </AppShell>
  );
}
