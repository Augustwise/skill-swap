import type { Metadata } from "next";
import { SessionOverview } from "../../components/SessionOverview";
import "../../styles/dashboard.css";
import "../../styles/profile.css";
import "../../styles/discovery.css";
import "../../styles/exchange.css";
import "../../styles/session.css";

export const metadata: Metadata = { title: "Заняття — SkillSwap" };

export default async function SessionPage({ params }: PageProps<"/sessions/[sessionId]">) {
  const { sessionId } = await params;
  return <SessionOverview sessionId={sessionId} />;
}
