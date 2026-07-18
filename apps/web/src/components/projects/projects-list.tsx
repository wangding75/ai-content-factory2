"use client";

import Link from "next/link";
import { useEffect, useMemo, useState } from "react";
import { Icon } from "@/components/ui/icons";
import { ApiError, listProjects, type ProjectStatus } from "@/lib/api";
import { mapProjectCard, projectErrorMessage } from "./project-presentation";

const filters: { value?: ProjectStatus; label: string }[] = [{ label: "全部" }, { value: "planning", label: "策划中" }, { value: "producing", label: "生产中" }, { value: "archived", label: "已归档" }];

export function ProjectsList() {
  const [selected, setSelected] = useState<ProjectStatus | undefined>();
  const [searchDraft, setSearchDraft] = useState("");
  const [q, setQ] = useState("");
  const [reload, setReload] = useState(0);
  const [state, setState] = useState<{ kind: "loading" } | { kind: "error"; message: string } | { kind: "success"; items: ReturnType<typeof mapProjectCard>[]; total: number }>({ kind: "loading" });

  useEffect(() => {
    const controller = new AbortController();
    void listProjects({ status: selected, q, limit: 100, offset: 0, signal: controller.signal }).then((result) => {
      if (!controller.signal.aborted) setState({ kind: "success", items: result.items.map(mapProjectCard), total: result.total });
    }).catch((error: unknown) => {
      if (!controller.signal.aborted) setState({ kind: "error", message: projectErrorMessage(error instanceof ApiError ? error : undefined) });
    });
    return () => controller.abort();
  }, [q, selected, reload]);

  const selectedLabel = useMemo(() => filters.find((filter) => filter.value === selected)?.label ?? "全部", [selected]);
  return <>
    <section className="projects-filter-bar" aria-label="项目筛选">
      <label className="projects-search"><Icon name="search" size={17} /><input aria-label="搜索项目名称" value={searchDraft} onChange={(event) => setSearchDraft(event.target.value)} onKeyDown={(event) => { if (event.key === "Enter") setQ(searchDraft.trim()); }} placeholder="搜索项目名称" />{searchDraft && <button type="button" aria-label="清空项目搜索" onClick={() => { setSearchDraft(""); setQ(""); }}>清空</button>}</label>
      <div className="projects-filter-controls" role="group" aria-label="项目状态">
        {filters.map((filter) => <button type="button" className={filter.value === selected ? "is-active" : ""} aria-pressed={filter.value === selected} onClick={() => { if (filter.value !== selected) { setState({ kind: "loading" }); setSelected(filter.value); } }} key={filter.label}>{filter.label}</button>)}
      </div>
      {state.kind === "success" && <p className="projects-count">{selectedLabel}：共 {state.total} 个项目</p>}
    </section>
    {state.kind === "loading" && <section className="projects-loading" aria-live="polite" aria-busy="true"><span /><span /><span /><p>正在加载项目…</p></section>}
    {state.kind === "error" && <section className="projects-error" role="alert"><h2>项目加载失败</h2><p>{state.message}</p><button type="button" onClick={() => { setState({ kind: "loading" }); setReload((value) => value + 1); }}>重试</button></section>}
    {state.kind === "success" && (state.items.length ? <div className="projects-grid"><CreateProjectCard />{state.items.map((project) => <ProjectCard key={project.id} project={project} />)}</div> : <EmptyState selectedLabel={selectedLabel} searched={Boolean(q)} clearSearch={() => { setSearchDraft(""); setQ(""); }} />)}
  </>;
}

function CreateProjectCard() { return <Link href="/projects/new" className="projects-create-card" aria-label="新建项目"><span className="projects-create-icon"><Icon name="plus" size={32} /></span><h2>创建新项目</h2><p>开始一个新的内容创作项目</p><span>新建项目</span></Link>; }
function ProjectCard({ project }: { project: ReturnType<typeof mapProjectCard> }) { return <article className="projects-card"><Link href={project.href} className="projects-card-link" aria-label={`进入项目 ${project.name}`}><div className="projects-cover"><Icon name="book" size={44} strokeWidth={1.35} /><span className="projects-cover-status">{project.statusLabel}</span><span className="projects-cover-type">{project.typeLabel}</span><h2>{project.name}</h2></div><div className="projects-card-body"><p className="projects-description">{project.description}</p><div className="projects-progress"><span><Icon name="chart" size={17} />当前阶段：{project.stageLabel}</span><div><i /></div></div><div className="projects-card-footer"><time>更新于 {project.updatedLabel}</time><span>进入项目 <Icon name="arrowRight" size={18} /></span></div></div></Link></article>; }
function EmptyState({ selectedLabel, searched, clearSearch }: { selectedLabel: string; searched: boolean; clearSearch: () => void }) { const filtered = selectedLabel !== "全部" || searched; return <section className="projects-empty"><span><Icon name="folder" size={28} /></span><h2>{filtered ? "暂无匹配项目" : "还没有项目"}</h2><p>{filtered ? "可以调整搜索或筛选条件后重试。" : "创建一个项目，开始内容创作。"}</p>{searched && <button type="button" onClick={clearSearch}>清空搜索</button>}<Link href="/projects/new">新建项目</Link></section>; }
