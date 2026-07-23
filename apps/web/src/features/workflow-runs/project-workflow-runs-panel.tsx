"use client";

import Link from "next/link";
import { useCallback, useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { ApiError } from "@/lib/api";
import { listProjectWorkflowBindings, newIdempotencyKey, stageLabels, stageOrder, type BindingStage, type WorkflowStage } from "@/features/workflow-bindings/workflow-binding-api";
import { createWorkflowRun, getProjectWorkflowRunSummary, type ProjectWorkflowRunSummaryVm } from "./workflow-run-api";

const createError = (error: unknown) => {
  if (!(error instanceof ApiError)) return "暂时无法创建运行，请稍后重试。";
  if (["workflow_binding_not_found", "workflow_configuration_not_found", "workflow_connection_not_found"].includes(error.code)) return "工作流绑定或其配置已变更，请刷新后重新选择。";
  if (error.code === "idempotency_key_reused_with_different_payload") return "本次请求的幂等键发生冲突，请明确重新发起运行。";
  if (error.code === "validation_error") return "运行请求未通过校验，请刷新项目数据后重试。";
  return "创建运行未完成，请稍后重试。";
};

const isRunnableBinding = (item: BindingStage | undefined) =>
  Boolean(item?.bound && item.binding && item.workflowConfigurationSummary && item.workflowConfigurationSummary.connectionId);

export function ProjectWorkflowRunsPanel({ projectId }: { projectId: string }) {
  const router = useRouter();
  const [summary, setSummary] = useState<ProjectWorkflowRunSummaryVm | null>(null);
  const [summaryError, setSummaryError] = useState(false);
  const [bindings, setBindings] = useState<BindingStage[] | null>(null);
  const [bindingError, setBindingError] = useState(false);
  const [unboundStage, setUnboundStage] = useState<WorkflowStage | null>(null);
  const [creating, setCreating] = useState<WorkflowStage | null>(null);
  const [createNotice, setCreateNotice] = useState<string | null>(null);
  const requestKey = useRef<string | null>(null);

  const loadSummary = useCallback(async (signal?: AbortSignal) => {
    setSummaryError(false);
    try {
      const value = await getProjectWorkflowRunSummary(projectId, { signal });
      if (!signal?.aborted) setSummary(value);
    } catch {
      if (!signal?.aborted) setSummaryError(true);
    }
  }, [projectId]);
  const loadBindings = useCallback(async (signal?: AbortSignal) => {
    setBindingError(false);
    try {
      const value = await listProjectWorkflowBindings(projectId, { signal });
      if (!signal?.aborted) setBindings(value.items);
    } catch {
      if (!signal?.aborted) setBindingError(true);
    }
  }, [projectId]);
  const refreshBindings = useCallback(async () => { await loadBindings(); }, [loadBindings]);

  useEffect(() => {
    const controller = new AbortController();
    const timer = window.setTimeout(() => {
      void loadSummary(controller.signal);
      void loadBindings(controller.signal);
    }, 0);
    return () => {
      window.clearTimeout(timer);
      controller.abort();
    };
  }, [loadBindings, loadSummary]);

  const run = async (stage: WorkflowStage) => {
    const binding = bindings?.find((item) => item.stage === stage);
    if (!isRunnableBinding(binding)) {
      setUnboundStage(stage);
      return;
    }
    if (creating) return;
    const key = newIdempotencyKey();
    requestKey.current = key;
    setCreateNotice(null);
    setCreating(stage);
    try {
      // The frozen CreateRun contract defines no additional stage parameters. Its business input is the empty object.
      const created = await createWorkflowRun(projectId, stage, {}, key);
      router.push(`/workflow-runs/${encodeURIComponent(created.id)}`);
    } catch (error) {
      setCreateNotice(createError(error));
      if (error instanceof ApiError && ["workflow_binding_not_found", "workflow_configuration_not_found", "workflow_connection_not_found"].includes(error.code)) await refreshBindings();
    } finally {
      setCreating(null);
      requestKey.current = null;
    }
  };

  return (
    <section className="project-workflow-runs" aria-labelledby="project-workflow-runs-title">
      <header>
        <div>
          <h2 id="project-workflow-runs-title">工作流运行摘要</h2>
          <p>查看项目最近的工作流运行，并按环节发起新的排队运行。</p>
        </div>
        <Link href={{ pathname: "/workflow-runs", query: { projectId } }}>查看全部运行</Link>
      </header>
      {summaryError ? <div className="project-workflow-state" role="alert"><p>暂时无法加载运行摘要，请稍后重试。</p><button onClick={() => void loadSummary()}>重试</button></div> : !summary ? <div className="project-workflow-loading" aria-label="正在加载工作流运行摘要"><span /><span /><span /><span /></div> : <>
        <dl className="project-workflow-stats">
          <div><dt>总运行次数</dt><dd>{summary.totalRuns}</dd></div>
          <div><dt>运行中</dt><dd>{summary.activeRuns}</dd></div>
          <div><dt>最近失败</dt><dd>{summary.recentFailedRuns}</dd></div>
          <div><dt>最近运行</dt><dd>{summary.lastRunAtLabel}</dd></div>
        </dl>
        {summary.recentRuns.length ? <ul className="project-workflow-recent">{summary.recentRuns.map((run) => <li key={run.id}><div><strong>{run.runNumber}</strong><span>{run.stageLabel} · {run.statusLabel} · {run.createdAtLabel}</span></div><Link href={`/workflow-runs/${encodeURIComponent(run.id)}`}>查看详情</Link></li>)}</ul> : <p className="project-workflow-empty">暂无运行记录。可从下方环节发起新的运行。</p>}
      </>}
      <div className="project-workflow-actions" aria-label="按环节运行">
        {stageOrder.map((stage) => <div key={stage}><div><strong>{stageLabels[stage]}</strong><Link href={{ pathname: "/workflow-runs", query: { projectId, stage } }}>查看该环节运行</Link></div><button className="project-workflow-run" disabled={creating !== null || bindings === null} onClick={() => void run(stage)}>{creating === stage ? "创建中…" : "运行"}</button></div>)}
      </div>
      {bindingError && <p className="project-workflow-notice" role="status">暂时无法确认工作流绑定；请重试后再发起运行。<button onClick={() => void loadBindings()}>重试</button></p>}
      {createNotice && <p className="project-workflow-notice" role="alert">{createNotice}<button onClick={() => setCreateNotice(null)}>关闭</button></p>}
      {unboundStage && <div className="project-workflow-dialog-layer" role="presentation"><button className="project-workflow-dialog-backdrop" aria-label="关闭未绑定工作流提示" onClick={() => setUnboundStage(null)} /><section className="project-workflow-dialog" role="dialog" aria-modal="true" aria-labelledby="workflow-not-bound-title"><h2 id="workflow-not-bound-title">尚未绑定工作流</h2><p>“{stageLabels[unboundStage]}”环节尚未绑定工作流，因此无法发起运行。</p><p>完成绑定后，返回当前项目页面即可继续运行。</p><footer><button onClick={() => setUnboundStage(null)}>取消</button><Link href={{ pathname: `/projects/${projectId}/settings`, query: { tab: "workflow-bindings", stage: unboundStage } }}>前往工作流绑定</Link></footer></section></div>}
    </section>
  );
}
