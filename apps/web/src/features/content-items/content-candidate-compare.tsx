"use client";
import { useEffect, useState } from "react";
import { ApiError } from "@/lib/api";
import { clearOperation, getOrCreateOperation } from "@/features/chapter-plans/use-idempotency";
import { getContentVersion, setCurrentContentVersion, type ContentGenerationSummary, type ContentVersion } from "./content-item-http-api";
import { contentGenerationCopy as copy } from "./content-generation-locale";

export function ContentCandidateCompare({ summary, onClose, onApplied, onRefresh }: { summary: ContentGenerationSummary; onClose: () => void; onApplied: () => Promise<void>; onRefresh: () => Promise<void> }) {
  const candidate = summary.latestCandidateVersion;
  const [current, setCurrent] = useState<ContentVersion | null>(null);
  const [selected, setSelected] = useState<"current" | "candidate">("candidate");
  const [confirming, setConfirming] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  useEffect(() => { const controller = new AbortController(); void getContentVersion(summary.currentVersionId, { signal: controller.signal }).then((x) => { if (!controller.signal.aborted) setCurrent(x.content_version); }).catch(() => setError(copy.currentReadFailed)); return () => controller.abort(); }, [summary.currentVersionId]);
  if (!candidate) return null;
  const showing = selected === "candidate" ? candidate : current;
  const apply = async () => {
    if (!summary.candidateCanBecomeCurrent || submitting) return;
    setSubmitting(true); setError(null);
    const payload = { contentItemId: summary.contentItemId, candidateVersionId: candidate.id, expectedCurrentVersionId: summary.currentVersionId, expectedCurrentVersion: summary.currentVersion.version };
    const scope = `content-generation-set-current:${summary.contentItemId}`;
    try {
      const idempotencyKey = await getOrCreateOperation(scope, payload);
      await setCurrentContentVersion(summary.contentItemId, payload, idempotencyKey);
      clearOperation(scope);
      await onApplied();
    } catch (cause) {
      const api = cause as ApiError;
      if (api.code === "candidate_source_stale" || api.code === "current_version_changed" || api.status === 409) { clearOperation(scope); setError(copy.stale); await onRefresh(); }
      else setError(copy.switchFailed);
    } finally { setSubmitting(false); }
  };
  return <section className="content-candidate-compare" aria-label={copy.candidateCompareAria}><header><div><strong>{copy.candidateViewing(candidate.version_no)}</strong><span>{copy.sourceRun(summary.latestRun?.runNumber ?? "—", summary.currentVersion.version_no)}</span></div><div><button onClick={() => setSelected("current")}>{copy.viewCurrent(summary.currentVersion.version_no)}</button><button onClick={() => setSelected("candidate")}>{copy.viewCandidateVersion(candidate.version_no)}</button><button onClick={onClose}>{copy.closeCandidate}</button><button className="content-candidate-apply" onClick={() => setConfirming(true)} disabled={!summary.candidateCanBecomeCurrent || submitting}>{submitting ? copy.settingCurrent : copy.setCurrent}</button></div></header>{confirming && <section className="content-candidate-confirm" role="alert"><p>{copy.confirmSetCurrent}</p><button onClick={() => void apply()} disabled={submitting}>{submitting ? copy.settingCurrent : copy.confirmSetCurrentAction}</button><button onClick={() => setConfirming(false)} disabled={submitting}>{copy.cancelSetCurrent}</button></section>}{!summary.candidateCanBecomeCurrent && <p className="content-inline-error">{copy.stale}</p>}{error && <p className="content-inline-error" role="alert">{error}</p>}<article><h2>{selected === "candidate" ? copy.candidateTitle(candidate.version_no) : copy.currentTitle(current?.version_no)}</h2><p>{showing?.content ?? copy.currentReading}</p></article></section>;
}
