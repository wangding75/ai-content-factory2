"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { ApiError } from "@/lib/api";
import { newIdempotencyKey } from "@/features/workflow-bindings/workflow-binding-api";
import { cancelWorkflowRun, formatWorkflowRunJson, formatWorkflowRunTime, getWorkflowRun, getWorkflowRunRetryOptions, listWorkflowRunEvents, retryWorkflowRun, type WorkflowRunDetailVm, type WorkflowRunEventVm, type WorkflowRunRetryMode, type WorkflowRunRetryOptions } from "./workflow-run-api";

const apiError = (cause: unknown) => cause instanceof ApiError ? cause : new ApiError("暂时无法获取运行详情。", 0);

function JsonBlock({ title, value }: { title: string; value: unknown }) {
  return <section className="workflow-run-detail-card"><h2>{title}</h2><pre>{formatWorkflowRunJson(value)}</pre></section>;
}

function Timeline({ events, error, retry }: { events: WorkflowRunEventVm[] | null; error: boolean; retry: () => void }) {
  if (error) return <section className="workflow-run-detail-card"><h2>事件时间线</h2><p>暂时无法获取事件记录。</p><button type="button" onClick={retry}>重新加载事件</button></section>;
  if (!events?.length) return <section className="workflow-run-detail-card"><h2>事件时间线</h2><p>暂无事件记录。</p></section>;
  return <section className="workflow-run-detail-card"><h2>事件时间线</h2><ol className="workflow-run-timeline">{events.map((event) => <li key={event.id}><div><strong>{event.title}</strong><span>{event.statusLabel} · {event.createdAtLabel}</span></div>{event.payload && <pre>{formatWorkflowRunJson(event.payload)}</pre>}</li>)}</ol></section>;
}

function Confirm({ action, busy, retryMode, retryOptions, onCancel, onConfirm, onRetryModeChange }: { action: "cancel" | "retry"; busy: boolean; retryMode: WorkflowRunRetryMode; retryOptions: WorkflowRunRetryOptions | null; onCancel: () => void; onConfirm: () => void; onRetryModeChange: (mode: WorkflowRunRetryMode) => void }) {
  const retry = action === "retry";
  const choices = retryOptions ? [retryOptions.currentConfiguration, retryOptions.originalConfiguration] : [];
  const selected = choices.find((option) => option.mode === retryMode);
  return <div className="workflow-run-modal" role="dialog" aria-modal="true" aria-label={retry ? "确认重试" : "确认取消"}><div><h2>{retry ? "确认重试运行" : "确认取消运行"}</h2><p>{retry ? "请选择服务器确认可用的重试方式。" : "取消后运行将停止，且无法恢复为执行中。"}</p>{retry && choices.map((option) => <label key={option.mode}><input type="radio" name="retry-mode" checked={retryMode === option.mode} disabled={busy || !option.enabled} onChange={() => onRetryModeChange(option.mode)} />{option.mode === "current_configuration" ? "使用当前配置" : "使用原配置"}{!option.enabled && <span>：{option.reasons.map((reason) => reason.message).join(" ")}</span>}</label>)}<footer><button type="button" disabled={busy} onClick={onCancel}>返回</button><button type="button" disabled={busy || (retry && !selected?.enabled)} onClick={onConfirm}>{busy ? "提交中…" : retry ? "确认重试" : "确认取消"}</button></footer></div></div>;
}

const asRecord = (value: unknown): Record<string, unknown> => value && typeof value === "object" && !Array.isArray(value) ? value as Record<string, unknown> : {};
const text = (value: unknown, fallback = "—") => typeof value === "string" || typeof value === "number" ? String(value) : fallback;
const redactSuccessValue = (value: unknown): unknown => {
  if (Array.isArray(value)) return value.map(redactSuccessValue);
  if (value && typeof value === "object") return Object.fromEntries(Object.entries(value).map(([key, item]) => /token|secret|password|authorization|api.?key/i.test(key) ? [key, "[已隐藏]"] : [key, redactSuccessValue(item)]));
  return value;
};
const formatSuccessPayload = (value: unknown) => {
  const safe = redactSuccessValue(value);
  return safe && typeof safe === "object" && Object.keys(safe).length ? JSON.stringify(safe, null, 2) : "暂无信息";
};

function SuccessRunDetail({ run, events }: { run: WorkflowRunDetailVm; events: WorkflowRunEventVm[] | null }) {
  const configuration = asRecord(run.configurationSnapshot);
  const workflow = asRecord(configuration.workflowConfiguration);
  const connection = asRecord(configuration.workflowConnection);
  const policy = asRecord(run.llmPolicySnapshot);
  const finishedAtLabel = formatWorkflowRunTime(run.finishedAt);
  const timeline = [
    ["已创建", run.createdAtLabel],
    ["开始执行", run.startedAtLabel],
    ["运行完成", finishedAtLabel]
  ].filter(([, value]) => value !== "—");

  return <main className="workflow-run-detail-main"><div className="workflow-run-detail-canvas workflow-run-success-detail">
    <Link className="workflow-run-back" href="/workflow-runs">← 返回运行记录</Link>
    <header><div><p>流程中心 / 运行记录</p><h1>运行详情 <i className="workflow-runs-status succeeded">{run.statusLabel}</i></h1><span>{run.stageLabel} · {run.triggerSourceLabel}</span></div></header>
    <section className="workflow-run-success-summary" aria-label="运行摘要">
      <div><span>Run ID</span><strong>{run.runNumber}</strong></div>
      <div><span>业务环节</span><strong>{run.stageLabel}</strong></div>
      <div><span>开始时间</span><strong>{run.startedAtLabel}</strong></div>
      <div><span>结束时间</span><strong>{finishedAtLabel}</strong></div>
      <div><span>耗时</span><strong>{run.durationLabel}</strong></div>
      <div><span>重试关系</span>{run.retryOfRunId ? <Link href={`/workflow-runs/${run.retryOfRunId}`}>重试自 {run.retryOfRunId}</Link> : <strong>首次执行</strong>}</div>
    </section>
    <div className="workflow-run-success-layout">
      <div>
        <section className="workflow-run-detail-card"><h2>状态时间线</h2><ol className="workflow-run-success-timeline">{timeline.map(([label, value]) => <li key={label}><strong>{label}</strong><span>{value}</span></li>)}</ol>{events?.length ? <p className="workflow-run-success-events">已记录 {events.length} 条真实运行事件。</p> : <p className="workflow-run-success-events">暂无额外运行事件。</p>}</section>
        <section className="workflow-run-detail-card"><h2>输入与结果摘要</h2><div className="workflow-run-success-data"><div><h3>输入参数</h3><pre>{formatSuccessPayload(run.inputPayload)}</pre></div><div><h3>结果数据</h3>{run.outputPayload ? <pre>{formatSuccessPayload(run.outputPayload)}</pre> : <p>运行已成功，但服务端未提供结果数据摘要。</p>}</div></div></section>
        <section className="workflow-run-detail-card workflow-run-success-result"><h2>结果摘要</h2><p>Runtime 已成功完成。领域结果以服务端返回的领域影响为准，不会仅根据成功状态推断候选、报告或版本已被消费。</p><Link className="workflow-run-primary" href="/workflow-runs">返回运行记录</Link></section>
      </div>
      <aside className="workflow-run-success-aside">
        <section className="workflow-run-detail-card"><h2>配置快照</h2><dl><div><dt>Connection</dt><dd>{text(connection.name, text(run.connectionName))}</dd></div><div><dt>Workflow</dt><dd>{text(workflow.name, text(run.workflowName))} {workflow.version ? `v${text(workflow.version)}` : run.workflowVersionLabel}</dd></div><div><dt>Provider</dt><dd>{text(policy.providerName, "不使用模型")}</dd></div><div><dt>Model</dt><dd>{text(policy.model, run.modelStrategyLabel)}</dd></div><div><dt>绑定版本</dt><dd>{text(run.bindingSnapshot?.bindingVersion)}</dd></div></dl></section>
        {run.externalExecutionId && <section className="workflow-run-detail-card"><h2>外部执行信息</h2><dl><div><dt>外部执行 ID</dt><dd>{run.externalExecutionId}</dd></div></dl></section>}
        <section className="workflow-run-detail-card"><h2>领域影响</h2>{run.domainImpact.length ? <ul className="workflow-run-domain-impact">{run.domainImpact.map((impact, index) => <li key={`${impact.resourceType}-${impact.resourceId ?? index}`}><strong>{impact.resourceType}</strong><span>{impact.resourceId ?? "—"}</span><em>{impact.outcome}</em></li>)}</ul> : <p className="workflow-run-safe-empty">服务端未返回领域影响记录；这不表示已创建或消费任何领域结果。</p>}</section>
        <section className="workflow-run-detail-card"><h2>运行元数据</h2><dl><div><dt>创建时间</dt><dd>{run.createdAtLabel}</dd></div><div><dt>更新时间</dt><dd>{run.updatedAtLabel}</dd></div><div><dt>当前版本</dt><dd>v{run.version}</dd></div></dl></section>
      </aside>
    </div>
  </div></main>;
}

export function WorkflowRunDetailPage({ runId }: { runId: string }) {
  const [run, setRun] = useState<WorkflowRunDetailVm | null>(null);
  const [events, setEvents] = useState<WorkflowRunEventVm[] | null>(null);
  const [retryOptions, setRetryOptions] = useState<WorkflowRunRetryOptions | null>(null);
  const [retryMode, setRetryMode] = useState<WorkflowRunRetryMode>("current_configuration");
  const [error, setError] = useState<ApiError | null>(null);
  const [eventsError, setEventsError] = useState(false);
  const [action, setAction] = useState<"cancel" | "retry" | null>(null);
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState("");

  const load = useCallback(async (signal?: AbortSignal) => {
    setError(null);
    try {
      const detail = await getWorkflowRun(runId, { signal });
      if (!signal?.aborted) setRun(detail);
      try {
        const options = await getWorkflowRunRetryOptions(runId, { signal });
        if (!signal?.aborted) {
          setRetryOptions(options);
          setRetryMode(options.currentConfiguration.enabled ? "current_configuration" : "original_configuration");
        }
      } catch { if (!signal?.aborted) setRetryOptions(null); }
      try {
        const list = await listWorkflowRunEvents(runId, { signal });
        if (!signal?.aborted) { setEvents(list); setEventsError(false); }
      } catch { if (!signal?.aborted) setEventsError(true); }
    } catch (cause) { if (!signal?.aborted) setError(apiError(cause)); }
  }, [runId]);

  useEffect(() => {
    const controller = new AbortController();
    const timer = window.setTimeout(() => void load(controller.signal), 0);
    return () => { window.clearTimeout(timer); controller.abort(); };
  }, [load]);

  const doAction = async () => {
    if (!run || !action) return;
    setBusy(true); setNotice("");
    try {
      const key = newIdempotencyKey();
      if (action === "cancel") { await cancelWorkflowRun(run.id, run.version, key); setAction(null); await load(); }
      else { const next = await retryWorkflowRun(run.id, run.version, key, retryMode); window.location.assign(`/workflow-runs/${next.id}`); }
    } catch (cause) {
      const item = apiError(cause);
      setNotice(item.status === 409 ? "运行状态已变化，已刷新当前详情。" : "操作未完成，请刷新后重试。");
      setAction(null); await load();
    } finally { setBusy(false); }
  };

  if (error) return <main className="workflow-run-detail-main"><section className="workflow-run-detail-state" role="alert"><h1>无法打开运行详情</h1><p>该运行不存在或暂时无法获取。</p><button type="button" onClick={() => void load()}>重新加载</button><Link href="/workflow-runs">返回运行记录</Link></section></main>;
  if (!run) return <main className="workflow-run-detail-main"><section className="workflow-run-detail-state" role="status"><span /><p>正在加载运行详情…</p></section></main>;
  if (run.status === "succeeded") return <SuccessRunDetail run={run} events={events} />;
  const retryAvailable = Boolean(retryOptions?.currentConfiguration.enabled || retryOptions?.originalConfiguration.enabled);
  return <main className="workflow-run-detail-main"><div className="workflow-run-detail-canvas"><Link className="workflow-run-back" href="/workflow-runs">← 返回运行记录</Link><header><div><p>流程中心 / 运行记录</p><h1>运行详情 <i className={`workflow-runs-status ${run.status}`}>{run.statusLabel}</i></h1><span>{run.stageLabel} · {run.triggerSourceLabel}</span></div><div>{run.canCancel && <button type="button" className="workflow-run-danger" onClick={() => setAction("cancel")}>取消运行</button>}{run.canRetry && retryAvailable && <button type="button" className="workflow-run-primary" onClick={() => setAction("retry")}>重新运行</button>}</div></header>{notice && <p className="workflow-run-notice" role="alert">{notice}</p>}<section className="workflow-run-detail-card"><h2>基本信息</h2><dl><div><dt>运行编号</dt><dd>{run.runNumber}</dd></div><div><dt>业务环节</dt><dd>{run.stageLabel}</dd></div><div><dt>触发来源</dt><dd>{run.triggerSourceLabel}</dd></div><div><dt>当前版本</dt><dd>v{run.version}</dd></div><div><dt>创建时间</dt><dd>{run.createdAtLabel}</dd></div><div><dt>更新时间</dt><dd>{run.updatedAtLabel}</dd></div></dl></section><div className="workflow-run-detail-grid"><JsonBlock title="输入参数" value={run.inputPayload} /><JsonBlock title="输出结果" value={run.outputPayload} /><JsonBlock title="安全错误信息" value={{ 错误编号: run.errorCode ?? "暂无", 错误说明: run.errorMessage ?? "暂无", 详情: run.errorDetails ?? {} }} /><JsonBlock title="配置快照" value={run.configurationSnapshot} /></div><Timeline events={events} error={eventsError} retry={() => void listWorkflowRunEvents(runId).then(setEvents).then(() => setEventsError(false)).catch(() => setEventsError(true))} /></div>{action && <Confirm action={action} busy={busy} retryMode={retryMode} retryOptions={retryOptions} onCancel={() => setAction(null)} onConfirm={() => void doAction()} onRetryModeChange={setRetryMode} />}</main>;
}
