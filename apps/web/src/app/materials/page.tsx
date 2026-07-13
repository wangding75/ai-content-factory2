import { AppShell } from "@/components/ui/app-shell";
import { GlobalMaterialsPage } from "@/features/planning-materials/materials/global-materials-page";

export default function Page() {
  return <AppShell active="materials"><GlobalMaterialsPage /></AppShell>;
}
