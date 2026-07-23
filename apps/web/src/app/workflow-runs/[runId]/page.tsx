import { AppShell } from "@/components/ui/app-shell";
import { WorkflowRunDetailPage } from "@/features/workflow-runs/workflow-run-detail-page";
export default async function Page({ params }: { params: Promise<{ runId: string }> }) { const { runId } = await params; return <AppShell active="workflows"><WorkflowRunDetailPage runId={runId} /></AppShell>; }
