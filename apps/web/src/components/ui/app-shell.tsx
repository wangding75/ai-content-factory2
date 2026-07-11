import Link from "next/link";
import type { ReactNode } from "react";
import { Icon, type IconName } from "@/components/ui/icons";

const nav: { href: string; label: string; icon: IconName; active?: boolean }[] = [
  { href: "/", label: "首页", icon: "home", active: true },
  { href: "/projects", label: "项目", icon: "folder" },
  { href: "#", label: "素材", icon: "archive" },
  { href: "#", label: "作品", icon: "wand" },
  { href: "#", label: "流程", icon: "workflow" },
];

export function AppShell({ children, active = "home" }: { children: ReactNode; active?: "home" | "projects" }) {
  return <div className="shell"><aside className="shell-sidebar"><div className="shell-brand"><span className="shell-mark"><Icon name="sparkles" size={21} strokeWidth={2.2} /></span><div><p>AI Content<br />Factory</p><small>内容创作平台</small></div></div><nav className="shell-nav">{nav.map((item) => <Link className={(item.label === "首页" ? active === "home" : item.label === "项目" ? active === "projects" : false) ? "is-active" : ""} href={item.href} key={item.label}><Icon name={item.icon} /><span>{item.label}</span></Link>)}<Link className="shell-settings" href="#"><Icon name="settings" /><span>设置</span></Link></nav></aside><header className="shell-topbar"><label className="shell-search"><Icon name="search" size={19} /><input placeholder="搜索项目或素材..." aria-label="搜索项目或素材" /></label><div className="shell-actions"><button aria-label="帮助"><Icon name="help" size={21} /></button><button aria-label="通知" className="shell-notification"><Icon name="bell" size={21} /><i /></button><span className="shell-divider" /><div className="shell-user"><span className="shell-avatar">CA</span><span>创作管理员</span></div></div></header>{children}</div>;
}