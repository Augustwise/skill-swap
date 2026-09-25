import type { Metadata } from "next";
import { Onboarding } from "../components/Onboarding";

export const metadata: Metadata = {
  title: "Онбординг — SkillSwap",
  description: "Обери навички, які хочеш вивчити, і знайди людей для обміну знаннями.",
};

export default function OnboardingPage() {
  return <Onboarding />;
}
