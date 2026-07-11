/* eslint-disable react-hooks/error-boundaries */
import Link from "next/link";
import { AppShell } from "@/components/ui/app-shell";
import { Icon } from "@/components/ui/icons";
import { ApiError, listProjects, type Project, type ProjectStatus } from "@/lib/api";

const statusLabels: Record<ProjectStatus, string> = { planning: "策划中", producing: "制作中", archived: "已归档" };
const formatUpdated = (value: string) => new Intl.DateTimeFormat("zh-CN", { month: "numeric", day: "numeric", hour: "2-digit", minute: "2-digit" }).format(new Date(value));

function QuickEntry({ href, icon, tone, title, description }: { href: string; icon: "filePlus" | "archive" | "wand"; tone: string; title: string; description: string }) {
  return <Link href={href} className="home-card home-quick-card"><span className={`home-quick-icon ${tone}`}><Icon name={icon} size={20} strokeWidth={2} /></span><span><h2>{title}</h2><p>{description}</p></span></Link>;
}

function ProjectCard({ project }: { project: Project }) {
  return <article className="home-card home-project-card"><div className="home-cover"><Icon name="book" size={42} strokeWidth={1.4} /><span>小说</span></div><div className="home-project-meta"><div><h3>{project.name}</h3><p>更新于 {formatUpdated(project.updated_at)}</p></div><span className="home-status">{statusLabels[project.status]}</span></div><Link className="home-open" href={`/projects/${project.id}`} aria-label="Open project">继续创作 <Icon name="arrowRight" size={16} strokeWidth={2} /></Link></article>;
}

function CreateCard() { return <article className="home-card home-create"><span className="home-create-icon"><Icon name="plus" size={24} /></span><h3>创建新项目</h3><p>开始一个新的内容项目</p><Link href="/projects/new" aria-label="New project">新建项目</Link></article>; }

function EmptyProjectCard() { return <article className="home-card home-empty"><span className="home-empty-icon"><Icon name="folder" size={24} /></span><h3>暂无项目</h3><span className="sr-only">No projects yet</span><p>创建你的第一个小说项目，开始一段全新的创作旅程。</p></article>; }

export default async function HomePage() {
  try {
    const projects = await listProjects({ limit: 5, offset: 0 });
    return <AppShell><main className="home-main"><div className="home-canvas"><header className="home-heading"><div><h1>欢迎回来</h1><p>管理你的内容项目、素材与作品</p></div><Link className="home-primary" href="/projects/new" aria-label="New project"><Icon name="plus" size={20} strokeWidth={2.2} />新建项目</Link></header><section className="home-quick" aria-label="快捷入口"><QuickEntry href="/projects/new" icon="filePlus" tone="" title="新建项目" description="开启一段全新的创作旅程" /><QuickEntry href="#" icon="archive" tone="blue" title="查看素材" description="管理并检索你的创作库" /><QuickEntry href="#" icon="wand" tone="orange" title="查看作品" description="回顾并分发已完成的作品" /></section><div className="home-grid"><section><div className="home-section-heading"><h2>最近项目</h2><Link href="/projects" aria-label="View all projects">查看全部</Link></div><div className="home-recent-grid">{projects.items.length ? projects.items.slice(0, 1).map((project) => <ProjectCard key={project.id} project={project} />) : <EmptyProjectCard />}<CreateCard /></div></section><section><div className="home-section-heading"><h2>数据概览</h2></div><div className="home-card home-dashboard"><span className="home-empty-icon"><Icon name="chart" size={30} strokeWidth={1.5} /></span><h3>数据大盘将在后续版本完善</h3><p>当前可以通过项目页面查看创作进度</p></div></section></div></div></main></AppShell>;
  } catch (error) {
    const message = error instanceof ApiError ? error.message : "Unable to load projects.";
    return <AppShell><main className="home-main"><div className="home-canvas"><header className="home-heading"><div><h1>欢迎回来</h1><p>管理你的内容项目、素材与作品</p></div><Link className="home-primary" href="/projects/new" aria-label="New project"><Icon name="plus" size={20} />新建项目</Link></header><section className="home-card home-error" role="alert">Unable to load recent projects: {message}</section></div></main></AppShell>;
  }
}