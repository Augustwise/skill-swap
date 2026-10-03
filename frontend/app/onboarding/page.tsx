import type { Metadata } from "next";
import { Onboarding } from "../components/Onboarding";

export const metadata: Metadata = {
  title: "Онбординг — SkillSwap",
  description: "Заповни профіль, додай свої навички та обери, чого хочеш навчитися.",
};

export default function OnboardingPage() {
  return <Onboarding />;
}
