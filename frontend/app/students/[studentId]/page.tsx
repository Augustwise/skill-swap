import type { Metadata } from "next";
import { CandidateProfile } from "../../components/CandidateProfile";
import { readPage } from "../../components/DiscoveryApi";
import "../../styles/dashboard.css";
import "../../styles/profile.css";
import "../../styles/discovery.css";
import "../../styles/exchange.css";

export const metadata: Metadata = {
  title: "Профіль студента — SkillSwap",
  description: "Навички, рівні, формат занять і відгуки студента.",
};

export default async function StudentPage({
  params,
  searchParams,
}: PageProps<"/students/[studentId]">) {
  const { studentId } = await params;
  const { from, reviewsPage } = await searchParams;
  return (
    <CandidateProfile
      studentId={studentId}
      from={typeof from === "string" ? from : ""}
      reviewsPage={readPage(typeof reviewsPage === "string" ? reviewsPage : null)}
    />
  );
}
