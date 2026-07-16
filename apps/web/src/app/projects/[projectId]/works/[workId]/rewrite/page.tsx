import {AppShell} from "@/components/ui/app-shell";import {RewriteWorkspace} from "@/features/project-works/rewrite-workspace";
export default async function RewriteRoute({params}:{params:Promise<{projectId:string;workId:string}>}){const {projectId,workId}=await params;return <AppShell active="projects"><RewriteWorkspace projectId={projectId} workId={workId}/></AppShell>}
