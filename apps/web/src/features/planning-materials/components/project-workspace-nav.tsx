import Link from "next/link";

const futureItems = ["Chapters", "Review", "Works", "Settings"];

export function ProjectWorkspaceNav({ projectId, active }: { projectId: string; active: "overview" | "planning" | "materials" | "storylines" }) {
  return <nav className="workspace-tabs" aria-label="Project workspace navigation">
    <Link className={active === "overview" ? "is-active" : ""} href={`/projects/${projectId}`}>Overview</Link>
    <Link className={active === "planning" ? "is-active" : ""} href={`/projects/${projectId}/planning`}>Planning</Link>
    <Link className={active === "materials" ? "is-active" : ""} href={`/projects/${projectId}/materials`}>Materials</Link>
    <Link className={active === "storylines" ? "is-active" : ""} href={`/projects/${projectId}/storylines`}>Storylines</Link>
    {futureItems.map((item) => <span className="is-disabled" aria-disabled="true" key={item}>{item}</span>)}
  </nav>;
}
