import type { Metadata } from "next";
import { Profile } from "../components/Profile";
import "../styles/dashboard.css";
import "../styles/profile.css";

export const metadata: Metadata = {
  title: "Мій профіль — SkillSwap",
  description: "Твої особисті дані, навички та формати навчання.",
};

export default function ProfilePage() {
  return <Profile />;
}
