import Link from "next/link";
import { AppShell } from "@/components/ui/app-shell";
import { Icon } from "@/components/ui/icons";
import { NewProjectForm } from "@/components/projects/new-project-form";

export default function NewProjectPage() {
  return (
    <AppShell active="projects">
      <main className="create-main"><div className="create-canvas">
        <nav className="create-breadcrumb"><Link href="/projects">项目</Link><Icon name="arrowRight" size={14} /><span>新建项目</span></nav>
        <header className="create-page-heading"><div><h1>新建项目</h1><p>创建一个新的内容项目，开始你的创作旅程</p></div><Link href="/projects" className="create-back"><Icon name="arrowLeft" size={18} />返回项目</Link></header>
        <section className="create-stage"><div className="create-stage-overlay" /><NewProjectForm /></section>
      </div></main>
    </AppShell>
  );
}
