import type { Metadata } from "next";
import { Suspense } from "react";
import { AppSkeleton } from "../components/Skeleton";
import { ExchangeRequests } from "../components/ExchangeRequests";
import "../styles/dashboard.css";
import "../styles/profile.css";
import "../styles/discovery.css";
import "../styles/exchange.css";

export const metadata: Metadata = {
  title: "Запити на обмін — SkillSwap",
  description: "Вхідні й надіслані запити на обмін навичками: прийняти, відхилити або відкликати.",
};

export default function RequestsPage() {
  return (
    <Suspense fallback={<AppSkeleton page="requests" />}>
      <ExchangeRequests />
    </Suspense>
  );
}
