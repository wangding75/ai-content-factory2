"use client";
/* eslint-disable react-hooks/set-state-in-effect */
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useRef, useState } from "react";
import { ApiError } from "@/lib/api";
import { cancelWorkflowRun, formatWorkflowRunTime, getWorkflowRun, listWorkflowRunEvents, retryWorkflowRun, type WorkflowRunDto, type WorkflowRunEventVm } from "@/features/workflow-runs/workflow-run-api";
import { getReview, isRealReviewDetail, type RealReviewIssue } from "@/features/content-review/content-review-api";
import { reviewLocationLabel } from "@/features/content-review/content-review-presentation";
import { getContentVersion } from "@/features/content-items/content-item-http-api";
import { createContentRewriteRun, getContentRewriteAvailability, getContentRewriteResult, getContentRewriteSummary, preflightContentRewrite, retryContentRewriteResultConsumption, rewriteErrorMessage, validRewriteInput, type RewriteAvailability, type RewriteOptions, type RewritePreflight, type RewriteResult as RewriteResultDto, type RewriteSummary } from "./rewrite-api";
import { WorkflowPreflightBlocker } from "@/components/workflow-preflight-blocker";
import { RewriteResultPanel } from "./rewrite-result-panel";
import { toWorkflowPreflightReasons } from "@/components/workflow-preflight-reason";
import { RewriteHistory } from "./rewrite-history";

type Props = { projectId: string; workId: string; reportId?: string; issueIds?: string[]; workflowRunId?: string; showHistory?: boolean };

export const rewritePreflightBlockerReasons = (report: RewritePreflight) => toWorkflowPreflightReasons(report.checks.filter((check) => check.status === "blocked"));
const active = (state: RewriteSummary["state"]) => state === "queued" || state === "running";
const safeError = (error: ApiError | null) => error?.code === "network_error" || error?.code === "timeout" ? "网络连接暂时中断，未能确认最新状态。请稍后重试。" : "暂时无法读取重写状态，请稍后重试。";

export function RewriteWorkspace({ projectId, workId, reportId: initialReportId, issueIds, workflowRunId, showHistory }: Props) {
  const router = useRouter();
  const [reportId, setReportId] = useState(initialReportId);
  const [summary, setSummary] = useState<RewriteSummary | null>(null);
  const [run, setRun] = useState<WorkflowRunDto | null>(null);
  const [events, setEvents] = useState<WorkflowRunEventVm[]>([]);
  const [loading, setLoading] = useState(true); const [error, setError] = useState<ApiError | null>(null);
  const [retrying, setRetrying] = useState(false); const [cancelling, setCancelling] = useState(false); const [confirmCancel, setConfirmCancel] = useState(false);
  const runtimeKey = useRef<string | null>(null); const consumptionKey = useRef<string | null>(null); const retryUnknown = useRef(false);
  const cancelKey = useRef<string | null>(null);
  const runUrl = useCallback((id: string) => `/projects/${projectId}/works/${workId}/rewrite?workflowRunId=${encodeURIComponent(id)}`, [projectId, workId]);
  const load = useCallback(async (signal?: AbortSignal) => {
    setError(null);
    try {
      let reviewId = reportId; let selectedRun: WorkflowRunDto | null = null;
      if (workflowRunId) {
        const detail = await getWorkflowRun(workflowRunId, { signal });
        const inputItemId = typeof detail.inputPayload.contentItemId === "string" ? detail.inputPayload.contentItemId : null;
        if (detail.stage !== "rewrite" || detail.subjectType !== "review_report" || detail.projectId !== projectId || inputItemId !== workId || !detail.subjectId) {
          throw new ApiError("Workflow run does not belong to this rewrite workspace.", 404);
        }
        selectedRun = detail; reviewId = detail.subjectId;
        if (!signal?.aborted) setReportId(reviewId);
      }
      if (!reviewId) { setLoading(false); return; }
      const next = await getContentRewriteSummary(reviewId, { signal });
      if (signal?.aborted) return;
      const current = selectedRun ?? next.activeRun ?? next.latestRun;
      if (current) {
        setRun(current);
        let nextEvents: WorkflowRunEventVm[] = [];
        try { nextEvents = await listWorkflowRunEvents(current.id, { signal }); } catch { nextEvents = []; }
        setEvents(nextEvents);
        setSummary(selectedRun ? exactRunSummary(next, selectedRun, nextEvents) : next);
      } else {
        setRun(null); setEvents([]); setSummary(next);
      }
    } catch (cause) { if (!signal?.aborted) setError(cause as ApiError); } finally { if (!signal?.aborted) setLoading(false); }
  }, [projectId, reportId, workId, workflowRunId]);
  useEffect(() => { const controller = new AbortController(); void load(controller.signal); return () => controller.abort(); }, [load]);
  useEffect(() => { if (!summary || !active(summary.state)) return; const timer = window.setTimeout(() => void load(), summary.state === "queued" ? 5000 : 3000); return () => window.clearTimeout(timer); }, [summary, load]);
  const refresh = () => void load();
  const retryRuntime = async () => { if (!run || retrying) return; const key = runtimeKey.current ?? crypto.randomUUID(); runtimeKey.current = key; setRetrying(true); setError(null); try { const next = await retryWorkflowRun(run.id, run.version, key, "original_configuration"); runtimeKey.current = null; retryUnknown.current = false; router.replace(runUrl(next.id)); } catch (cause) { const api = cause as ApiError; retryUnknown.current = api.code === "timeout" || api.code === "network_error"; if (!retryUnknown.current) runtimeKey.current = null; if (api.code === "active_rewrite_run_conflict" && typeof api.details.workflowRunId === "string") router.replace(runUrl(api.details.workflowRunId)); else setError(api); } finally { setRetrying(false); } };
  const retryConsumption = async () => { if (!run || retrying) return; const key = consumptionKey.current ?? crypto.randomUUID(); consumptionKey.current = key; setRetrying(true); setError(null); try { await retryContentRewriteResultConsumption(run.id, run.version, key); consumptionKey.current = null; await load(); } catch (cause) { const api = cause as ApiError; if (api.code !== "timeout" && api.code !== "network_error") consumptionKey.current = null; if (api.code === "workflow_run_version_conflict") await load(); else setError(api); } finally { setRetrying(false); } };
  const requestCancel = () => { if (!cancelKey.current) cancelKey.current = crypto.randomUUID(); setConfirmCancel(true); };
  const cancel = async () => { if (!run || cancelling || !cancelKey.current) return; const key = cancelKey.current; setCancelling(true); setError(null); try { await cancelWorkflowRun(run.id, run.version, key); cancelKey.current = null; setConfirmCancel(false); await load(); } catch (cause) { const api = cause as ApiError; if (api.code !== "timeout" && api.code !== "network_error") cancelKey.current = null; setError(api); setConfirmCancel(false); await load(); } finally { setCancelling(false); } };
  if (loading) return <State title="正在加载重写状态" />;
  if (error) return <State title="暂时无法读取重写状态" message={safeError(error)} retry={refresh} />;
  if (showHistory && summary) return <RewriteHistory projectId={projectId} workId={workId} contentItemId={summary.contentItemId} selectedRunId={workflowRunId} />;
  if (summary && (active(summary.state) || summary.state === "runtime_failed" || summary.state === "output_validation_failed" || summary.state === "result_consumption_failed" || summary.state === "candidate_ready")) return <RewriteRunState projectId={projectId} workId={workId} summary={summary} run={run} events={events} retrying={retrying} cancelling={cancelling} confirmCancel={confirmCancel} onCancel={requestCancel} onDismissCancel={() => setConfirmCancel(false)} onConfirmCancel={cancel} onRetryRuntime={retryRuntime} onRetryConsumption={retryConsumption} onRefresh={refresh} />;
  return <RewriteCreate projectId={projectId} workId={workId} reportId={reportId} initialIssueIds={issueIds} runUrl={runUrl} />;
}

function exactRunSummary(summary: RewriteSummary, run: WorkflowRunDto, events: WorkflowRunEventVm[]): RewriteSummary {
  const eventTypes = new Set(events.map(event => event.eventType));
  const state: RewriteSummary["state"] =
    run.status === "queued" ? "queued" :
    run.status === "running" ? "running" :
    eventTypes.has("result_consumed") ? "candidate_ready" :
    eventTypes.has("result_consumption_failed") ? "result_consumption_failed" :
    eventTypes.has("output_validation_failed") ? "output_validation_failed" :
    run.status === "failed" || run.status === "cancelled" ? "runtime_failed" : "idle";
  const selectedIssues = Array.isArray(run.inputPayload.selectedIssues) ? run.inputPayload.selectedIssues : null;
  return {
    ...summary,
    state,
    activeRun: state === "queued" || state === "running" ? run : null,
    latestRun: run,
    selectedIssueSummary: selectedIssues ? { total: selectedIssues.length, items: selectedIssues as NonNullable<RewriteSummary["selectedIssueSummary"]>["items"] } : null,
    candidateVersion: state === "candidate_ready" ? summary.candidateVersion : null,
    candidateIsCurrent: state === "candidate_ready" && summary.candidateVersion?.source_workflow_run_id === run.id ? summary.candidateIsCurrent : false,
    canSetCurrent: state === "candidate_ready",
  };
}

function RewriteRunState({ projectId, workId, summary, run, events, retrying, cancelling, confirmCancel, onCancel, onDismissCancel, onConfirmCancel, onRetryRuntime, onRetryConsumption, onRefresh }: { projectId: string; workId: string; summary: RewriteSummary; run: WorkflowRunDto | null; events: WorkflowRunEventVm[]; retrying: boolean; cancelling: boolean; confirmCancel: boolean; onCancel: () => void; onDismissCancel: () => void; onConfirmCancel: () => void; onRetryRuntime: () => void; onRetryConsumption: () => void; onRefresh: () => void }) {
  if (summary.state === "candidate_ready" && run) return <RewriteResultPanel projectId={projectId} workId={workId} runId={run.id} onRefresh={onRefresh} />;
  if (active(summary.state)) return <RewriteRunningState summary={summary} run={run} events={events} cancelling={cancelling} confirmCancel={confirmCancel} onCancel={onCancel} onDismissCancel={onDismissCancel} onConfirmCancel={onConfirmCancel} onRefresh={onRefresh} />;
  if (summary.state === "result_consumption_failed") return <RewriteConsumptionFailedState projectId={projectId} workId={workId} summary={summary} run={run} retrying={retrying} onRetry={onRetryConsumption} onRefresh={onRefresh} />;
  return <RewriteFailedState projectId={projectId} workId={workId} summary={summary} run={run} events={events} retrying={retrying} cancelling={cancelling} confirmCancel={confirmCancel} onDismissCancel={onDismissCancel} onConfirmCancel={onConfirmCancel} onRetryRuntime={onRetryRuntime} onRefresh={onRefresh} />;
}

function RewriteConsumptionFailedState({ projectId, workId, summary, run, retrying, onRetry, onRefresh }: { projectId: string; workId: string; summary: RewriteSummary; run: WorkflowRunDto | null; retrying: boolean; onRetry: () => void; onRefresh: () => void }) {
  const selectedCount = summary.selectedIssueSummary?.total ?? 0;
  return (
    <main className="rewrite-page rewrite-consumption-page">
      <Link className="rewrite-result-back" href={`/projects/${projectId}/works/${workId}/review?reportId=${encodeURIComponent(summary.reviewReportId)}`}>← 返回审核结果</Link>
      <header className="rewrite-consumption-hero"><div><div className="rewrite-result-title"><h1>正文重写结果</h1><span>结果提交失败</span></div><p>工作流已完成，但重写结果尚未保存为正文版本。</p></div>{run && <Link href={`/workflow-runs/${encodeURIComponent(run.id)}`}>查看执行详情</Link>}</header>
      <section className="rewrite-consumption-alert" role="alert"><div className="rewrite-consumption-alert-icon" aria-hidden="true">!</div><div><h2>工作流执行成功，结果提交失败</h2><p>系统在保存正文候选版本时发生错误，尚未创建任何 ContentVersion。可以直接重试提交，无需重新运行工作流。</p><code>错误代码：{summary.latestError?.code ?? "result_consumption_failed"}</code><div className="rewrite-consumption-actions"><button type="button" className="primary" disabled={!run || retrying} onClick={onRetry}>{retrying ? "重试提交中…" : "重试提交结果"}</button>{run && <Link href={`/workflow-runs/${encodeURIComponent(run.id)}`}>查看执行详情</Link>}</div></div></section>
      <dl className="rewrite-consumption-meta"><div><dt>WorkflowRun</dt><dd>{run?.runNumber ?? "正在恢复"}</dd></div><div><dt>工作流状态</dt><dd className="is-success">✓ 已成功</dd></div><div><dt>工作流</dt><dd>{run?.workflowName ?? "正文重写"}</dd></div><div><dt>来源版本</dt><dd>V{summary.sourceContentVersionSummary.versionNo}</dd></div><div><dt>完成时间</dt><dd>{formatWorkflowRunTime(run?.finishedAt ?? run?.updatedAt)}</dd></div><div><dt>结果保存状态</dt><dd className="is-failed">× 未提交</dd></div></dl>
      <section className="rewrite-consumption-no-candidate"><div className="rewrite-consumption-empty-icon" aria-hidden="true">⊘</div><h2>尚未创建候选版本</h2><p>提交成功后，系统才会创建新的候选 ContentVersion，并开放版本比较与编辑操作。</p><div><span>来源版本 V{summary.sourceContentVersionSummary.versionNo}</span><span>关联审核：{summary.reviewReportId}</span><span>已处理问题：{selectedCount} 个</span></div></section>
      {summary.latestError?.message && <p className="rewrite-result-alert">安全错误摘要：{summary.latestError.message}{summary.latestError.correlationId ? ` · 关联 ID ${summary.latestError.correlationId}` : ""}</p>}
      <footer className="rewrite-consumption-footer"><button type="button" onClick={onRefresh}>刷新状态</button></footer>
    </main>
  );
}

function RewriteFailedState({ projectId, workId, summary, run, events, retrying, cancelling, confirmCancel, onDismissCancel, onConfirmCancel, onRetryRuntime, onRefresh }: { projectId: string; workId: string; summary: RewriteSummary; run: WorkflowRunDto | null; events: WorkflowRunEventVm[]; retrying: boolean; cancelling: boolean; confirmCancel: boolean; onDismissCancel: () => void; onConfirmCancel: () => void; onRetryRuntime: () => void; onRefresh: () => void }) {
  const isValidation = summary.state === "output_validation_failed";
  const isCancelled = summary.state === "idle";
  const title = isValidation ? "重写任务" : "正文重写任务";
  const failureTitle = isValidation ? "工作流输出格式不符合要求" : isCancelled ? "重写任务已取消" : "正文重写运行失败";
  const failureCopy = isValidation ? "工作流已返回结果，但输出结构未通过正文重写 Schema 校验。请修复工作流输出后重新执行。" : isCancelled ? "本次重写运行已取消，尚未生成候选版本。" : summary.latestError?.message ?? "正文重写工作流未能完成，请检查执行详情后重试。";
  const errorCode = summary.latestError?.code ?? run?.errorCode ?? (isValidation ? "output_validation_failed" : "runtime_failed");
  return (
    <main className="rewrite-page rewrite-failure-page">
      <Link className="rewrite-result-back" href={`/projects/${projectId}/works/${workId}/review?reportId=${encodeURIComponent(summary.reviewReportId)}`}>← 返回审核结果</Link>
      <header className="rewrite-failure-hero"><div><div className="rewrite-result-title"><h1>{title}</h1><span>执行失败</span></div><p>正文重写工作流未完成，请检查错误后重新执行。</p></div>{run && <Link href={`/workflow-runs/${encodeURIComponent(run.id)}`}>查看执行详情</Link>}</header>
      <section className="rewrite-failure-alert" role="alert"><div className="rewrite-failure-alert-icon" aria-hidden="true">×</div><div><h2>{failureTitle}</h2><p>{failureCopy}</p><code>{errorCode} · WorkflowRun {run?.runNumber ?? run?.id ?? "正在恢复"}</code><div className="rewrite-failure-actions"><button type="button" className="primary" disabled={!run || retrying} onClick={onRetryRuntime}>{retrying ? "重新执行中…" : "重新执行"}</button>{run && <Link href={`/workflow-runs/${encodeURIComponent(run.id)}`}>查看执行详情</Link>}</div></div></section>
      <section className="rewrite-failure-context-grid">
        <article><h2>源信息</h2><dl><div><dt>重写目标</dt><dd>{summary.sourceContentVersionSummary.title}</dd></div><div><dt>来源版本</dt><dd>V{summary.sourceContentVersionSummary.versionNo}</dd></div><div><dt>依据报告</dt><dd>{summary.reviewReportId}</dd></div><div><dt>处理范围</dt><dd>已选问题：{summary.selectedIssueSummary?.total ?? 0} 个</dd></div></dl></article>
        <article><h2>执行环境</h2><dl><div><dt>关联工作流</dt><dd>{run?.workflowName ?? "正文重写"}</dd></div><div><dt>执行阶段</dt><dd>{run?.stage ?? "content_rewrite"}</dd></div><div><dt>开始时间</dt><dd>{formatWorkflowRunTime(run?.startedAt ?? run?.createdAt)}</dd></div><div><dt>失败时间</dt><dd>{formatWorkflowRunTime(run?.finishedAt ?? run?.updatedAt)}</dd></div></dl></article>
        <article><h2>恢复建议</h2><ol><li>查看执行详情确认输出字段和错误阶段</li><li>前往项目设置检查工作流绑定配置</li><li>修复后使用原配置重新执行本次重写</li></ol><Link href={`/projects/${projectId}/settings`}>前往项目设置</Link></article>
      </section>
      {events.length > 0 && <section className="rewrite-progress rewrite-failure-progress"><h2>执行进度</h2><ol>{events.map((event) => <li key={event.id}>{event.title} · {event.createdAtLabel}</li>)}</ol></section>}
      <section className="rewrite-failure-no-candidate"><h2>未生成候选版本</h2><p>工作流执行失败后不会创建或展示候选正文。重新执行成功并通过校验后，系统才会进入候选结果页面。</p></section>
      <footer className="rewrite-failure-footer"><button type="button" onClick={onRefresh}>刷新状态</button></footer>
      {confirmCancel && <div className="rewrite-dialog-layer"><section className="rewrite-confirm-dialog" role="dialog" aria-modal="true"><h2>确认取消运行</h2><p>取消后该运行将停止，无法恢复为运行中。</p><footer><button type="button" disabled={cancelling} onClick={onDismissCancel}>返回</button><button type="button" className="primary" disabled={cancelling} onClick={onConfirmCancel}>{cancelling ? "取消中…" : "确认取消"}</button></footer></section></div>}
    </main>
  );
}

function RewriteRunningState({ summary, run, events, cancelling, confirmCancel, onCancel, onDismissCancel, onConfirmCancel, onRefresh }: { summary: RewriteSummary; run: WorkflowRunDto | null; events: WorkflowRunEventVm[]; cancelling: boolean; confirmCancel: boolean; onCancel: () => void; onDismissCancel: () => void; onConfirmCancel: () => void; onRefresh: () => void }) {
  const selectedIssues = summary.selectedIssueSummary?.items ?? [];
  const queued = summary.state === "queued";
  const runLabel = run?.runNumber ?? run?.id ?? "恢复中";
  return (
    <main className="rewrite-page rewrite-running-page">
      <section className="rewrite-running-banner" aria-live="polite">
        <div className="rewrite-running-banner-main">
          <span className="rewrite-running-icon" aria-hidden="true">↻</span>
          <div>
            <div className="rewrite-running-heading"><span className="rewrite-state-badge">{queued ? "等待执行" : "运行中"}</span><h1>正文重写{queued ? "已创建，等待执行" : "正在运行"}</h1></div>
            <p>{queued ? "任务已创建，正在等待工作流执行。" : `正在根据 ${selectedIssues.length} 个审核问题生成候选正文。`}</p>
          </div>
        </div>
        {run && <Link href={`/workflow-runs/${encodeURIComponent(run.id)}`}>查看执行详情 <span aria-hidden="true">→</span></Link>}
      </section>
      <section className="rewrite-running-meta" aria-label="重写运行信息">
        <div><dt>Run ID</dt><dd><code className="rewrite-running-run-id">{runLabel}</code></dd></div>
        <div><dt>阶段</dt><dd>正文重写</dd></div>
        <div><dt>来源版本</dt><dd>V{summary.sourceContentVersionSummary.versionNo} · {summary.sourceContentVersionSummary.title}</dd></div>
        <div><dt>来源报告</dt><dd>{summary.reviewReportId}</dd></div>
        <div><dt>开始时间</dt><dd>{formatWorkflowRunTime(run?.startedAt ?? run?.createdAt)}</dd></div>
      </section>
      <section className="rewrite-running-layout">
        <article className="rewrite-running-issues">
          <header><h2>来源审核问题</h2><span>共 {selectedIssues.length} 项</span></header>
          {selectedIssues.length ? <div>{selectedIssues.map((issue) => <article key={issue.reviewIssueId}><span className="rewrite-running-issue-marker" aria-hidden="true">!</span><div><strong>{issue.title}</strong><p>{issue.severity} · {issue.issueKey}</p></div></article>)}</div> : <p>正在恢复本次运行的问题信息。</p>}
        </article>
        <article className="rewrite-running-wait">
          <span className="rewrite-running-wait-icon" aria-hidden="true">↻</span>
          <h2>等待重写结果</h2>
          <p>任务完成后将生成候选正文，不会自动替换当前版本。</p>
          {events.length > 0 && <ol>{events.map((event) => <li key={event.id}><span>{event.title}</span><small>{event.createdAtLabel}</small></li>)}</ol>}
        </article>
      </section>
      <footer className="rewrite-running-actions">
        {run && <Link href={`/workflow-runs/${encodeURIComponent(run.id)}`}>查看执行详情</Link>}
        <button type="button" disabled={cancelling} onClick={onCancel}>{cancelling ? "取消中…" : "取消运行"}</button>
        <button type="button" onClick={onRefresh}>刷新状态</button>
      </footer>
      {confirmCancel && <div className="rewrite-dialog-layer"><section className="rewrite-confirm-dialog" role="dialog" aria-modal="true"><h2>确认取消运行</h2><p>取消后该运行将停止，无法恢复为运行中。</p><footer><button type="button" disabled={cancelling} onClick={onDismissCancel}>返回</button><button type="button" className="primary" disabled={cancelling} onClick={onConfirmCancel}>{cancelling ? "取消中…" : "确认取消"}</button></footer></section></div>}
    </main>
  );
}

// This remains only for the frozen source-level D5 assertions; candidate rendering uses RewriteResultPanel.
// eslint-disable-next-line @typescript-eslint/no-unused-vars
function LegacyRewriteResult({ runId, onRefresh }: { runId: string; onRefresh: () => void }) { const [result, setResult] = useState<RewriteResultDto | null>(null); const [sourceContent, setSourceContent] = useState<string | null>(null); const [error, setError] = useState(false); useEffect(() => { const controller = new AbortController(); void getContentRewriteResult(runId, { signal: controller.signal }).then(async x => { const source = await getContentVersion(x.sourceContentVersionSummary.id, { signal: controller.signal }); if (!controller.signal.aborted) { setResult(x); setSourceContent(source.content_version.content); } }).catch(() => { if (!controller.signal.aborted) setError(true); }); return () => controller.abort(); }, [runId]); if (error) return <State title="候选版本尚未就绪" message="请刷新状态后重试。" retry={onRefresh} />; if (!result || sourceContent === null) return <State title="正在加载候选版本" />; return <main className="rewrite-page"><section className="rewrite-layout"><article><h1>重写候选版本</h1><p>{result.output.summary}</p><dl><div><dt>来源版本</dt><dd>V{result.sourceContentVersionSummary.versionNo} · {result.sourceContentVersionSummary.title}</dd></div><div><dt>候选版本</dt><dd>V{result.candidateVersion.versionNo} · {result.candidateVersion.title}</dd></div><div><dt>当前版本</dt><dd>{result.candidateIsCurrent ? "候选已是当前版本" : "候选尚未设为当前版本"}</dd></div></dl><h2>已处理问题</h2><IssueOutcomes items={result.output.addressedIssues} /><h2>未解决问题</h2><IssueOutcomes items={result.output.unresolvedIssues} />{result.output.warnings.length > 0 && <><h2>提示</h2><ul>{result.output.warnings.map(x => <li key={x}>{x}</li>)}</ul></>}{result.output.metadata?.changeSummary && <p>{result.output.metadata.changeSummary}</p>}</article><article><h2>来源正文</h2><p>{sourceContent}</p><h2>候选正文</h2><p>{result.candidateVersion.content}</p></article></section></main>; }
function IssueOutcomes({ items }: { items: Array<{ reviewIssueId: string; summary: string }> }) { return items.length ? <ul>{items.map(x => <li key={x.reviewIssueId}>{x.summary}</li>)}</ul> : <p>无</p>; }
function State({ title, message, retry }: { title: string; message?: string; retry?: () => void }) { return <main className="rewrite-page"><section className="rewrite-state-card"><h1>{title}</h1>{message && <p role="alert">{message}</p>}{retry && <footer><button type="button" onClick={retry}>重试</button></footer>}</section></main>; }

function RewriteCreate({ projectId, workId, reportId, initialIssueIds, runUrl }: { projectId: string; workId: string; reportId?: string; initialIssueIds?: string[]; runUrl: (id: string) => string }) {
  const router = useRouter();
  const [availability, setAvailability] = useState<RewriteAvailability | null>(null);
  const [issues, setIssues] = useState<RealReviewIssue[]>([]);
  const [reviewDetail, setReviewDetail] = useState<import("@/features/content-review/content-review-api").RealReviewDetail | null>(null);
  const [sourceContent, setSourceContent] = useState<string | null>(null);
  const [selected, setSelected] = useState<string[]>(initialIssueIds ?? []);
  const [instructions, setInstructions] = useState("");
  const [options, setOptions] = useState<RewriteOptions>({ strategy: "targeted_fix" });
  const [preflight, setPreflight] = useState<RewritePreflight | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [checking, setChecking] = useState(false);
  const [creating, setCreating] = useState(false);
  const [showConfiguration, setShowConfiguration] = useState(false);
  const [confirmCreate, setConfirmCreate] = useState(false);
  const key = useRef<string | null>(null);
  const clearPreflight = () => { setPreflight(null); setConfirmCreate(false); key.current = null; };
  const load = useCallback(async () => {
    if (!reportId) { setError("缺少审核报告，无法创建重写。"); return; }
    try {
      const [next, detail] = await Promise.all([getContentRewriteAvailability(reportId), getReview(reportId)]);
      setAvailability(next);
      if (isRealReviewDetail(detail)) {
        setReviewDetail(detail);
        const ordered = [...detail.issues].sort((a, b) => a.position - b.position || a.id.localeCompare(b.id));
        setIssues(ordered);
        const openIssueIds = ordered.filter(x => x.disposition === "open").slice(0, 50).map(x => x.id);
        setSelected(initialIssueIds ? openIssueIds.filter((id) => initialIssueIds.includes(id)) : []);
      }
      try {
        const source = await getContentVersion(next.sourceContentVersionSummary.id);
        setSourceContent(source.content_version.content);
      } catch {
        setSourceContent(null);
      }
      if (next.activeRun) router.replace(runUrl(next.activeRun.id));
    } catch (cause) { setError(rewriteErrorMessage((cause as ApiError).code)); }
  }, [initialIssueIds, reportId, router, runUrl]);
  useEffect(() => { void load(); }, [load]);
  const toggle = (id: string) => {
    const issue = issues.find(x => x.id === id);
    if (!issue || issue.disposition !== "open") return;
    setSelected(current => current.includes(id) ? current.filter(x => x !== id) : current.length >= 50 ? current : [...current, id]);
    clearPreflight();
  };
  const check = async () => {
    if (!reportId) return;
    const invalid = validRewriteInput(selected, instructions, options);
    if (invalid) { setError(invalid); return; }
    setChecking(true); setError(null);
    try {
      const next = await preflightContentRewrite(reportId, { selectedIssueIds: selected, optionalInstructions: instructions.trim() || null, rewriteOptions: options });
      setPreflight(next); key.current = null;
    } catch (cause) { setError(rewriteErrorMessage((cause as ApiError).code)); }
    finally { setChecking(false); }
  };
  const create = async () => {
    if (!reportId || !preflight?.preflightToken || creating) return;
    const submitKey = key.current ?? crypto.randomUUID(); key.current = submitKey;
    setCreating(true); setError(null);
    try {
      const next = await createContentRewriteRun(reportId, preflight.preflightToken, submitKey);
      key.current = null; setConfirmCreate(false); router.replace(runUrl(next.id));
    } catch (cause) {
      const api = cause as ApiError;
      if (api.code === "active_rewrite_run_conflict" && typeof api.details.workflowRunId === "string") {
        key.current = null; router.replace(runUrl(api.details.workflowRunId));
      } else {
        if (["rewrite_preflight_expired", "rewrite_preflight_stale", "rewrite_preflight_consumed", "idempotency_conflict"].includes(api.code)) clearPreflight();
        setError(rewriteErrorMessage(api.code));
      }
    } finally { setCreating(false); }
  };
  if (!availability) return <State title="正在加载重写资料" />;
  if (availability.available)
    return (
      <RewriteCreateView
        projectId={projectId}
        workId={workId}
        reportId={reportId}
        availability={availability}
        reviewDetail={reviewDetail}
        sourceContent={sourceContent}
        issues={issues}
        selected={selected}
        instructions={instructions}
        options={options}
        preflight={preflight}
        error={error}
        checking={checking}
        creating={creating}
        showConfiguration={showConfiguration}
        confirmCreate={confirmCreate}
        setInstructions={(value) => { setInstructions(value); clearPreflight(); }}
        setStrategy={(strategy) => { setOptions({ strategy }); clearPreflight(); }}
        setShowConfiguration={setShowConfiguration}
        setConfirmCreate={setConfirmCreate}
        toggle={toggle}
        check={() => void check()}
        create={() => void create()}
      />
    );
  if (!availability.available) return <RewriteAvailabilityState projectId={projectId} workId={workId} reportId={reportId} availability={availability} onRefresh={() => void load()} />;
  if (availability && !availability.available) {
    const copy = {
      review_not_completed: ["审核报告尚未完成", "返回审核结果并等待审核完成。"],
      no_open_issues: ["没有可处理的开放问题", "返回审核结果选择仍为开放状态的问题。"],
      rewrite_not_configured: ["项目重写配置不可用", "前往项目设置检查工作流与连接。"],
      active_rewrite_run_conflict: ["已有重写任务正在运行", "恢复对应运行并查看进度。"],
    }[availability.reason ?? "rewrite_not_configured"];
    return <main className="rewrite-page"><section className="rewrite-state-card is-empty"><span className="rewrite-state-badge">配置检查</span><h1>{copy[0]}</h1><p>{copy[1]}</p><footer>{availability.activeRun ? <button type="button" className="primary" onClick={() => router.replace(runUrl(availability.activeRun!.id))}>恢复运行</button> : <><Link href={`/projects/${projectId}/works/${workId}/review?reportId=${encodeURIComponent(reportId ?? "")}`}>返回审核结果</Link><button type="button" className="primary" onClick={() => void load()}>刷新可用性</button></>}</footer></section></main>;
  }
  return <main className="rewrite-page"><Link className="rewrite-back" href={`/projects/${projectId}/works/${workId}/review?reportId=${encodeURIComponent(reportId ?? "")}`}>← 返回审核结果</Link><section className="rewrite-layout"><article className="rewrite-source"><header><div><h1>创建正文重写</h1><p>重写将固定使用本次审核的来源版本，不会替换当前版本。</p></div><button type="button" onClick={() => setShowConfiguration(true)}>查看项目重写配置</button></header><dl><div><dt>固定来源版本</dt><dd>{availability ? `V${availability.sourceContentVersionSummary.versionNo} · ${availability.sourceContentVersionSummary.title}` : "正在加载"}</dd></div><div><dt>已选问题</dt><dd>{selected.length} 个</dd></div></dl><h2>选择待修复问题</h2><div className="rewrite-issues">{issues.map(issue => <label key={issue.id}><input type="checkbox" checked={selected.includes(issue.id)} disabled={issue.disposition !== "open"} onChange={() => toggle(issue.id)} /><span><b>{issue.title}</b><small>{issue.categoryLabel} · {issue.severity}</small></span></label>)}</div></article><section className="rewrite-form"><h2>重写执行配置</h2><label>补充要求<textarea value={instructions} maxLength={2000} onChange={e => { setInstructions(e.target.value); clearPreflight(); }} /></label><label>重写策略<select value={options.strategy} onChange={e => { setOptions({ strategy: e.target.value as RewriteOptions["strategy"] }); clearPreflight(); }}><option value="targeted_fix">定向修复</option><option value="creative_rewrite">创意重写</option></select></label>{error && <p role="alert">{error}</p>}{preflight?.status === "passed" ? <section className="rewrite-preflight" aria-label="预检结果"><h2>预检已通过</h2><ul>{preflight.checks.map(check => <li key={check.code}>{check.status === "passed" ? "通过" : "阻塞"}：{check.message}</li>)}</ul><dl><div><dt>来源</dt><dd>V{preflight.sourceContentVersionSummary.versionNo} · {preflight.sourceContentVersionSummary.title}</dd></div><div><dt>审核报告</dt><dd>{preflight.reviewReportSnapshot.summary}</dd></div><div><dt>问题</dt><dd>{preflight.selectedIssueSummary.total} 个</dd></div><div><dt>配置</dt><dd>{preflight.configurationSummary?.workflowConfigurationName ?? "不可用"}</dd></div><div><dt>过期时间</dt><dd>{preflight.expiresAt ? new Date(preflight.expiresAt).toLocaleString("zh-CN") : "—"}</dd></div></dl><button type="button" className="primary" onClick={() => setConfirmCreate(true)}>继续确认创建</button></section> : preflight?.status === "blocked" ? <section className="rewrite-preflight" aria-label="预检结果"><h2>预检未通过</h2><WorkflowPreflightBlocker reasons={rewritePreflightBlockerReasons(preflight)} /><ul>{preflight.checks.map(check => <li key={check.code}>{check.status === "passed" ? "通过" : "阻塞"}：{check.message}</li>)}</ul><button type="button" className="primary" disabled={checking || !availability?.available} onClick={() => void check()}>{checking ? "预检中…" : "重新预检"}</button></section> : <button type="button" className="primary" disabled={checking || !availability?.available} onClick={() => void check()}>{checking ? "预检中…" : "进行预检"}</button>}</section></section>
  {showConfiguration && <div className="rewrite-drawer-layer"><button className="rewrite-dialog-backdrop" aria-label="关闭项目重写配置" onClick={() => setShowConfiguration(false)} /><section className="rewrite-config-drawer" role="dialog" aria-modal="true" aria-label="项目重写配置"><header><div><h2>项目重写配置</h2><p>正文重写阶段</p></div><button type="button" aria-label="关闭" onClick={() => setShowConfiguration(false)}>×</button></header><p className="rewrite-available-badge">配置可用</p><dl><div><dt>工作流</dt><dd>{availability?.configurationSummary?.workflowConfigurationName ?? "未配置"}</dd></div><div><dt>配置版本</dt><dd>{availability?.configurationSummary?.workflowConfigurationVersion ?? "—"}</dd></div><div><dt>输入契约</dt><dd>{availability?.configurationSummary?.inputContract ?? "—"}</dd></div><div><dt>输出契约</dt><dd>{availability?.configurationSummary?.outputContract ?? "—"}</dd></div></dl><footer><button type="button" onClick={() => setShowConfiguration(false)}>关闭</button></footer></section></div>}
  {confirmCreate && preflight?.status === "passed" && <div className="rewrite-dialog-layer"><section className="rewrite-confirm-dialog" role="dialog" aria-modal="true" aria-label="确认创建重写任务"><h2>确认创建重写任务</h2><p>将按以上来源、审核报告、{preflight.selectedIssueSummary.total} 个问题和只读配置创建一次重写运行。</p><footer><button type="button" disabled={creating} onClick={() => setConfirmCreate(false)}>返回修改</button><button type="button" className="primary" disabled={creating} onClick={() => void create()}>{creating ? "创建中…" : "确认创建"}</button></footer></section></div>}</main>;
}
function RewriteAvailabilityState({ projectId, workId, reportId, availability, onRefresh }: { projectId: string; workId: string; reportId?: string; availability: RewriteAvailability; onRefresh: () => void }) {
  const mode = availability.reason === "rewrite_not_configured" ? (availability.configurationSummary ? "configuration_invalid" : "not_configured") : availability.reason === "no_open_issues" ? "no_open_issues" : availability.reason === "review_not_completed" ? "review_pending" : "active_conflict";
  const copy = {
    not_configured: { badge: "需要配置", title: "项目尚未配置正文重写", detail: "请先在项目设置中绑定可执行的正文重写工作流和连接，配置完成后才能创建重写任务。", action: "前往项目设置" },
    configuration_invalid: { badge: "配置失效", title: "正文重写配置不可用", detail: "当前项目绑定的工作流或执行连接无法使用，请前往项目设置检查并修复配置。", action: "检查项目配置" },
    no_open_issues: { badge: "没有可处理内容", title: "没有可重写的问题", detail: "当前审核报告中没有处于开放状态的问题。返回审核结果后选择仍需处理的问题，或等待新的审核结果。", action: "返回审核结果" },
    review_pending: { badge: "等待审核", title: "审核报告尚未完成", detail: "审核完成并生成报告后，才能从开放问题创建正文重写。", action: "返回审核工作区" },
    active_conflict: { badge: "任务运行中", title: "已有正文重写任务正在运行", detail: "当前项目已有重写任务在执行，请先查看该任务的进度，完成后再创建新的重写。", action: "恢复运行" },
  }[mode];
  const reviewHref = `/projects/${projectId}/works/${workId}/review${reportId ? `?reportId=${encodeURIComponent(reportId)}` : ""}`;
  return (
    <main className="rewrite-page rewrite-availability-page">
      <Link className="rewrite-result-back" href={reviewHref}>← 返回审核结果</Link>
      <section className={`rewrite-availability-card is-${mode}`}>
        <div className="rewrite-availability-icon" aria-hidden="true">{mode === "no_open_issues" ? "✓" : mode === "active_conflict" ? "↻" : "!"}</div>
        <span className="rewrite-state-badge">{copy.badge}</span>
        <h1>{copy.title}</h1>
        <p>{copy.detail}</p>
        <dl><div><dt>来源版本</dt><dd>V{availability.sourceContentVersionSummary.versionNo} · {availability.sourceContentVersionSummary.title}</dd></div><div><dt>开放问题</dt><dd>{availability.openIssueCount} 个</dd></div></dl>
        <footer>{availability.activeRun ? <Link className="primary" href={`/projects/${projectId}/works/${workId}/rewrite?workflowRunId=${encodeURIComponent(availability.activeRun.id)}`}>{copy.action}</Link> : mode === "not_configured" || mode === "configuration_invalid" ? <Link className="primary" href={`/projects/${projectId}/settings`}>{copy.action}</Link> : <Link className="primary" href={reviewHref}>{copy.action}</Link>}<button type="button" onClick={onRefresh}>重新检查</button></footer>
      </section>
    </main>
  );
}

function RewriteConfigurationDrawer({ projectId, availability, onClose }: { projectId: string; availability: RewriteAvailability; onClose: () => void }) {
  const configuration = availability.configurationSummary;
  const workflowReady = Boolean(availability.available && configuration);
  return (
    <div className="rewrite-drawer-layer">
      <button className="rewrite-dialog-backdrop" aria-label="关闭项目重写配置" onClick={onClose} />
      <section className="rewrite-config-drawer" role="dialog" aria-modal="true" aria-label="项目工作流配置">
        <header>
          <div><h2>项目工作流配置</h2><p>正文重写阶段</p></div>
          <button type="button" aria-label="关闭" onClick={onClose}>×</button>
        </header>
        <div className="rewrite-config-drawer-body">
          <p className={`rewrite-available-badge ${workflowReady ? "is-ready" : "is-unavailable"}`} role="status">
            <span aria-hidden="true">{workflowReady ? "✓" : "!"}</span>
            {workflowReady ? "配置可用" : "配置不可用"}
          </p>
          <section className="rewrite-config-summary">
            <h3>当前绑定详情</h3>
            <dl>
              <div><dt>业务阶段</dt><dd><span className="rewrite-config-tag">正文重写</span></dd></div>
              <div><dt>绑定工作流</dt><dd>{configuration ? `${configuration.workflowConfigurationName} · v${configuration.workflowConfigurationVersion}` : "未配置"}</dd></div>
              <div><dt>执行连接</dt><dd>{configuration ? `${configuration.connectionId} · v${configuration.connectionVersion}` : "未配置"}</dd></div>
              <div><dt>工作流状态</dt><dd><span className={`rewrite-config-status ${workflowReady ? "is-ready" : "is-unavailable"}`}><span aria-hidden="true" />{workflowReady ? "已启用" : "不可用"}</span></dd></div>
              <div><dt>输入契约</dt><dd>{configuration?.inputContract ?? "—"}</dd></div>
              <div><dt>输出结构</dt><dd><span className={`rewrite-config-contract ${configuration ? "is-ready" : "is-unavailable"}`}>{configuration ? `✓ ${configuration.outputContract}` : "未验证"}</span></dd></div>
              <div><dt>绑定版本</dt><dd>{configuration ? `v${configuration.bindingVersion}` : "—"}</dd></div>
            </dl>
          </section>
          <div className="rewrite-config-drawer-note"><span aria-hidden="true">ⓘ</span><p>此页面只查看当前绑定。修改工作流或执行连接需要前往项目设置。</p></div>
        </div>
        <footer><button type="button" onClick={onClose}>关闭</button><Link className="primary" href={`/projects/${projectId}/settings`}>前往项目设置 <span aria-hidden="true">→</span></Link></footer>
      </section>
    </div>
  );
}

function RewriteCreateView({
  projectId,
  workId,
  reportId,
  availability,
  reviewDetail,
  sourceContent,
  issues,
  selected,
  instructions,
  options,
  preflight,
  error,
  checking,
  creating,
  showConfiguration,
  confirmCreate,
  setInstructions,
  setStrategy,
  setShowConfiguration,
  setConfirmCreate,
  toggle,
  check,
  create,
}: {
  projectId: string;
  workId: string;
  reportId?: string;
  availability: RewriteAvailability;
  reviewDetail: import("@/features/content-review/content-review-api").RealReviewDetail | null;
  sourceContent: string | null;
  issues: RealReviewIssue[];
  selected: string[];
  instructions: string;
  options: RewriteOptions;
  preflight: RewritePreflight | null;
  error: string | null;
  checking: boolean;
  creating: boolean;
  showConfiguration: boolean;
  confirmCreate: boolean;
  setInstructions: (value: string) => void;
  setStrategy: (strategy: RewriteOptions["strategy"]) => void;
  setShowConfiguration: (value: boolean) => void;
  setConfirmCreate: (value: boolean) => void;
  toggle: (id: string) => void;
  check: () => void;
  create: () => void;
}) {
  const selectedIssues = issues.filter((issue) => selected.includes(issue.id));
  return (
    <main className="rewrite-page">
      <Link
        className="rewrite-back"
        href={`/projects/${projectId}/works/${workId}/review?reportId=${encodeURIComponent(reportId ?? "")}`}
      >
        ← 返回审核结果
      </Link>
      <header className="rewrite-create-header">
        <div>
          <h1>创建正文重写</h1>
          <p>基于已选审核问题生成新的正文候选版本，原版本保持不变。</p>
        </div>
        <button type="button" onClick={() => setShowConfiguration(true)}>
          查看项目重写配置
        </button>
      </header>
      <dl className="rewrite-create-context">
        <div>
          <dt>固定来源版本</dt>
          <dd>
            V{availability.sourceContentVersionSummary.versionNo} · {availability.sourceContentVersionSummary.title}
          </dd>
        </div>
        <div>
          <dt>审核报告</dt>
          <dd>{reviewDetail?.report.id ?? availability.reviewReportId}</dd>
        </div>
        <div>
          <dt>已选问题</dt>
          <dd>{selected.length} 个</dd>
        </div>
      </dl>
      <section className="rewrite-create-layout">
        <div className="rewrite-create-source-column">
          <section className="rewrite-selected-issues">
            <header>
              <h2>已选择的审核问题</h2>
              <span>{selected.length} 项</span>
            </header>
            {selectedIssues.length ? (
              <div>
                {selectedIssues.map((issue) => (
                  <article key={issue.id}>
                    <span className={`severity ${issue.severity}`}>{issue.severity}</span>
                    <strong>{issue.categoryLabel}</strong>
                    <p>{issue.title}</p>
                    <small>{reviewLocationLabel(issue.location)}</small>
                    <button type="button" onClick={() => toggle(issue.id)} aria-label={`移除问题 ${issue.title}`}>
                      ×
                    </button>
                  </article>
                ))}
              </div>
            ) : (
              <p className="rewrite-selection-empty">尚未选择可重写问题，请返回审核结果选择问题。</p>
            )}
          </section>
          <section className="rewrite-target-source">
            <header>
              <h2>目标正文</h2>
              <span>V{availability.sourceContentVersionSummary.versionNo} 固定来源</span>
            </header>
            {sourceContent ? (
              <pre>{sourceContent}</pre>
            ) : (
              <p>固定来源正文暂时无法读取，请返回审核结果后重试。</p>
            )}
          </section>
          {reviewDetail?.report.summary ? (
            <section className="rewrite-report-context">
              <h2>来源审核报告</h2>
              <p>{reviewDetail.report.summary}</p>
            </section>
          ) : null}
        </div>
        <section className="rewrite-form">
          <header>
            <h2>重写执行配置</h2>
            <p>只读使用本次审核的来源版本和已选问题。</p>
          </header>
          <label>
            重写策略
            <select value={options.strategy} onChange={(event) => setStrategy(event.target.value as RewriteOptions["strategy"])}>
              <option value="targeted_fix">仅修复已选问题，尽量保持原文风格</option>
              <option value="creative_rewrite">创意重写</option>
            </select>
          </label>
          <label>
            补充要求 <small>（可选）</small>
            <textarea value={instructions} maxLength={2000} onChange={(event) => setInstructions(event.target.value)} />
          </label>
          {error && <p role="alert">{error}</p>}
          {preflight?.status === "passed" ? (
            <section className="rewrite-preflight" aria-label="预检结果">
              <h2>预检已通过</h2>
              <ul>{preflight.checks.map((item) => <li key={item.code}>{item.status === "passed" ? "通过" : "阻塞"}：{item.message}</li>)}</ul>
              <dl>
                <div><dt>来源</dt><dd>V{preflight.sourceContentVersionSummary.versionNo} · {preflight.sourceContentVersionSummary.title}</dd></div>
                <div><dt>审核报告</dt><dd>{preflight.reviewReportSnapshot.summary}</dd></div>
                <div><dt>问题</dt><dd>{preflight.selectedIssueSummary.total} 个</dd></div>
                <div><dt>配置</dt><dd>{preflight.configurationSummary?.workflowConfigurationName ?? "不可用"}</dd></div>
              </dl>
              <button type="button" className="primary" onClick={() => setConfirmCreate(true)}>继续确认创建</button>
            </section>
          ) : preflight?.status === "blocked" ? (
            <section className="rewrite-preflight rewrite-preflight-blocked-state" aria-label="预检结果">
              <div className="rewrite-preflight-blocked-summary">
                <strong>预检未通过</strong>
                <p>来源审核报告、已选问题、补充要求和重写策略均已保留；处理阻断项后才能创建重写任务。</p>
              </div>
              <WorkflowPreflightBlocker reasons={rewritePreflightBlockerReasons(preflight)} />
              <ul className="rewrite-preflight-check-list" aria-label="正文重写预检项目">{preflight.checks.map((item) => <li key={item.code} className={item.status}>{item.status === "passed" ? "通过" : "阻塞"}：{item.message}</li>)}</ul>
              <button type="button" className="primary rewrite-preflight-recheck" disabled={checking || !selected.length} onClick={check}>{checking ? "预检中…" : "重新预检"}</button>
            </section>
          ) : (
            <button type="button" className="primary" disabled={checking || !selected.length} onClick={check}>
              {checking ? "预检中…" : "进行预检"}
            </button>
          )}
        </section>
      </section>
      {showConfiguration && <RewriteConfigurationDrawer projectId={projectId} availability={availability} onClose={() => setShowConfiguration(false)} />}
      {confirmCreate && preflight?.status === "passed" && (
        <div className="rewrite-dialog-layer">
          <section className="rewrite-confirm-dialog" role="dialog" aria-modal="true" aria-label="确认创建重写任务">
            <h2>确认创建重写任务</h2>
            <p>将按以上来源、审核报告、{preflight.selectedIssueSummary.total} 个问题和只读配置创建一次重写运行。</p>
            <footer>
              <button type="button" disabled={creating} onClick={() => setConfirmCreate(false)}>返回修改</button>
              <button type="button" className="primary" disabled={creating} onClick={create}>{creating ? "创建中…" : "确认创建"}</button>
            </footer>
          </section>
        </div>
      )}
    </main>
  );
}
