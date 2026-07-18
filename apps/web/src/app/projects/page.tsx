import Link from "next/link";
import { AppShell } from "@/components/ui/app-shell";
import { Icon } from "@/components/ui/icons";
import { ProjectsList } from "@/components/projects/projects-list";
export default function 项目Page() {
  return <AppShell active="projects"><main className="projects-main"><div className="projects-canvas"><nav className="projects-breadcrumb"><span>首页</span><Icon name="arrowRight" size={14} /><span>项目</span></nav><header className="projects-heading"><div><h1>项目</h1><p>管理和继续你的内容创作项目</p></div><Link className="projects-primary" href="/projects/new" aria-label="新建项目"><Icon name="plus" size={20} strokeWidth={2.2} />新建项目</Link></header><ProjectsList /></div></main></AppShell>;
}
