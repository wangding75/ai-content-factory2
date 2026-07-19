import { AppShell } from "@/components/ui/app-shell";
import { SettingsPage } from "@/features/global-lite/settings-page";
import { ConnectionSettingsPage } from "@/features/global-config/connection-settings-page";
export default async function Page({searchParams}:{searchParams:Promise<{tab?:string}>}){const {tab}=await searchParams;return <AppShell active="settings">{tab==="connections"?<ConnectionSettingsPage/>:<SettingsPage/>}</AppShell>}
