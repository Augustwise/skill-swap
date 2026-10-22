import type { Metadata } from "next";
import { Settings } from "../components/Settings";
import { settingsTab } from "../components/SafetyApi";
import { readPage } from "../components/DiscoveryApi";
import "../styles/dashboard.css";
import "../styles/profile.css";
import "../styles/discovery.css";

export const metadata: Metadata = { title: "Налаштування — SkillSwap" };
export default async function SettingsPage({ searchParams }: PageProps<"/settings">) {
  const query = await searchParams;
  return (
    <Settings
      tab={settingsTab(typeof query.tab === "string" ? query.tab : null)}
      page={readPage(typeof query.page === "string" ? query.page : null)}
    />
  );
}
