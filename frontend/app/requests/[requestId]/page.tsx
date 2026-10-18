import type { Metadata } from "next";
import { RequestOverview } from "../../components/RequestOverview";
import "../../styles/dashboard.css";
import "../../styles/profile.css";
import "../../styles/discovery.css";
import "../../styles/exchange.css";
import "../../styles/chat.css";

export const metadata: Metadata = {
  title: "Запит на обмін — SkillSwap",
  description: "Умови запиту на обмін навичками, повідомлення та історія статусів.",
};

export default async function RequestPage({
  params,
  searchParams,
}: PageProps<"/requests/[requestId]">) {
  const { requestId } = await params;
  const { from } = await searchParams;
  return <RequestOverview requestId={requestId} from={typeof from === "string" ? from : ""} />;
}
