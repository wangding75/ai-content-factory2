"use client";

import Link from "next/link";
import { useMemo, useState } from "react";
import { Icon } from "@/components/ui/icons";
import type { Project, ProjectStatus } from "@/lib/api";

const statusLabels: Record<ProjectStatus, string> = { planning: "策划中", producing: "制作中", archived: "已归档" };
const formatUpdated = (value: string) => new Intl.DateTimeFormat("zh-CN", { month: "numeric", day: "numeric", hour: "2-digit", minute: "2-digit", timeZone: "Asia/Shanghai" }).format(new Date(value));

function CreateProjectCard() { return <Link href="/projects/new" className="projects-create-card" aria-label="新建项目"><span className="projects-create-icon"><Icon name="plus" size={32} /></span><h2>创建新项目</h2><p>开始一个新的内容创作项目</p><span>新建项目</span></Link>; }

function ProjectCard({ project }: { project: Project }) { return <article className="projects-card"><div className="projects-cover"><Icon name="book" size={44} strokeWidth={1.35} /><span className="projects-cover-status">{statusLabels[project.status]}</span><span className="projects-cover-type">{project.type === "novel" ? "小说" : project.type}</span><h2>{project.name}</h2></div><div className="projects-card-body"><p className="projects-description">{project.description || "暂无项目简介"}</p><div className="projects-progress"><span><Icon name="chart" size={17} />项目创作进度待开始</span><div><i /></div></div><div className="projects-card-footer"><time>更新于 {formatUpdated(project.updated_at)}</time><Link href={`/projects/${project.id}`} aria-label={`进入项目 ${project.name}`}>进入项目 <Icon name="arrowRight" size={18} /></Link></div></div></article>; }

export function ProjectsList({ items, total, status }: { items: Project[]; total: number; status?: ProjectStatus }) {
  const [query, setQuery] = useState("");
  const visibleItems = useMemo(() => { const normalized = query.trim().toLocaleLowerCase(); return normalized ? items.filter((project) => project.name.toLocaleLowerCase().includes(normalized)) : items; }, [items, query]);
  return <><form className="projects-filter-bar" action="/projects"><div className="projects-filter-controls"><select id="status" name="status" defaultValue={status ?? ""} aria-label="项目状态"><option value="">全部</option><option value="planning">策划中</option><option value="producing">制作中</option><option value="archived">已归档</option></select><button type="submit">筛选</button></div><label className="projects-search"><Icon name="search" size={19} /><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="搜索项目名称" aria-label="搜索项目名称" /></label></form><p className="projects-count">共 {total} 个项目{query ? `，显示 ${visibleItems.length} 个` : ""}</p><div className="projects-grid"><CreateProjectCard />{visibleItems.map((project) => <ProjectCard key={project.id} project={project} />)}</div>{visibleItems.length === 0 && <section className="projects-empty"><span><Icon name="folder" size={28} /></span><h2>暂无匹配项目</h2><p>调整筛选条件，或创建一个新的小说项目。</p><Link href="/projects/new">新建项目</Link></section>}</>;
}
