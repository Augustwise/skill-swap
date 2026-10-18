import type { Metadata } from "next";
import { Suspense } from "react";
import { Notifications } from "../components/Notifications";
import { AppSkeleton } from "../components/Skeleton";
import "../styles/dashboard.css";
import "../styles/profile.css";
import "../styles/discovery.css";
import "../styles/exchange.css";
import "../styles/chat.css";

export const metadata: Metadata = {
  title: "Сповіщення — SkillSwap",
  description: "Нові запити, рішення, повідомлення та зміни занять у SkillSwap.",
};

export default function NotificationsPage() {
  return (
    <Suspense fallback={<AppSkeleton page="notifications" />}>
      <Notifications />
    </Suspense>
  );
}
