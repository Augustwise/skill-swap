import type { Metadata } from "next";
import { Administration } from "../components/Administration";
import { adminTab, reportStatus } from "../components/AdminApi";
import { readPage } from "../components/DiscoveryApi";
import "../styles/dashboard.css";
import "../styles/profile.css";
import "../styles/discovery.css";

export const metadata: Metadata = { title: "Модерація — SkillSwap" };
export default async function AdminPage({ searchParams }: PageProps<"/admin">) {
  const query = await searchParams;
  const value = (key: string) => (typeof query[key] === "string" ? (query[key] as string) : "");
  return (
    <Administration
      tab={adminTab(value("tab"))}
      page={readPage(value("page"))}
      status={query.status === undefined ? "OPEN" : reportStatus(value("status"))}
      q={Array.from(value("q").trim()).slice(0, 100).join("")}
      userId={value("userId")}
    />
  );
}
