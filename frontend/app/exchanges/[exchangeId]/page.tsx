import type { Metadata } from "next";
import { ExchangeOverview } from "../../components/ExchangeOverview";
import { safeAppPath } from "../../components/AppRoutes";
import "../../styles/dashboard.css";
import "../../styles/profile.css";
import "../../styles/discovery.css";
import "../../styles/exchange.css";
import "../../styles/chat.css";
import "../../styles/session.css";

export const metadata: Metadata = {
  title: "Обмін навичками — SkillSwap",
  description: "Погоджені умови обміну навичками та його прогрес.",
};

export default async function ExchangePage({
  params,
  searchParams,
}: PageProps<"/exchanges/[exchangeId]">) {
  const { exchangeId } = await params;
  const { started, from } = await searchParams;
  return (
    <ExchangeOverview
      exchangeId={exchangeId}
      started={started === "1"}
      from={typeof from === "string" ? safeAppPath(from, "/exchanges") : undefined}
    />
  );
}
