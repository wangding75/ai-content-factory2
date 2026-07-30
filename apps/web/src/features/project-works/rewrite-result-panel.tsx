"use client";
/* eslint-disable react-hooks/set-state-in-effect */

import Link from "next/link";
import { useCallback, useEffect, useRef, useState } from "react";
import { ApiError } from "@/lib/api";
import { getContentItem, getContentVersion, setCurrentContentVersion, type ContentItemDetail } from "@/features/content-items/content-item-http-api";
import { getContentRewriteResult, getContentRewriteSummary, type RewriteResult, type RewriteSummary } from "./rewrite-api";

type Props = { projectId: string; workId: string; runId: string; onRefresh?: () => void };

export function RewriteResultPanel({ projectId, workId, runId, onRefresh }: Props) {
  const [result, setResult] = useState<RewriteResult | null>(null);
  const [summary, setSummary] = useState<RewriteSummary | null>(null);
  const [item, setItem] = useState<ContentItemDetail | null>(null);
  const [source, setSource] = useState<string | null>(null);
  const [error, setError] = useState<ApiError | null>(null);
  const [confirming, setConfirming] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [casNotice, setCasNotice] = useState(false);
  const key = useRef<string | null>(null);
  const inFlight = useRef(false);

  const load = useCallback(async (signal?: AbortSignal) => {
    setError(null);
    try {
      const next = await getContentRewriteResult(runId, { signal });
      const [nextSummary, nextItem, sourceVersion] = await Promise.all([
        getContentRewriteSummary(next.reviewReportSnapshot.reviewReportId, { signal }),
        getContentItem(next.candidateVersion.content_item_id, { signal }),
        getContentVersion(next.sourceContentVersionSummary.id, { signal }),
      ]);
      if (!signal?.aborted) {
        setResult(next); setSummary(nextSummary); setItem(nextItem); setSource(sourceVersion.content_version.content);
      }
    } catch (cause) { if (!signal?.aborted) setError(cause as ApiError); }
  }, [runId]);
  useEffect(() => { const controller = new AbortController(); void load(controller.signal); return () => controller.abort(); }, [load]);

  const setCurrent = async () => {
    if (!result || !item || submitting || inFlight.current || result.candidateIsCurrent || !result.canSetCurrent) return;
    const idempotencyKey = key.current ?? crypto.randomUUID(); key.current = idempotencyKey;
    inFlight.current = true; setSubmitting(true); setError(null);
    try {
      await setCurrentContentVersion(item.content_item.id, {
        candidateVersionId: result.candidateVersion.id,
        expectedCurrentVersionId: item.current_version.id,
        expectedCurrentVersion: item.current_version.version,
      }, idempotencyKey);
      key.current = null; setConfirming(false); setCasNotice(false); await load(); onRefresh?.();
    } catch (cause) {
      const api = cause as ApiError;
      if (api.code === "content_version_conflict") {
        key.current = null; setConfirming(false); setCasNotice(true); await load(); onRefresh?.();
      } else {
        if (api.code !== "timeout" && api.code !== "network_error") key.current = null;
        setError(api);
      }
    } finally { inFlight.current = false; setSubmitting(false); }
  };

  if (!result && !error) return <ResultState title="正在加载重写候选版本" />;
  if (error && !result) return <ResultState title={error.status === 404 ? "重写记录或候选版本不存在" : "重写结果加载失败"} retry={() => void load()} />;
  if (!result || !item) return <ResultState title="正在恢复当前版本信息" />;
  const summaryConfirmsCurrent = summary?.candidateVersion?.id === result.candidateVersion.id && summary.candidateIsCurrent;
  const candidateIsCurrent = result.candidateIsCurrent || summaryConfirmsCurrent || result.candidateVersion.id === item.current_version.id;
  const candidateRelationValid = result.candidateVersion.content_item_id === item.content_item.id && result.candidateVersion.source_content_version_id === result.sourceContentVersionSummary.id;
  const canSet = result.canSetCurrent && !candidateIsCurrent && candidateRelationValid;
  const historyHref = `/projects/${projectId}/works/${workId}/rewrite?workflowRunId=${encodeURIComponent(runId)}&history=1`;
  return <main className="rewrite-page"><section className="rewrite-layout rewrite-result-layout"><article className="rewrite-source"><header><h1>重写候选版本</h1><p>{result.output.summary}</p></header>
    <dl><div><dt>固定来源版本</dt><dd>V{result.sourceContentVersionSummary.versionNo} · {result.sourceContentVersionSummary.title}</dd></div><div><dt>当前版本</dt><dd>V{item.current_version.version_no} · {item.current_version.title}</dd></div><div><dt>目标 Candidate</dt><dd>V{result.candidateVersion.version_no} · {result.candidateVersion.title}</dd></div></dl>
    {candidateIsCurrent ? <p role="status">该 Candidate 已是当前版本，无需再次提交。</p> : canSet ? <button type="button" className="primary" onClick={() => setConfirming(true)}>设为当前版本</button> : <p>候选版本尚未满足设为当前版本的条件。</p>}
    <Link href={historyHref}>查看重写历史</Link>{casNotice && <p role="alert">当前版本已经变化。已加载新的当前版本，请确认目标 Candidate 后重新提交。</p>}{error && <p role="alert">{safeSetCurrentError(error)}</p>}
    <h2>已处理问题</h2><Outcomes items={result.output.addressedIssues} /><h2>未解决问题</h2><Outcomes items={result.output.unresolvedIssues} />
    {result.output.warnings.length > 0 && <><h2>提示</h2><ul>{result.output.warnings.map((value) => <li key={value}>{value}</li>)}</ul></>}{result.output.metadata?.changeSummary && <p>{result.output.metadata.changeSummary}</p>}
  </article><article className="rewrite-result-content"><header><h2>正文版本对比</h2><span>固定来源 → Candidate</span></header><section><h3>来源正文</h3><p>{source ?? "正在读取来源正文。"}</p></section><section><h3>候选正文</h3><p>{result.candidateVersion.content}</p></section></article></section>
  {confirming && <div className="rewrite-dialog-layer"><section className="rewrite-confirm-dialog" role="dialog" aria-modal="true" aria-label="设为当前版本确认"><h2>设为当前版本</h2><p>确认将 Candidate v{result.candidateVersion.version_no} 设为当前版本。</p><dl><div><dt>当前版本</dt><dd>v{item.current_version.version_no}</dd></div><div><dt>目标 Candidate</dt><dd>v{result.candidateVersion.version_no}</dd></div><div><dt>固定来源版本</dt><dd>v{result.sourceContentVersionSummary.versionNo}</dd></div></dl><p>此操作只会切换 currentVersion，不会删除旧版本，也不会修改 ReviewReport 或 Issue。</p><p>若他人已切换当前版本，系统会拒绝本次提交并要求重新确认。</p><footer><button type="button" disabled={submitting} onClick={() => setConfirming(false)}>取消</button><button type="button" className="primary" disabled={submitting} onClick={() => void setCurrent()}>{submitting ? "正在设为当前版本…" : "确认设为当前版本"}</button></footer></section></div>}</main>;
}
function Outcomes({ items }: { items: Array<{ reviewIssueId: string; summary: string }> }) { return items.length ? <ul>{items.map((item) => <li key={item.reviewIssueId}>{item.summary}</li>)}</ul> : <p>无</p>; }
function ResultState({ title, retry }: { title: string; retry?: () => void }) { return <main className="rewrite-page"><section className="rewrite-state-card"><h1>{title}</h1>{retry && <footer><button type="button" onClick={retry}>重试</button></footer>}</section></main>; }
function safeSetCurrentError(error: ApiError) { return ({ idempotency_conflict: "本次提交状态不一致，请刷新后重新确认。", rewrite_candidate_not_found: "Candidate 不存在或来源关系异常。", rewrite_candidate_not_ready: "Candidate 尚未就绪，暂不能设为当前版本。", internal_error: "暂时无法设为当前版本，请稍后重试。", timeout: "请求结果暂未确认，请使用原确认操作重试。", network_error: "网络连接中断，请使用原确认操作重试。" }[error.code] ?? "暂时无法设为当前版本，请稍后重试。"); }
