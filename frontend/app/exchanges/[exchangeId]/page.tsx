import type { Metadata } from "next";
import { ExchangeOverview } from "../../components/ExchangeOverview";
import "../../styles/dashboard.css";
import "../../styles/profile.css";
import "../../styles/discovery.css";
import "../../styles/exchange.css";
import "../../styles/chat.css";

export const metadata: Metadata = {
  title: "Обмін навичками — SkillSwap",
  description: "Погоджені умови обміну навичками та його прогрес.",
};

export default async function ExchangePage({
  params,
  searchParams,
}: PageProps<"/exchanges/[exchangeId]">) {
  const { exchangeId } = await params;
  const { started } = await searchParams;
  return <ExchangeOverview exchangeId={exchangeId} started={started === "1"} />;
}
