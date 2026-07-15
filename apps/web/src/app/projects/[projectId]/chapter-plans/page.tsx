import {AppShell} from "@/components/ui/app-shell";
import {ChapterPlansWorkspace} from "@/features/chapter-plans/chapter-plans-workspace";
export default async function ChapterPlansRoute({params}:{params:Promise<{projectId:string}>}){const {projectId}=await params;return <AppShell active="projects"><ChapterPlansWorkspace projectId={projectId}/></AppShell>}
