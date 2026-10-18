import { Suspense } from "react";
import type { Metadata } from "next";
import { Schedule } from "../components/Schedule";
import "../styles/dashboard.css";
import "../styles/profile.css";
import "../styles/discovery.css";
import "../styles/exchange.css";
import "../styles/session.css";

export const metadata: Metadata = { title: "Розклад — SkillSwap" };

export default function SchedulePage() {
  return (
    <Suspense>
      <Schedule />
    </Suspense>
  );
}
