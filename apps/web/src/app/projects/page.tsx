import Link from "next/link";
import { AppShell } from "@/components/ui/app-shell";
import { Icon } from "@/components/ui/icons";
import { ProjectsList } from "@/components/projects/projects-list";
import { ApiError, listProjects, type ProjectStatus } from "@/lib/api";

const validStatuses = new Set<ProjectStatus>(["planning", "producing", "archived"]);

export default async function 项目Page({ searchParams }: { searchParams: Promise<{ status?: string }> }) {
  const { status: statusParam } = await searchParams;
  const status = statusParam && validStatuses.has(statusParam as ProjectStatus) ? statusParam as ProjectStatus : undefined;
  try {
    const projects = await listProjects({ status, limit: 20, offset: 0 });
    return <AppShell active="projects"><main className="projects-main"><div className="projects-canvas"><nav className="projects-breadcrumb"><span>首页</span><Icon name="arrowRight" size={14} /><span>项目</span></nav><header className="projects-heading"><div><h1>项目</h1><p>管理和继续你的内容创作项目</p></div><Link className="projects-primary" href="/projects/new" aria-label="新建项目"><Icon name="plus" size={20} strokeWidth={2.2} />新建项目</Link></header><ProjectsList items={projects.items} total={projects.total} status={status} /></div></main></AppShell>;
  } catch (error) {
    const message = error instanceof ApiError ? error.message : "Unable to load projects.";
    return <AppShell active="projects"><main className="projects-main"><div className="projects-canvas"><nav className="projects-breadcrumb"><span>首页</span><Icon name="arrowRight" size={14} /><span>项目</span></nav><header className="projects-heading"><div><h1>项目</h1><p>管理和继续你的内容创作项目</p></div><Link className="projects-primary" href="/projects/new"><Icon name="plus" size={20} />新建项目</Link></header><section className="projects-error" role="alert">Unable to load projects: {message}</section></div></main></AppShell>;
  }
}