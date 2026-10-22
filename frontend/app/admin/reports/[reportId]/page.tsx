import type { Metadata } from "next";
import { Administration } from "../../../components/Administration";
import "../../../styles/dashboard.css";
import "../../../styles/profile.css";
import "../../../styles/discovery.css";

export const metadata: Metadata = { title: "Розгляд скарги — SkillSwap" };
export default async function ReportPage({
  params,
  searchParams,
}: PageProps<"/admin/reports/[reportId]">) {
  const { reportId } = await params;
  const { from } = await searchParams;
  return (
    <Administration
      tab="reports"
      page={1}
      status="OPEN"
      q=""
      userId=""
      reportId={reportId}
      from={typeof from === "string" ? from : ""}
    />
  );
}
