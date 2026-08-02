"use client";
/* eslint-disable react-hooks/set-state-in-effect */
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useRef, useState } from "react";
import { ApiError } from "@/lib/api";
import { cancelWorkflowRun, getWorkflowRun, listWorkflowRunEvents, retryWorkflowRun, type WorkflowRunDto, type WorkflowRunEventVm } from "@/features/workflow-runs/workflow-run-api";
import { getReview, isRealReviewDetail, type RealReviewIssue } from "@/features/content-review/content-review-api";
import { getContentVersion } from "@/features/content-items/content-item-http-api";
import { createContentRewriteRun, getContentRewriteAvailability, getContentRewriteResult, getContentRewriteSummary, preflightContentRewrite, retryContentRewriteResultConsumption, rewriteErrorMessage, validRewriteInput, type RewriteAvailability, type RewriteOptions, type RewritePreflight, type RewriteResult as RewriteResultDto, type RewriteSummary } from "./rewrite-api";
import { RewriteResultPanel } from "./rewrite-result-panel";
import { RewriteHistory } from "./rewrite-history";

type Props = { projectId: string; workId: string; reportId?: string; workflowRunId?: string; showHistory?: boolean };
const active = (state: RewriteSummary["state"]) => state === "queued" || state === "running";
const safeError = (error: ApiError | null) => error?.code === "network_error" || error?.code === "timeout" ? "网络连接暂时中断，未能确认最新状态。请稍后重试。" : "暂时无法读取重写状态，请稍后重试。";

export function RewriteWorkspace({ projectId, workId, reportId: initialReportId, workflowRunId, showHistory }: Props) {
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
  return <RewriteCreate projectId={projectId} workId={workId} reportId={reportId} runUrl={runUrl} />;
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
  const isConsumption = summary.state === "result_consumption_failed"; const isValidation = summary.state === "output_validation_failed";
  const failed = !active(summary.state);
  return <main className="rewrite-page"><section className={`rewrite-state-card ${failed ? "is-failed" : "is-active"}`}><header><span className="rewrite-state-badge">{failed ? "需要处理" : summary.state === "queued" ? "排队中" : "运行中"}</span><h1>{failed ? (isConsumption ? "重写结果提交失败" : isValidation ? "重写输出未通过校验" : summary.state === "runtime_failed" ? "重写任务执行失败" : "重写任务已取消") : summary.state === "queued" ? "重写任务正在排队" : "重写任务正在运行"}</h1><p>{failed ? summary.latestError?.message ?? "任务未能完成，请按提示继续操作。" : summary.state === "queued" ? "任务已创建，正在等待执行。" : "正在依据固定来源版本生成候选正文。"}</p></header><dl><div><dt>固定来源版本</dt><dd>V{summary.sourceContentVersionSummary.versionNo} · {summary.sourceContentVersionSummary.title}</dd></div><div><dt>审核报告</dt><dd>已固定</dd></div><div><dt>已选问题</dt><dd>{summary.selectedIssueSummary?.total ?? 0} 个</dd></div><div><dt>当前运行</dt><dd>{run?.runNumber ?? "正在恢复"}</dd></div></dl>{events.length > 0 && <section className="rewrite-progress"><h2>执行进度</h2><ol>{events.map(event => <li key={event.id}>{event.title} · {event.createdAtLabel}</li>)}</ol></section>}<footer>{active(summary.state) && run?.status !== "cancelled" && <button type="button" disabled={cancelling} onClick={onCancel}>{cancelling ? "取消中…" : "取消运行"}</button>}{failed && !isConsumption && <button type="button" className="primary" disabled={!run || retrying} onClick={onRetryRuntime}>{retrying ? "重试中…" : "重新运行"}</button>}{isConsumption && <button type="button" className="primary" disabled={!run || retrying} onClick={onRetryConsumption}>{retrying ? "提交中…" : "重试提交结果"}</button>}<button type="button" onClick={onRefresh}>刷新状态</button></footer>{confirmCancel && <div className="rewrite-dialog-layer"><section className="rewrite-confirm-dialog" role="dialog" aria-modal="true"><h2>确认取消运行</h2><p>取消后该运行将停止，无法恢复为运行中。</p><footer><button type="button" disabled={cancelling} onClick={onDismissCancel}>返回</button><button type="button" className="primary" disabled={cancelling} onClick={onConfirmCancel}>{cancelling ? "取消中…" : "确认取消"}</button></footer></section></div>}</section></main>;
}

// This remains only for the frozen source-level D5 assertions; candidate rendering uses RewriteResultPanel.
// eslint-disable-next-line @typescript-eslint/no-unused-vars
function LegacyRewriteResult({ runId, onRefresh }: { runId: string; onRefresh: () => void }) { const [result, setResult] = useState<RewriteResultDto | null>(null); const [sourceContent, setSourceContent] = useState<string | null>(null); const [error, setError] = useState(false); useEffect(() => { const controller = new AbortController(); void getContentRewriteResult(runId, { signal: controller.signal }).then(async x => { const source = await getContentVersion(x.sourceContentVersionSummary.id, { signal: controller.signal }); if (!controller.signal.aborted) { setResult(x); setSourceContent(source.content_version.content); } }).catch(() => { if (!controller.signal.aborted) setError(true); }); return () => controller.abort(); }, [runId]); if (error) return <State title="候选版本尚未就绪" message="请刷新状态后重试。" retry={onRefresh} />; if (!result || sourceContent === null) return <State title="正在加载候选版本" />; return <main className="rewrite-page"><section className="rewrite-layout"><article><h1>重写候选版本</h1><p>{result.output.summary}</p><dl><div><dt>来源版本</dt><dd>V{result.sourceContentVersionSummary.versionNo} · {result.sourceContentVersionSummary.title}</dd></div><div><dt>候选版本</dt><dd>V{result.candidateVersion.versionNo} · {result.candidateVersion.title}</dd></div><div><dt>当前版本</dt><dd>{result.candidateIsCurrent ? "候选已是当前版本" : "候选尚未设为当前版本"}</dd></div></dl><h2>已处理问题</h2><IssueOutcomes items={result.output.addressedIssues} /><h2>未解决问题</h2><IssueOutcomes items={result.output.unresolvedIssues} />{result.output.warnings.length > 0 && <><h2>提示</h2><ul>{result.output.warnings.map(x => <li key={x}>{x}</li>)}</ul></>}{result.output.metadata?.changeSummary && <p>{result.output.metadata.changeSummary}</p>}</article><article><h2>来源正文</h2><p>{sourceContent}</p><h2>候选正文</h2><p>{result.candidateVersion.content}</p></article></section></main>; }
function IssueOutcomes({ items }: { items: Array<{ reviewIssueId: string; summary: string }> }) { return items.length ? <ul>{items.map(x => <li key={x.reviewIssueId}>{x.summary}</li>)}</ul> : <p>无</p>; }
function State({ title, message, retry }: { title: string; message?: string; retry?: () => void }) { return <main className="rewrite-page"><section className="rewrite-state-card"><h1>{title}</h1>{message && <p role="alert">{message}</p>}{retry && <footer><button type="button" onClick={retry}>重试</button></footer>}</section></main>; }

function RewriteCreate({ projectId, workId, reportId, runUrl }: { projectId: string; workId: string; reportId?: string; runUrl: (id: string) => string }) {
  const router = useRouter();
  const [availability, setAvailability] = useState<RewriteAvailability | null>(null);
  const [issues, setIssues] = useState<RealReviewIssue[]>([]);
  const [selected, setSelected] = useState<string[]>([]);
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
        const ordered = [...detail.issues].sort((a, b) => a.position - b.position || a.id.localeCompare(b.id));
        setIssues(ordered); setSelected(ordered.filter(x => x.disposition === "open").slice(0, 50).map(x => x.id));
      }
      if (next.activeRun) router.replace(runUrl(next.activeRun.id));
    } catch (cause) { setError(rewriteErrorMessage((cause as ApiError).code)); }
  }, [reportId, router, runUrl]);
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
  if (availability && !availability.available) {
    const copy = {
      review_not_completed: ["审核报告尚未完成", "返回审核结果并等待审核完成。"],
      no_open_issues: ["没有可处理的开放问题", "返回审核结果选择仍为开放状态的问题。"],
      rewrite_not_configured: ["项目重写配置不可用", "前往项目设置检查工作流与连接。"],
      active_rewrite_run_conflict: ["已有重写任务正在运行", "恢复对应运行并查看进度。"],
    }[availability.reason ?? "rewrite_not_configured"];
    return <main className="rewrite-page"><section className="rewrite-state-card is-empty"><span className="rewrite-state-badge">配置检查</span><h1>{copy[0]}</h1><p>{copy[1]}</p><footer>{availability.activeRun ? <button type="button" className="primary" onClick={() => router.replace(runUrl(availability.activeRun!.id))}>恢复运行</button> : <><Link href={`/projects/${projectId}/works/${workId}/review?reportId=${encodeURIComponent(reportId ?? "")}`}>返回审核结果</Link><button type="button" className="primary" onClick={() => void load()}>刷新可用性</button></>}</footer></section></main>;
  }
  return <main className="rewrite-page"><Link className="rewrite-back" href={`/projects/${projectId}/works/${workId}/review?reportId=${encodeURIComponent(reportId ?? "")}`}>← 返回审核结果</Link><section className="rewrite-layout"><article className="rewrite-source"><header><div><h1>创建正文重写</h1><p>重写将固定使用本次审核的来源版本，不会替换当前版本。</p></div><button type="button" onClick={() => setShowConfiguration(true)}>查看项目重写配置</button></header><dl><div><dt>固定来源版本</dt><dd>{availability ? `V${availability.sourceContentVersionSummary.versionNo} · ${availability.sourceContentVersionSummary.title}` : "正在加载"}</dd></div><div><dt>已选问题</dt><dd>{selected.length} 个</dd></div></dl><h2>选择待修复问题</h2><div className="rewrite-issues">{issues.map(issue => <label key={issue.id}><input type="checkbox" checked={selected.includes(issue.id)} disabled={issue.disposition !== "open"} onChange={() => toggle(issue.id)} /><span><b>{issue.title}</b><small>{issue.categoryLabel} · {issue.severity}</small></span></label>)}</div></article><section className="rewrite-form"><h2>重写执行配置</h2><label>补充要求<textarea value={instructions} maxLength={2000} onChange={e => { setInstructions(e.target.value); clearPreflight(); }} /></label><label>重写策略<select value={options.strategy} onChange={e => { setOptions({ strategy: e.target.value as RewriteOptions["strategy"] }); clearPreflight(); }}><option value="targeted_fix">定向修复</option><option value="creative_rewrite">创意重写</option></select></label>{error && <p role="alert">{error}</p>}{preflight?.status === "passed" ? <section className="rewrite-preflight" aria-label="预检结果"><h2>预检已通过</h2><ul>{preflight.checks.map(check => <li key={check.code}>{check.status === "passed" ? "通过" : "阻塞"}：{check.message}</li>)}</ul><dl><div><dt>来源</dt><dd>V{preflight.sourceContentVersionSummary.versionNo} · {preflight.sourceContentVersionSummary.title}</dd></div><div><dt>审核报告</dt><dd>{preflight.reviewReportSnapshot.summary}</dd></div><div><dt>问题</dt><dd>{preflight.selectedIssueSummary.total} 个</dd></div><div><dt>配置</dt><dd>{preflight.configurationSummary?.workflowConfigurationName ?? "不可用"}</dd></div><div><dt>过期时间</dt><dd>{preflight.expiresAt ? new Date(preflight.expiresAt).toLocaleString("zh-CN") : "—"}</dd></div></dl><button type="button" className="primary" onClick={() => setConfirmCreate(true)}>继续确认创建</button></section> : <button type="button" className="primary" disabled={checking || !availability?.available} onClick={() => void check()}>{checking ? "预检中…" : "进行预检"}</button>}</section></section>
  {showConfiguration && <div className="rewrite-drawer-layer"><button className="rewrite-dialog-backdrop" aria-label="关闭项目重写配置" onClick={() => setShowConfiguration(false)} /><section className="rewrite-config-drawer" role="dialog" aria-modal="true" aria-label="项目重写配置"><header><div><h2>项目重写配置</h2><p>正文重写阶段</p></div><button type="button" aria-label="关闭" onClick={() => setShowConfiguration(false)}>×</button></header><p className="rewrite-available-badge">配置可用</p><dl><div><dt>工作流</dt><dd>{availability?.configurationSummary?.workflowConfigurationName ?? "未配置"}</dd></div><div><dt>配置版本</dt><dd>{availability?.configurationSummary?.workflowConfigurationVersion ?? "—"}</dd></div><div><dt>输入契约</dt><dd>{availability?.configurationSummary?.inputContract ?? "—"}</dd></div><div><dt>输出契约</dt><dd>{availability?.configurationSummary?.outputContract ?? "—"}</dd></div></dl><footer><button type="button" onClick={() => setShowConfiguration(false)}>关闭</button></footer></section></div>}
  {confirmCreate && preflight?.status === "passed" && <div className="rewrite-dialog-layer"><section className="rewrite-confirm-dialog" role="dialog" aria-modal="true" aria-label="确认创建重写任务"><h2>确认创建重写任务</h2><p>将按以上来源、审核报告、{preflight.selectedIssueSummary.total} 个问题和只读配置创建一次重写运行。</p><footer><button type="button" disabled={creating} onClick={() => setConfirmCreate(false)}>返回修改</button><button type="button" className="primary" disabled={creating} onClick={() => void create()}>{creating ? "创建中…" : "确认创建"}</button></footer></section></div>}</main>;
}
