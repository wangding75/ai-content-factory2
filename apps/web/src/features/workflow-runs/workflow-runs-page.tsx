"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { ApiError, listProjects, type Project } from "@/lib/api";
import { stageLabels, type WorkflowStage } from "@/features/workflow-bindings/workflow-binding-api";
import { listWorkflowRuns, type WorkflowRunStatus, type WorkflowRunVm } from "./workflow-run-api";
import Link from "next/link";

type TimeRange = "all" | "today" | "sevenDays" | "thirtyDays";
const statusOptions: Array<{ value: "" | WorkflowRunStatus; label: string }> = [{ value: "", label: "全部状态" }, { value: "queued", label: "等待执行" }, { value: "running", label: "运行中" }, { value: "succeeded", label: "已成功" }, { value: "failed", label: "失败" }, { value: "cancelled", label: "已取消" }];
const stageOptions: Array<{ value: "" | WorkflowStage; label: string }> = [{ value: "", label: "全部环节" }, ...Object.entries(stageLabels).map(([value, label]) => ({ value: value as WorkflowStage, label }))];

function timeBounds(range: TimeRange) {
  if (range === "all") return {};
  const end = new Date();
  const start = new Date(end);
  if (range === "today") start.setHours(0, 0, 0, 0);
  if (range === "sevenDays") start.setDate(start.getDate() - 7);
  if (range === "thirtyDays") start.setDate(start.getDate() - 30);
  return { startTime: start.toISOString(), endTime: end.toISOString() };
}

function RunTable({ items, projects }: { items: WorkflowRunVm[]; projects: Project[] }) {
  const projectNames = new Map(projects.map((project) => [project.id, project.name]));
  return <div className="workflow-runs-table" role="table" aria-label="运行记录"><div className="workflow-runs-table-head" role="row"><span>运行编号</span><span>项目</span><span>业务环节</span><span>状态</span><span>触发来源</span><span>创建时间</span><span>更新时间</span><span>操作</span></div>{items.map((item) => <div className="workflow-runs-row" role="row" key={item.id}><strong>{item.runNumber}</strong><span>{projectNames.get(item.projectId) ?? "未知项目"}</span><span>{item.stageLabel}</span><span><i className={`workflow-runs-status ${item.status}`}>{item.statusLabel}</i></span><span>{item.triggerSourceLabel}</span><time>{item.createdAtLabel}</time><time>{item.updatedAtLabel}</time><Link className="workflow-runs-detail-link" href={`/workflow-runs/${item.id}`}>查看详情</Link></div>)}</div>;
}

export function WorkflowRunsPage({ projectId: initialProjectId, initialStage }: { projectId?: string; initialStage?: WorkflowStage }) {
  const [items, setItems] = useState<WorkflowRunVm[] | null>(null);
  const [projects, setProjects] = useState<Project[]>([]);
  const [error, setError] = useState<ApiError | null>(null);
  const [q, setQ] = useState("");
  const [projectId, setProjectId] = useState(initialProjectId ?? "");
  const [stage, setStage] = useState<"" | WorkflowStage>(initialStage ?? "");
  const [status, setStatus] = useState<"" | WorkflowRunStatus>("");
  const [timeRange, setTimeRange] = useState<TimeRange>("all");
  const query = useMemo(() => ({ projectId: projectId || undefined, stage: stage || undefined, status: status || undefined, q, limit: 50, offset: 0, ...timeBounds(timeRange) }), [projectId, q, stage, status, timeRange]);
  const load = useCallback(async (signal?: AbortSignal) => {
    setError(null);
    try {
      const result = await listWorkflowRuns(query, { signal });
      if (!signal?.aborted) setItems(result.items);
    } catch (cause) {
      if (!signal?.aborted && !(cause instanceof ApiError && cause.code === "cancelled")) setError(cause instanceof ApiError ? cause : new ApiError("暂时无法获取运行记录。", 0));
    }
  }, [query]);
  useEffect(() => {
    const controller = new AbortController();
    const timer = window.setTimeout(() => void load(controller.signal), q ? 300 : 0);
    return () => { window.clearTimeout(timer); controller.abort(); };
  }, [load, q]);
  useEffect(() => {
    const controller = new AbortController();
    void listProjects({ limit: 100, offset: 0, signal: controller.signal }).then((result) => { if (!controller.signal.aborted) setProjects(result.items); }).catch(() => undefined);
    return () => controller.abort();
  }, []);
  const reset = () => { setQ(""); setProjectId(initialProjectId ?? ""); setStage(initialStage ?? ""); setStatus(""); setTimeRange("all"); };
  const isLoading = items === null && !error;
  const isFiltered = Boolean(projectId || stage || status || q || timeRange !== "all");
  return <main className="workflow-runs-main"><div className="workflow-runs-canvas"><header className="workflow-runs-heading"><p className="workflow-runs-breadcrumb">流程中心 <span>/</span> 运行记录</p><h1>流程中心</h1><p>统一查看所有项目的工作流运行情况。</p>{initialProjectId && <p className="workflow-runs-context">已从项目入口应用初始筛选</p>}</header><section className="workflow-runs-panel" aria-label="运行记录列表"><div className="workflow-runs-filters"><label className="workflow-runs-search"><span>搜索</span><input value={q} onChange={(event) => setQ(event.target.value)} placeholder="搜索运行编号" aria-label="搜索运行编号" /></label><label><span>所属项目</span><select value={projectId} onChange={(event) => setProjectId(event.target.value)} aria-label="所属项目"><option value="">全部项目</option>{initialProjectId && !projects.some((project) => project.id === initialProjectId) && <option value={initialProjectId}>当前项目</option>}{projects.map((project) => <option value={project.id} key={project.id}>{project.name}</option>)}</select></label><label><span>业务环节</span><select value={stage} onChange={(event) => setStage(event.target.value as "" | WorkflowStage)} aria-label="业务环节">{stageOptions.map((option) => <option value={option.value} key={option.value}>{option.label}</option>)}</select></label><label><span>状态</span><select value={status} onChange={(event) => setStatus(event.target.value as "" | WorkflowRunStatus)} aria-label="状态">{statusOptions.map((option) => <option value={option.value} key={option.value}>{option.label}</option>)}</select></label><label><span>时间范围</span><select value={timeRange} onChange={(event) => setTimeRange(event.target.value as TimeRange)} aria-label="时间范围"><option value="all">全部时间</option><option value="today">今天</option><option value="sevenDays">最近 7 天</option><option value="thirtyDays">最近 30 天</option></select></label><button type="button" className="workflow-runs-reset" onClick={reset}>重置筛选</button></div>{error ? <section className="workflow-runs-state error" role="alert"><h2>运行记录加载失败</h2><p>暂时无法获取工作流运行记录，请稍后重试。</p><button type="button" onClick={() => void load()}>重新加载</button></section> : isLoading ? <section className="workflow-runs-state loading" role="status"><span/><p>正在加载运行记录…</p></section> : items?.length ? <RunTable items={items} projects={projects} /> : <section className="workflow-runs-state empty"><h2>{isFiltered ? "暂无符合筛选条件的运行记录" : "还没有工作流运行记录"}</h2><p>{isFiltered ? "请调整筛选条件后重试。" : "当项目触发章节规划、内容生成、审核或改写后，这里会展示运行状态与结果。"}</p></section>}</section></div></main>;
}
