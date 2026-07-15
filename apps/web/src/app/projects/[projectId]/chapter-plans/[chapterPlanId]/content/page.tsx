import {AppShell} from "@/components/ui/app-shell";
import {ContentEditorWorkspace} from "@/features/content-items/content-editor-workspace";
export default async function ContentEditorRoute({params}:{params:Promise<{projectId:string;chapterPlanId:string}>}){const {projectId,chapterPlanId}=await params;return <AppShell active="projects"><ContentEditorWorkspace projectId={projectId} chapterPlanId={chapterPlanId}/></AppShell>}
