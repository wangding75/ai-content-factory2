"use client";
import { useEffect, useRef, useState } from "react";
import { ApiError } from "@/lib/api";
import { getContentVersion, setCurrentContentVersion, type ContentGenerationSummary, type ContentVersion } from "./content-item-http-api";

export function ContentCandidateCompare({ summary, onClose, onApplied, onRefresh }: { summary: ContentGenerationSummary; onClose: () => void; onApplied: () => Promise<void>; onRefresh: () => Promise<void> }) {
  const candidate = summary.latestCandidateVersion;
  const [current, setCurrent] = useState<ContentVersion | null>(null);
  const [selected, setSelected] = useState<"current" | "candidate">("candidate");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const key = useRef<string | null>(null);
  useEffect(() => { const controller = new AbortController(); void getContentVersion(summary.currentVersionId, { signal: controller.signal }).then((x) => { if (!controller.signal.aborted) setCurrent(x.content_version); }).catch(() => setError("无法读取当前版本，请稍后重试。")); return () => controller.abort(); }, [summary.currentVersionId]);
  if (!candidate) return null;
  const showing = selected === "candidate" ? candidate : current;
  const apply = async () => {
    if (!summary.candidateCanBecomeCurrent || submitting || !window.confirm("确认将候选版本设为当前正文吗？当前版本会保留在版本历史中。")) return;
    setSubmitting(true); setError(null);
    try {
      const idempotencyKey = key.current ?? crypto.randomUUID(); key.current = idempotencyKey;
      await setCurrentContentVersion(summary.contentItemId, { candidateVersionId: candidate.id, expectedCurrentVersionId: summary.currentVersionId, expectedCurrentVersion: summary.currentVersion.version }, idempotencyKey);
      await onApplied();
    } catch (cause) {
      const api = cause as ApiError;
      if (api.code === "candidate_source_stale" || api.code === "current_version_changed" || api.status === 409) { setError("当前正文已变化，候选仍可查看和比较，但不能设为当前版本。"); await onRefresh(); }
      else setError("切换版本失败，当前正文未被覆盖，请稍后重试。");
    } finally { setSubmitting(false); }
  };
  return <section className="content-candidate-compare" aria-label="候选版本比较"><header><div><strong>当前查看：v{candidate.version_no}（候选）</strong><span>来源 Run：{summary.latestRun?.runNumber ?? "—"}；基线版本：v{summary.currentVersion.version_no}</span></div><div><button onClick={() => setSelected("current")}>查看当前 v{summary.currentVersion.version_no}</button><button onClick={() => setSelected("candidate")}>查看候选 v{candidate.version_no}</button><button onClick={onClose}>关闭候选</button><button className="content-candidate-apply" onClick={() => void apply()} disabled={!summary.candidateCanBecomeCurrent || submitting}>{submitting ? "正在设为当前…" : "设为当前版本"}</button></div></header>{!summary.candidateCanBecomeCurrent && <p className="content-inline-error">当前正文已变化；候选仍可比较，但不能强制覆盖。</p>}{error && <p className="content-inline-error" role="alert">{error}</p>}<article><h2>{selected === "candidate" ? `候选版本 v${candidate.version_no}` : `当前版本 v${current?.version_no ?? "—"}`}</h2><p>{showing?.content ?? "正在读取当前版本…"}</p></article></section>;
}
