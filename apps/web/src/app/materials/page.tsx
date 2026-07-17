import { AppShell } from "@/components/ui/app-shell";
import { GlobalMaterialsPage } from "@/features/global-lite/global-pages";

export default function Page() {
  return <AppShell active="materials"><GlobalMaterialsPage /></AppShell>;
}
