import type { Metadata } from "next";
import { Dashboard } from "../components/Dashboard";
import "../styles/dashboard.css";

export const metadata: Metadata = {
  title: "Головна — SkillSwap",
  description: "Твої взаємні збіги, активні обміни навичками та найближчі сесії.",
};

export default function MainPage() {
  return <Dashboard />;
}
