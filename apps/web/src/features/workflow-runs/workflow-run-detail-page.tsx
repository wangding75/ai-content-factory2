"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { ApiError } from "@/lib/api";
import { newIdempotencyKey } from "@/features/workflow-bindings/workflow-binding-api";
import {
  cancelWorkflowRun,
  formatWorkflowRunTime,
  getWorkflowRun,
  getWorkflowRunRetryOptions,
  listWorkflowRunEvents,
  retryWorkflowRun,
  retryWorkflowRunResultConsumption,
  type WorkflowRunDetailVm,
  type WorkflowRunEventVm,
  type WorkflowRunRetryMode,
  type WorkflowRunRetryOptions,
} from "./workflow-run-api";

type RunAction = "cancel" | "retry" | "retry_result";

const apiError = (cause: unknown) => cause instanceof ApiError ? cause : new ApiError("暂时无法获取运行详情。", 0);
const asRecord = (value: unknown): Record<string, unknown> => value && typeof value === "object" && !Array.isArray(value) ? value as Record<string, unknown> : {};
const text = (value: unknown, fallback = "—") => typeof value === "string" || typeof value === "number" ? String(value) : fallback;

const redactValue = (value: unknown): unknown => {
  if (Array.isArray(value)) return value.map(redactValue);
  if (value && typeof value === "object") {
    return Object.fromEntries(Object.entries(value).map(([key, item]) => /token|secret|password|authorization|api.?key/i.test(key) ? [key, "[已隐藏]"] : [key, redactValue(item)]));
  }
  return value;
};

const formatPayload = (value: unknown) => {
  const safe = redactValue(value);
  return safe && typeof safe === "object" && Object.keys(safe).length ? JSON.stringify(safe, null, 2) : "暂无信息";
};

function JsonBlock({ title, value }: { title: string; value: unknown }) {
  return <section className="workflow-run-detail-card"><h2>{title}</h2><pre>{formatPayload(value)}</pre></section>;
}

function Timeline({ events, error, retry }: { events: WorkflowRunEventVm[] | null; error: boolean; retry: () => void }) {
  if (error) return <section className="workflow-run-detail-card"><h2>事件时间线</h2><p>暂时无法获取事件记录。</p><button type="button" onClick={retry}>重新加载事件</button></section>;
  if (!events?.length) return <section className="workflow-run-detail-card"><h2>事件时间线</h2><p>暂无事件记录。</p></section>;
  return <section className="workflow-run-detail-card"><h2>事件时间线</h2><ol className="workflow-run-timeline">{events.map((event) => <li key={event.id}><div><strong>{event.title}</strong><span>{event.statusLabel} · {event.createdAtLabel}</span></div>{event.payload && <pre>{formatPayload(event.payload)}</pre>}</li>)}</ol></section>;
}

function RetryOption({ option, selected, busy, onSelect }: { option: WorkflowRunRetryOptions["currentConfiguration"] | WorkflowRunRetryOptions["originalConfiguration"]; selected: boolean; busy: boolean; onSelect: () => void }) {
  const differences = option.configurationDifferences ?? [];
  return <label className={`workflow-run-retry-option${selected ? " selected" : ""}${!option.enabled ? " disabled" : ""}`}><input type="radio" name="retry-mode" checked={selected} disabled={busy || !option.enabled} onChange={onSelect} /><div><div className="workflow-run-retry-option-heading"><strong>{option.mode === "current_configuration" ? "使用当前配置重试" : "使用原配置重试"}</strong><span className={option.enabled ? "enabled" : "disabled"}>{option.enabled ? "可重试" : "不可用"}</span></div><p>{option.enabled ? "服务端已确认该方式可用。" : option.reasons.length ? option.reasons.map((reason) => reason.message).join("；") : "服务端未确认该方式可用。"}</p>{differences.length > 0 ? <ul>{differences.map((difference) => <li key={difference.field}><span>{difference.field}</span>{difference.summary}</li>)}</ul> : <small>服务端未返回配置差异摘要。</small>}</div></label>;
}

function Confirm({ action, busy, retryMode, retryOptions, retryReason, onCancel, onConfirm, onRetryModeChange, onRetryReasonChange }: { action: RunAction; busy: boolean; retryMode: WorkflowRunRetryMode; retryOptions: WorkflowRunRetryOptions | null; retryReason: string; onCancel: () => void; onConfirm: () => void; onRetryModeChange: (mode: WorkflowRunRetryMode) => void; onRetryReasonChange: (reason: string) => void }) {
  const cancel = action === "cancel";
  const runtimeRetry = action === "retry";
  const resultRetry = action === "retry_result";
  const choices = runtimeRetry && retryOptions ? [retryOptions.currentConfiguration, retryOptions.originalConfiguration] : [];
  const selected = choices.find((option) => option.mode === retryMode);
  const title = cancel ? "确认取消运行" : resultRetry ? "确认重试结果消费" : "确认重试运行";
  const description = cancel ? "取消后运行将停止，且无法恢复为执行中。" : resultRetry ? "该操作只会重新消费已返回的结果，不会再次调用外部工作流。" : "请选择服务器确认可用的重试方式。";
  return <div className="workflow-run-modal" role="dialog" aria-modal="true" aria-label={title}><div className="workflow-run-retry-dialog"><header><div><h2>{title}</h2><p>运行 {retryOptions?.runId ?? "当前运行"}</p></div><button type="button" aria-label="关闭" disabled={busy} onClick={onCancel}>×</button></header><section><p className="workflow-run-retry-dialog-note">{description}</p>{runtimeRetry && choices.map((option) => <RetryOption key={option.mode} option={option} selected={retryMode === option.mode} busy={busy} onSelect={() => onRetryModeChange(option.mode)} />)}{runtimeRetry && <label className="workflow-run-retry-reason">重试原因 <span>（选填）</span><textarea value={retryReason} onChange={(event) => onRetryReasonChange(event.target.value)} placeholder="说明修复或重试原因" rows={2} /></label>}</section><footer><button type="button" disabled={busy} onClick={onCancel}>取消</button><button type="button" disabled={busy || (runtimeRetry && !selected?.enabled)} onClick={onConfirm}>{busy ? "提交中…" : cancel ? "确认取消" : resultRetry ? "确认重试消费" : "确认重试"}</button></footer></div></div>;
}

function SuccessRunDetail({ run, events }: { run: WorkflowRunDetailVm; events: WorkflowRunEventVm[] | null }) {
  const configuration = asRecord(run.configurationSnapshot);
  const workflow = asRecord(configuration.workflowConfiguration);
  const connection = asRecord(configuration.workflowConnection);
  const policy = asRecord(run.llmPolicySnapshot);
  const finishedAtLabel = formatWorkflowRunTime(run.finishedAt);
  const timeline = [["已创建", run.createdAtLabel], ["开始执行", run.startedAtLabel], ["运行完成", finishedAtLabel]].filter(([, value]) => value !== "—");

  return <main className="workflow-run-detail-main"><div className="workflow-run-detail-canvas workflow-run-success-detail">
    <Link className="workflow-run-back" href="/workflow-runs">← 返回运行记录</Link>
    <header><div><p>流程中心 / 运行记录</p><h1>运行详情 <i className="workflow-runs-status succeeded">{run.statusLabel}</i></h1><span>{run.stageLabel} · {run.triggerSourceLabel}</span></div></header>
    <section className="workflow-run-success-summary" aria-label="运行摘要">
      <div><span>Run ID</span><strong>{run.id}</strong></div><div><span>运行编号</span><strong>{run.runNumber}</strong></div><div><span>业务环节</span><strong>{run.stageLabel}</strong></div><div><span>开始时间</span><strong>{run.startedAtLabel}</strong></div><div><span>结束时间</span><strong>{finishedAtLabel}</strong></div><div><span>耗时</span><strong>{run.durationLabel}</strong></div>
    </section>
    <div className="workflow-run-success-layout"><div>
      <section className="workflow-run-detail-card"><h2>状态时间线</h2><ol className="workflow-run-success-timeline">{timeline.map(([label, value]) => <li key={label}><strong>{label}</strong><span>{value}</span></li>)}</ol><p className="workflow-run-success-events">{events?.length ? `已记录 ${events.length} 条真实运行事件。` : "暂无额外运行事件。"}</p></section>
      <section className="workflow-run-detail-card"><h2>输入与结果摘要</h2><div className="workflow-run-success-data"><div><h3>输入参数</h3><pre>{formatPayload(run.inputPayload)}</pre></div><div><h3>结果数据</h3>{run.outputPayload ? <pre>{formatPayload(run.outputPayload)}</pre> : <p>运行已成功，但服务端未提供结果数据摘要。</p>}</div></div></section>
      <section className="workflow-run-detail-card workflow-run-success-result"><h2>结果摘要</h2><p>Runtime 已成功完成。领域结果以服务端返回的领域影响为准，不会仅根据成功状态推断候选、报告或版本已被消费。</p><Link className="workflow-run-primary" href="/workflow-runs">返回运行记录</Link></section>
    </div><aside className="workflow-run-success-aside">
      <section className="workflow-run-detail-card"><h2>配置快照</h2><dl><div><dt>Connection</dt><dd>{text(connection.name, text(run.connectionName))}</dd></div><div><dt>Workflow</dt><dd>{text(workflow.name, text(run.workflowName))} {workflow.version ? `v${text(workflow.version)}` : run.workflowVersionLabel}</dd></div><div><dt>Provider</dt><dd>{text(policy.providerName, "未使用模型")}</dd></div><div><dt>Model</dt><dd>{text(policy.model, run.modelStrategyLabel)}</dd></div><div><dt>绑定版本</dt><dd>{text(run.bindingSnapshot?.bindingVersion)}</dd></div></dl></section>
      {run.externalExecutionId && <section className="workflow-run-detail-card"><h2>外部执行信息</h2><dl><div><dt>外部执行 ID</dt><dd>{run.externalExecutionId}</dd></div></dl></section>}
      {run.domainImpact.length > 0 && <section className="workflow-run-detail-card"><h2>领域影响</h2><ul className="workflow-run-domain-impact">{run.domainImpact.map((impact, index) => <li key={`${impact.resourceType}-${impact.resourceId ?? index}`}><strong>{impact.resourceType}</strong><span>{impact.resourceId ?? "—"}</span><em>{impact.outcome}</em></li>)}</ul></section>}
      <section className="workflow-run-detail-card"><h2>运行元数据</h2><dl><div><dt>创建时间</dt><dd>{run.createdAtLabel}</dd></div><div><dt>更新时间</dt><dd>{run.updatedAtLabel}</dd></div><div><dt>当前版本</dt><dd>v{run.version}</dd></div><div><dt>重试关系</dt><dd>{run.retryOfRunId ? <Link href={`/workflow-runs/${run.retryOfRunId}`}>重试自 {run.retryOfRunId}</Link> : "首次执行"}</dd></div></dl></section>
    </aside></div>
  </div></main>;
}

const failurePhaseLabels: Record<string, string> = { external_execution: "外部执行", output_validation: "输出校验", result_consumption: "结果消费", cancellation: "取消处理" };

function FailureRunDetail({ run, events, eventsError, retryOptions, retryAvailable, notice, onAction, onRetryEvents }: { run: WorkflowRunDetailVm; events: WorkflowRunEventVm[] | null; eventsError: boolean; retryOptions: WorkflowRunRetryOptions | null; retryAvailable: boolean; notice: string; onAction: (action: RunAction) => void; onRetryEvents: () => void }) {
  const failureTitle = run.displayStatus === "output_validation_failed" ? "输出校验失败" : run.displayStatus === "result_consumption_failed" ? "结果消费失败" : run.status === "timed_out" ? "运行超时" : "运行失败";
  const phaseLabel = run.failurePhase ? failurePhaseLabels[run.failurePhase] ?? run.failurePhase : "—";
  const errorCode = run.failureCode ?? run.errorCode ?? "—";
  const errorMessage = run.safeError?.message ?? run.errorMessage ?? "服务端未提供安全错误摘要。";
  const runtimeRetry = run.canRetry && run.retryability === "runtime_retry";
  const resultRetry = run.canRetryResultConsumption || run.retryability === "result_consumption_retry";
  const retryReasons = retryOptions ? [...retryOptions.currentConfiguration.reasons, ...retryOptions.originalConfiguration.reasons].map((reason) => reason.message).filter((message, index, list) => list.indexOf(message) === index) : [];
  const configuration = asRecord(run.configurationSnapshot);
  const workflow = asRecord(configuration.workflowConfiguration);
  const connection = asRecord(configuration.workflowConnection);
  const policy = asRecord(run.llmPolicySnapshot);

  return <main className="workflow-run-detail-main"><div className="workflow-run-detail-canvas workflow-run-failure-detail">
    <Link className="workflow-run-back" href="/workflow-runs">← 返回运行记录</Link>
    <header><div><p>流程中心 / 运行记录</p><h1>运行详情 <i className="workflow-runs-status failed">{run.statusLabel}</i></h1><span>{run.stageLabel} · {run.triggerSourceLabel}</span></div><div className="workflow-run-detail-actions">{runtimeRetry && <button type="button" className="workflow-run-primary" disabled={!retryAvailable} onClick={() => onAction("retry")}>重试运行</button>}{resultRetry && <button type="button" className="workflow-run-primary" onClick={() => onAction("retry_result")}>重试结果消费</button>}</div></header>
    {notice && <p className="workflow-run-notice" role="alert">{notice}</p>}
    <div className="workflow-run-failure-layout"><div>
      <section className="workflow-run-detail-card workflow-run-failure-summary"><div className="workflow-run-failure-heading"><span aria-hidden="true">!</span><div><h2>{failureTitle}</h2><p>{errorMessage}</p></div></div><dl><div><dt>失败阶段</dt><dd>{phaseLabel}</dd></div><div><dt>错误编号</dt><dd className="workflow-run-error-code">{errorCode}</dd></div><div><dt>处理建议</dt><dd>{resultRetry ? "仅重新消费服务端已返回的结果。" : runtimeRetry ? "检查配置或连接后再重试运行。" : "当前状态没有可用的恢复操作。"}</dd></div></dl><details><summary>安全错误详情</summary><pre>{formatPayload({ safeError: run.safeError, errorDetails: run.errorDetails })}</pre></details><div className="workflow-run-recovery">{runtimeRetry && !retryAvailable && <p>服务端未确认可安全重试：{retryReasons.length ? retryReasons.join("；") : "暂无可用的重试配置。"}</p>}{runtimeRetry && retryAvailable && <button type="button" className="workflow-run-primary" onClick={() => onAction("retry")}>选择重试配置</button>}{resultRetry && <button type="button" className="workflow-run-primary" onClick={() => onAction("retry_result")}>重新消费结果</button>}</div></section>
      <section className="workflow-run-detail-card"><h2>基本信息</h2><dl><div><dt>Run ID</dt><dd>{run.id}</dd></div><div><dt>运行编号</dt><dd>{run.runNumber}</dd></div><div><dt>所属项目</dt><dd>{run.projectId}</dd></div><div><dt>业务环节</dt><dd>{run.stageLabel}</dd></div><div><dt>工作流</dt><dd>{run.workflowName} {run.workflowVersionLabel}</dd></div><div><dt>触发来源</dt><dd>{run.triggerSourceLabel}</dd></div><div><dt>开始时间</dt><dd>{run.startedAtLabel}</dd></div><div><dt>结束时间</dt><dd>{formatWorkflowRunTime(run.finishedAt)}</dd></div><div><dt>总耗时</dt><dd>{run.durationLabel}</dd></div><div><dt>当前版本</dt><dd>v{run.version}</dd></div></dl></section>
      <JsonBlock title="输入参数" value={run.inputPayload} />
      {run.outputPayload && <JsonBlock title="外部输出（未形成领域结果）" value={run.outputPayload} />}
      <Timeline events={events} error={eventsError} retry={onRetryEvents} />
    </div><aside className="workflow-run-failure-aside">
      <section className="workflow-run-detail-card"><h2>运行摘要</h2><dl><div><dt>最终状态</dt><dd className="workflow-run-error-code">{run.statusLabel}</dd></div><div><dt>失败阶段</dt><dd>{phaseLabel}</dd></div><div><dt>可重试性</dt><dd>{resultRetry ? "结果消费重试" : runtimeRetry ? "Runtime 重试" : "不可重试"}</dd></div><div><dt>重试关系</dt><dd>{run.retryOfRunId ? <Link href={`/workflow-runs/${run.retryOfRunId}`}>重试自 {run.retryOfRunId}</Link> : "首次执行"}</dd></div></dl></section>
      <section className="workflow-run-detail-card"><h2>配置快照</h2><dl><div><dt>Connection</dt><dd>{text(connection.name, text(run.connectionName))}</dd></div><div><dt>Workflow</dt><dd>{text(workflow.name, text(run.workflowName))} {workflow.version ? `v${text(workflow.version)}` : run.workflowVersionLabel}</dd></div><div><dt>Provider</dt><dd>{text(policy.providerName, "未使用模型")}</dd></div><div><dt>Model</dt><dd>{text(policy.model, run.modelStrategyLabel)}</dd></div><div><dt>绑定版本</dt><dd>{text(run.bindingSnapshot?.bindingVersion)}</dd></div></dl></section>
      {run.externalExecutionId && <section className="workflow-run-detail-card"><h2>外部执行信息</h2><dl><div><dt>外部执行 ID</dt><dd>{run.externalExecutionId}</dd></div></dl></section>}
      {run.domainImpact.length > 0 && <section className="workflow-run-detail-card"><h2>领域影响</h2><ul className="workflow-run-domain-impact">{run.domainImpact.map((impact, index) => <li key={`${impact.resourceType}-${impact.resourceId ?? index}`}><strong>{impact.resourceType}</strong><span>{impact.resourceId ?? "—"}</span><em>{impact.outcome}</em></li>)}</ul></section>}
    </aside></div>
  </div></main>;
}

export function WorkflowRunDetailPage({ runId }: { runId: string }) {
  const [run, setRun] = useState<WorkflowRunDetailVm | null>(null);
  const [events, setEvents] = useState<WorkflowRunEventVm[] | null>(null);
  const [retryOptions, setRetryOptions] = useState<WorkflowRunRetryOptions | null>(null);
  const [retryMode, setRetryMode] = useState<WorkflowRunRetryMode>("current_configuration");
  const [error, setError] = useState<ApiError | null>(null);
  const [eventsError, setEventsError] = useState(false);
  const [action, setAction] = useState<RunAction | null>(null);
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState("");
  const [retryReason, setRetryReason] = useState("");

  const load = useCallback(async (signal?: AbortSignal) => {
    setError(null);
    try {
      const detail = await getWorkflowRun(runId, { signal });
      if (!signal?.aborted) setRun(detail);
      try {
        const options = await getWorkflowRunRetryOptions(runId, { signal });
        if (!signal?.aborted) { setRetryOptions(options); setRetryMode(options.currentConfiguration.enabled ? "current_configuration" : "original_configuration"); }
      } catch { if (!signal?.aborted) setRetryOptions(null); }
      try {
        const list = await listWorkflowRunEvents(runId, { signal });
        if (!signal?.aborted) { setEvents(list); setEventsError(false); }
      } catch { if (!signal?.aborted) setEventsError(true); }
    } catch (cause) { if (!signal?.aborted) setError(apiError(cause)); }
  }, [runId]);

  useEffect(() => { const controller = new AbortController(); const timer = window.setTimeout(() => void load(controller.signal), 0); return () => { window.clearTimeout(timer); controller.abort(); }; }, [load]);

  const doAction = async () => {
    if (!run || !action) return;
    setBusy(true); setNotice("");
    try {
      const key = newIdempotencyKey();
      if (action === "cancel") { await cancelWorkflowRun(run.id, run.version, key); setAction(null); await load(); }
      else if (action === "retry_result") { await retryWorkflowRunResultConsumption(run, key); setAction(null); await load(); }
      else { const next = await retryWorkflowRun(run.id, run.version, key, retryMode, undefined, retryReason.trim() || undefined); window.location.assign(`/workflow-runs/${next.id}`); }
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
  const isFailure = run.status === "failed" || run.status === "timed_out" || run.displayStatus === "output_validation_failed" || run.displayStatus === "result_consumption_failed";
  if (isFailure) return <><FailureRunDetail run={run} events={events} eventsError={eventsError} retryOptions={retryOptions} retryAvailable={retryAvailable} notice={notice} onAction={(nextAction) => { setRetryReason(""); setAction(nextAction); }} onRetryEvents={() => void listWorkflowRunEvents(runId).then(setEvents).then(() => setEventsError(false)).catch(() => setEventsError(true))} />{action && <Confirm action={action} busy={busy} retryMode={retryMode} retryOptions={retryOptions} retryReason={retryReason} onCancel={() => setAction(null)} onConfirm={() => void doAction()} onRetryModeChange={setRetryMode} onRetryReasonChange={setRetryReason} />}</>;
  return <main className="workflow-run-detail-main"><div className="workflow-run-detail-canvas"><Link className="workflow-run-back" href="/workflow-runs">← 返回运行记录</Link><header><div><p>流程中心 / 运行记录</p><h1>运行详情 <i className={`workflow-runs-status ${run.status}`}>{run.statusLabel}</i></h1><span>{run.stageLabel} · {run.triggerSourceLabel}</span></div><div>{run.canCancel && <button type="button" className="workflow-run-danger" onClick={() => setAction("cancel")}>取消运行</button>}</div></header>{notice && <p className="workflow-run-notice" role="alert">{notice}</p>}<section className="workflow-run-detail-card"><h2>基本信息</h2><dl><div><dt>运行编号</dt><dd>{run.runNumber}</dd></div><div><dt>业务环节</dt><dd>{run.stageLabel}</dd></div><div><dt>触发来源</dt><dd>{run.triggerSourceLabel}</dd></div><div><dt>当前版本</dt><dd>v{run.version}</dd></div><div><dt>创建时间</dt><dd>{run.createdAtLabel}</dd></div><div><dt>更新时间</dt><dd>{run.updatedAtLabel}</dd></div></dl></section><div className="workflow-run-detail-grid"><JsonBlock title="输入参数" value={run.inputPayload} /><JsonBlock title="输出结果" value={run.outputPayload} /><JsonBlock title="安全错误信息" value={{ errorCode: run.errorCode ?? "暂无", errorMessage: run.errorMessage ?? "暂无", details: run.errorDetails ?? {} }} /><JsonBlock title="配置快照" value={run.configurationSnapshot} /></div><Timeline events={events} error={eventsError} retry={() => void listWorkflowRunEvents(runId).then(setEvents).then(() => setEventsError(false)).catch(() => setEventsError(true))} /></div>{action && <Confirm action={action} busy={busy} retryMode={retryMode} retryOptions={retryOptions} retryReason={retryReason} onCancel={() => setAction(null)} onConfirm={() => void doAction()} onRetryModeChange={setRetryMode} onRetryReasonChange={setRetryReason} />}</main>;
}
