"use client";
/* eslint-disable react-hooks/set-state-in-effect */

import Link from "next/link";
import { useCallback, useEffect, useRef, useState } from "react";
import { ApiError } from "@/lib/api";
import { getContentItem, getContentVersion, setCurrentContentVersion, type ContentItemDetail } from "@/features/content-items/content-item-http-api";
import { formatWorkflowRunTime } from "@/features/workflow-runs/workflow-run-api";
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
  return <RewriteResultView projectId={projectId} workId={workId} result={result} item={item} source={source} candidateIsCurrent={candidateIsCurrent} canSet={canSet} historyHref={historyHref} confirming={confirming} submitting={submitting} casNotice={casNotice} error={error} onConfirm={() => setConfirming(true)} onDismiss={() => setConfirming(false)} onSetCurrent={() => void setCurrent()} />;
}
function RewriteResultView({ projectId, workId, result, item, source, candidateIsCurrent, canSet, historyHref, confirming, submitting, casNotice, error, onConfirm, onDismiss, onSetCurrent }: { projectId: string; workId: string; result: RewriteResult; item: ContentItemDetail; source: string | null; candidateIsCurrent: boolean; canSet: boolean; historyHref: string; confirming: boolean; submitting: boolean; casNotice: boolean; error: ApiError | null; onConfirm: () => void; onDismiss: () => void; onSetCurrent: () => void }) {
  const addressed = new Set(result.output.addressedIssues.map((issue) => issue.reviewIssueId));
  const unresolved = new Set(result.output.unresolvedIssues.map((issue) => issue.reviewIssueId));
  const protectionStatement = "本操作不会删除旧版本，也不会修改 ReviewReport 或 Issue";
  return (
    <main className="rewrite-page rewrite-result-page" data-protection-statement={protectionStatement} data-operation="currentVersion">
      <Link className="rewrite-result-back" href={`/projects/${projectId}/works/${workId}/review?reportId=${encodeURIComponent(result.reviewReportSnapshot.reviewReportId)}`}>← 返回审核结果</Link>
      <header className="rewrite-result-hero">
        <div><div className="rewrite-result-title"><h1>正文重写结果</h1><span>✓ 重写成功</span></div><p>{result.output.summary || `已根据 ${result.selectedIssueSummary.total} 个审核问题创建候选正文。`}</p></div>
        <Link href={`/workflow-runs/${encodeURIComponent(result.workflowRun.id)}`}>查看执行详情 <span aria-hidden="true">→</span></Link>
      </header>
      <section className="rewrite-result-meta" aria-label="重写结果信息">
        <div><dt>Run</dt><dd><code>{result.workflowRun.runNumber}</code></dd></div>
        <div><dt>工作流</dt><dd>{result.workflowRun.workflowName ?? "正文重写"} · v{result.workflowRun.workflowConfigurationVersion ?? "—"}</dd></div>
        <div><dt>来源版本</dt><dd>V{result.sourceContentVersionSummary.versionNo}</dd></div>
        <div><dt>候选版本</dt><dd>V{result.candidateVersion.version_no}</dd></div>
        <div><dt>完成时间</dt><dd>{formatWorkflowRunTime(result.workflowRun.finishedAt ?? result.workflowRun.updatedAt)}</dd></div>
      </section>
      <section className="rewrite-result-layout">
        <aside className="rewrite-result-sidebar">
          <section className="rewrite-result-version-card">
            <h2>版本关系</h2>
            <div className="rewrite-result-version-flow"><div><strong>V{item.current_version.version_no}</strong><span>当前版本</span></div><span aria-hidden="true">→</span><div className="candidate"><strong>V{result.candidateVersion.version_no}</strong><span>候选版本</span></div></div>
            <p>{candidateIsCurrent ? `V${result.candidateVersion.version_no} 已是当前版本` : `V${result.candidateVersion.version_no} 尚未设为当前版本`}</p>
            {candidateIsCurrent ? <p role="status" className="rewrite-result-current-note">候选已生效，无需再次提交。</p> : canSet ? <button type="button" className="primary" onClick={onConfirm}>设为当前版本</button> : <p>候选版本尚未满足设为当前版本的条件。</p>}
            <Link href={historyHref}>查看重写历史</Link>
          </section>
          <section className="rewrite-result-metrics"><h2>生成摘要</h2><div><article><strong>{result.output.addressedIssues.length}</strong><span>已处理问题</span></article><article><strong>{result.output.unresolvedIssues.length}</strong><span>未解决问题</span></article><article><strong>{result.selectedIssueSummary.total}</strong><span>处理问题</span></article></div></section>
          <section className="rewrite-result-issues"><header><h2>审核问题处理结果</h2><span>{result.selectedIssueSummary.total} 项</span></header>{result.selectedIssueSummary.items.map((issue) => <article key={issue.reviewIssueId}><span className={`rewrite-result-issue-state ${addressed.has(issue.reviewIssueId) ? "is-addressed" : unresolved.has(issue.reviewIssueId) ? "is-unresolved" : "is-pending"}`} aria-hidden="true">{addressed.has(issue.reviewIssueId) ? "✓" : unresolved.has(issue.reviewIssueId) ? "!" : "·"}</span><div><strong>{issue.title}</strong><p>{addressed.has(issue.reviewIssueId) ? "已处理" : unresolved.has(issue.reviewIssueId) ? "未解决" : "未标记结果"} · {issue.severity}</p></div></article>)}</section>
        </aside>
        <article className="rewrite-result-content rewrite-result-candidate-card">
          <header><div><h2>{result.candidateVersion.title}</h2><span>V{result.candidateVersion.version_no} 候选版本</span></div><Link href={`/projects/${projectId}/works/${workId}`}>打开编辑器 ↗</Link></header>
          <div className="rewrite-result-notice" role="status"><span aria-hidden="true">ⓘ</span><p>该候选版本尚未影响当前作品，来源版本 V{result.sourceContentVersionSummary.versionNo} 保持不变。</p></div>
          <section className="rewrite-result-text"><h3>候选正文</h3><pre>{result.candidateVersion.content}</pre></section>
          <details className="rewrite-result-source"><summary>查看固定来源正文</summary><pre>{source ?? "正在读取来源正文。"}</pre></details>
        </article>
      </section>
      {casNotice && <p role="alert" className="rewrite-result-alert">当前版本已经变化。已加载新的当前版本，请确认目标 Candidate 后重新提交。</p>}
      {error && <p role="alert" className="rewrite-result-alert">{safeSetCurrentError(error)}</p>}
      {confirming && <div className="rewrite-dialog-layer"><section className="rewrite-confirm-dialog rewrite-set-current-dialog" role="dialog" aria-modal="true" aria-label="设为当前版本确认"><header><h2>设为当前版本</h2><button type="button" aria-label="关闭" disabled={submitting} onClick={onDismiss}>×</button></header><p className="rewrite-set-current-question">确认将候选版本 v{result.candidateVersion.version_no} 设为当前版本？</p><div className="rewrite-set-current-flow"><div><span>当前版本</span><strong>v{item.current_version.version_no}</strong></div><span aria-hidden="true">→</span><div><span>新当前版本</span><strong>v{result.candidateVersion.version_no}</strong></div></div><ul className="rewrite-set-current-impact"><li>当前版本将保留在版本历史中</li><li>审核报告仍绑定来源版本 v{result.sourceContentVersionSummary.versionNo}</li><li>本操作不会删除或覆盖其他历史版本</li><li>设为当前版本后不会自动触发重新审核</li></ul><p className="rewrite-set-current-hint">之后仍可从版本历史切换回旧版本。</p><footer><button type="button" disabled={submitting} onClick={onDismiss}>取消</button><button type="button" className="primary" disabled={submitting} onClick={onSetCurrent}>{submitting ? "正在设为当前版本…" : "设为当前版本"}</button></footer></section></div>}
    </main>
  );
}

function ResultState({ title, retry }: { title: string; retry?: () => void }) { return <main className="rewrite-page"><section className="rewrite-state-card"><h1>{title}</h1>{retry && <footer><button type="button" onClick={retry}>重试</button></footer>}</section></main>; }
function safeSetCurrentError(error: ApiError) { return ({ idempotency_conflict: "本次提交状态不一致，请刷新后重新确认。", rewrite_candidate_not_found: "Candidate 不存在或来源关系异常。", rewrite_candidate_not_ready: "Candidate 尚未就绪，暂不能设为当前版本。", internal_error: "暂时无法设为当前版本，请稍后重试。", timeout: "请求结果暂未确认，请使用原确认操作重试。", network_error: "网络连接中断，请使用原确认操作重试。" }[error.code] ?? "暂时无法设为当前版本，请稍后重试。"); }
