import { AppShell } from "@/components/ui/app-shell";
import { SettingsPage } from "@/features/global-lite/settings-page";
import { ConnectionSettingsPage } from "@/features/global-config/connection-settings-page";
import { WorkflowSettingsPage } from "@/features/global-config/workflow-settings-page";
import { DistributionSettingsPage } from "@/features/global-config/distribution-settings-page";
import { GlobalSettingsWorkspace } from "@/features/global-config/global-settings-tabs";
export default async function Page({searchParams}:{searchParams:Promise<{tab?:string}>}){const {tab}=await searchParams;const section=tab==="distribution"?"distribution":tab==="connections"||tab==="workflows"?"workflow":"llm";return <AppShell active="settings"><GlobalSettingsWorkspace section={section} workflowSection={tab==="workflows"?"workflows":"connections"}>{tab==="connections"?<ConnectionSettingsPage/>:tab==="workflows"?<WorkflowSettingsPage/>:tab==="distribution"?<DistributionSettingsPage/>:<SettingsPage/>}</GlobalSettingsWorkspace></AppShell>}
