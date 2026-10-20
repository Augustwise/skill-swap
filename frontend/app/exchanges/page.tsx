import { Suspense } from "react";
import type { Metadata } from "next";
import { ActiveExchanges } from "../components/ActiveExchanges";
import "../styles/dashboard.css";
import "../styles/profile.css";
import "../styles/discovery.css";
import "../styles/exchange.css";
import "../styles/session.css";

export const metadata: Metadata = { title: "Мої обміни — SkillSwap" };

export default function ExchangesPage() {
  return (
    <Suspense>
      <ActiveExchanges />
    </Suspense>
  );
}
