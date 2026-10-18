import type { Metadata } from "next";
import { Suspense } from "react";
import { AppMessage } from "../components/AppMessage";
import { MutualMatches } from "../components/MutualMatches";
import "../styles/dashboard.css";
import "../styles/profile.css";
import "../styles/discovery.css";

export const metadata: Metadata = {
  title: "Мої збіги — SkillSwap",
  description: "Студенти, з якими можна взаємно обмінятися навичками.",
};

export default function MatchesPage() {
  return (
    <Suspense fallback={<AppMessage status="loading" subject="збіги" path="/matches" />}>
      <MutualMatches />
    </Suspense>
  );
}
