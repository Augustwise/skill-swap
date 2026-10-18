import type { Metadata } from "next";
import { CandidateProfile } from "../../components/CandidateProfile";
import "../../styles/dashboard.css";
import "../../styles/profile.css";
import "../../styles/discovery.css";

export const metadata: Metadata = {
  title: "Профіль студента — SkillSwap",
  description: "Навички, рівні, формат занять і відгуки студента.",
};

export default async function StudentPage({
  params,
  searchParams,
}: PageProps<"/students/[studentId]">) {
  const { studentId } = await params;
  const { from } = await searchParams;
  return <CandidateProfile studentId={studentId} from={typeof from === "string" ? from : ""} />;
}
