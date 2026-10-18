import type { Metadata } from "next";
import { Messages } from "../../components/Messages";
import "../../styles/dashboard.css";
import "../../styles/profile.css";
import "../../styles/discovery.css";
import "../../styles/exchange.css";
import "../../styles/chat.css";

export const metadata: Metadata = {
  title: "Чат — SkillSwap",
  description: "Приватний чат з учасником запиту або обміну навичками.",
};

export default async function ConversationPage({
  params,
}: PageProps<"/messages/[conversationId]">) {
  const { conversationId } = await params;
  return <Messages conversationId={conversationId} />;
}
