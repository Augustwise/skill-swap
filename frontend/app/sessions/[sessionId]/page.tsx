import type { Metadata } from "next";
import { SessionOverview } from "../../components/SessionOverview";
import { safeAppPath } from "../../components/AppRoutes";
import "../../styles/dashboard.css";
import "../../styles/profile.css";
import "../../styles/discovery.css";
import "../../styles/exchange.css";
import "../../styles/session.css";

export const metadata: Metadata = { title: "Заняття — SkillSwap" };

export default async function SessionPage({
  params,
  searchParams,
}: PageProps<"/sessions/[sessionId]">) {
  const { sessionId } = await params;
  const { from } = await searchParams;
  const returnPath = typeof from === "string" ? safeAppPath(from, "/schedule") : undefined;
  return <SessionOverview sessionId={sessionId} from={returnPath} />;
}
