"use client";
import Link from "next/link";
import { useEffect, useRef, useState } from "react";
import { getOrCreateOperation } from "@/features/chapter-plans/use-idempotency";
import { getWorkflowRunEvents, retryContentGenerationResultConsumption, retryWorkflowRun, type ContentGenerationSummary, type GenerationEvent } from "./content-item-http-api";

const failed = new Set(["runtime_failed", "output_validation_failed", "result_consumption_failed"]);
const safe = (message: string | null | undefined) => message && message.length <= 300 && !/(stack|sql|postgres|https?:\/\/|\\\\)/i.test(message) ? message : "任务未能完成，当前正文已保留。";
const eventLabel = (value: string) => ({ queued: "已进入队列", worker_started: "开始执行", request_sent: "已发送生成请求", response_received: "已收到生成结果", output_validated: "输出已校验", result_consumed: "候选版本已创建", result_consumption_failed: "候选版本创建失败", succeeded: "任务完成", failed: "任务失败", cancelled: "任务已取消", retry_created: "已创建重试任务" })[value] ?? "运行状态已更新";

export function ContentGenerationStatus({ projectId, summary, onRefresh, onCandidate }: { projectId: string; summary: ContentGenerationSummary; onRefresh: () => Promise<void>; onCandidate: () => void }) {
  const [fetchedEvents, setFetchedEvents] = useState<GenerationEvent[] | null>(null);
  const [showEvents, setShowEvents] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const retryKey = useRef<string | null>(null);
  const run = summary.activeRun ?? summary.latestRun;
  useEffect(() => {
    if (!run || !["queued", "running"].includes(summary.state)) return;
    const controller = new AbortController();
    void getWorkflowRunEvents(run.id, { signal: controller.signal }).then((next) => { if (!controller.signal.aborted) setFetchedEvents(next.items); }).catch(() => undefined);
    return () => controller.abort();
  }, [run, summary.state]);
  if (!["queued", "running", "candidate_ready", "not_configured", ...failed].includes(summary.state)) return null;
  const title = summary.state === "queued" ? "排队中" : summary.state === "running" ? "运行中" : summary.state === "candidate_ready" ? "候选版本已创建" : summary.state === "not_configured" ? "尚未配置正文生成工作流" : "正文生成未完成";
  const retry = async () => {
    if (!run || submitting) return;
    setSubmitting(true);
    try {
      const key = retryKey.current ?? await getOrCreateOperation(`content-generation-retry:${run.id}:${summary.state}`, { runId: run.id, state: summary.state });
      retryKey.current = key;
      if (summary.state === "result_consumption_failed") await retryContentGenerationResultConsumption(run.id, run.version, key);
      else await retryWorkflowRun(run.id, run.version, key);
      await onRefresh();
    } catch { /* The summary remains the authority; expose only a safe message below. */ }
    finally { setSubmitting(false); }
  };
  return <section className={`content-generation-status content-generation-status-${summary.state}`} aria-live="polite">
    <div><strong>{title}</strong>{run && <span>Run ID: {run.runNumber}</span>}
      {summary.state === "queued" && <span>正文生成任务已进入队列，等待执行。</span>}
      {summary.state === "running" && <span>正文生成正在执行，进度以运行事件为准。</span>}
      {summary.state === "candidate_ready" && <span>候选版本 v{summary.latestCandidateVersion?.version_no} 已创建，当前正文尚未被替换。</span>}
      {failed.has(summary.state) && <span>{safe(summary.latestError?.message ?? run?.errorMessage)}</span>}
      {summary.state === "not_configured" && <span>请在项目设置中绑定正文生成工作流，并确认执行连接可用。</span>}
    </div>
    <div className="content-generation-status-actions">
      {summary.state === "candidate_ready" && <button onClick={onCandidate}>查看候选</button>}
      {run && <button onClick={() => setShowEvents((x) => !x)}>{showEvents ? "收起详情" : "查看详情"}</button>}
      {summary.state === "running" && run && <Link href={`/workflow-runs/${run.id}`}>前往流程中心</Link>}
      {summary.state === "not_configured" && <Link href={`/projects/${projectId}/settings?tab=workflow-bindings`}>配置工作流</Link>}
      {summary.state === "runtime_failed" && <button onClick={() => void retry()} disabled={submitting}>{submitting ? "正在重试…" : "重新执行 Runtime"}</button>}
      {summary.state === "output_validation_failed" && <button onClick={() => void retry()} disabled={submitting}>{submitting ? "正在重试…" : "重新执行 Runtime"}</button>}
      {summary.state === "result_consumption_failed" && <button onClick={() => void retry()} disabled={submitting}>{submitting ? "正在重试…" : "重试结果消费"}</button>}
    </div>
    {showEvents && <ol className="content-generation-events">{(fetchedEvents ?? summary.latestEvents).length ? (fetchedEvents ?? summary.latestEvents).map((event) => <li key={event.id}><b>{eventLabel(event.eventType)}</b><span>{new Date(event.createdAt).toLocaleString("zh-CN")}</span></li>) : <li>暂无可公开的运行事件。</li>}</ol>}
  </section>;
}
