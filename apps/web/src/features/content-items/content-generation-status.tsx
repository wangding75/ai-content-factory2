"use client";
import Link from "next/link";
import { useEffect, useState } from "react";
import { clearOperation, getOrCreateOperation } from "@/features/chapter-plans/use-idempotency";
import { getWorkflowRunEvents, retryContentGenerationResultConsumption, retryWorkflowRun, type ContentGenerationSummary, type GenerationEvent } from "./content-item-http-api";
import { contentGenerationCopy as copy } from "./content-generation-locale";

const failed = new Set(["runtime_failed", "output_validation_failed", "result_consumption_failed"]);
const safe = (message: string | null | undefined) => message && message.length <= 300 && !/(stack|sql|postgres|https?:\/\/|\\\\)/i.test(message) ? message : copy.safeFailure;
const eventLabel = (value: string) => copy.events[value] ?? copy.eventFallback;

export function ContentGenerationStatus({ projectId, summary, onRefresh, onCandidate }: { projectId: string; summary: ContentGenerationSummary; onRefresh: () => Promise<void>; onCandidate: () => void }) {
  const [fetchedEvents, setFetchedEvents] = useState<GenerationEvent[] | null>(null);
  const [showEvents, setShowEvents] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [retryError, setRetryError] = useState<{ identity: string; message: string } | null>(null);
  const run = summary.activeRun ?? summary.latestRun;
  useEffect(() => {
    if (!run || !["queued", "running"].includes(summary.state)) return;
    const controller = new AbortController();
    void getWorkflowRunEvents(run.id, { signal: controller.signal }).then((next) => { if (!controller.signal.aborted) setFetchedEvents(next.items); }).catch(() => undefined);
    return () => controller.abort();
  }, [run, summary.state]);
  if (!["queued", "running", "candidate_ready", "not_configured", ...failed].includes(summary.state)) return null;
  const title = summary.state === "queued" ? copy.titles.queued : summary.state === "running" ? copy.titles.running : summary.state === "candidate_ready" ? copy.titles.candidate_ready : summary.state === "not_configured" ? copy.titles.not_configured : copy.titles.failed;
  const retryType = summary.state === "result_consumption_failed" ? "result-consumption-retry" : "runtime-retry";
  const retryPayload = run ? { runId: run.id, state: summary.state, runVersion: run.version, retryType } : null;
  const retryIdentity = retryPayload ? JSON.stringify(retryPayload) : "";
  const retry = async () => {
    if (!run || !retryPayload || submitting) return;
    const scope = `content-generation-retry:${run.id}`;
    setRetryError(null); setSubmitting(true);
    try {
      const key = await getOrCreateOperation(scope, retryPayload);
      if (summary.state === "result_consumption_failed") await retryContentGenerationResultConsumption(run.id, run.version, key);
      else await retryWorkflowRun(run.id, run.version, key, "original_configuration");
      clearOperation(scope);
      await onRefresh();
    } catch { setRetryError({ identity: retryIdentity, message: copy.retryFailure }); }
    finally { setSubmitting(false); }
  };
  return <section className={`content-generation-status content-generation-status-${summary.state}`} aria-live="polite">
    <div><strong>{title}</strong>{run && <span>Run ID: {run.runNumber}</span>}
      {summary.state === "queued" && <span>{copy.queuedDetail}</span>}{summary.state === "running" && <span>{copy.runningDetail}</span>}{summary.state === "candidate_ready" && <span>{copy.candidateDetail(summary.latestCandidateVersion?.version_no)}</span>}{failed.has(summary.state) && <span>{safe(summary.latestError?.message ?? run?.errorMessage)}</span>}{summary.state === "not_configured" && <span>{copy.notConfiguredDetail}</span>}
    </div>
    <div className="content-generation-status-actions">
      {summary.state === "candidate_ready" && <button onClick={onCandidate}>{copy.viewCandidate}</button>}{run && <button onClick={() => setShowEvents((x) => !x)}>{showEvents ? copy.hideDetails : copy.viewDetails}</button>}{summary.state === "running" && run && <Link href={`/workflow-runs/${run.id}`}>{copy.workflowCenter}</Link>}{summary.state === "not_configured" && <Link href={`/projects/${projectId}/settings?tab=workflow-bindings`}>{copy.configureWorkflow}</Link>}
      {summary.state === "runtime_failed" && <button onClick={() => void retry()} disabled={submitting}>{submitting ? copy.retrying : copy.retryRuntime}</button>}{summary.state === "output_validation_failed" && <button onClick={() => void retry()} disabled={submitting}>{submitting ? copy.retrying : copy.retryRuntime}</button>}{summary.state === "result_consumption_failed" && <button onClick={() => void retry()} disabled={submitting}>{submitting ? copy.retrying : copy.retryConsumption}</button>}
    </div>
    {retryError?.identity === retryIdentity && <p className="content-inline-error" role="alert">{retryError.message}</p>}
    {showEvents && <ol className="content-generation-events">{(fetchedEvents ?? summary.latestEvents).length ? (fetchedEvents ?? summary.latestEvents).map((event) => <li key={event.id}><b>{eventLabel(event.eventType)}</b><span>{new Date(event.createdAt).toLocaleString("zh-CN")}</span></li>) : <li>{copy.eventEmpty}</li>}</ol>}
  </section>;
}
