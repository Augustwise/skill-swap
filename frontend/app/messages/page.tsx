import type { Metadata } from "next";
import { Messages } from "../components/Messages";
import "../styles/dashboard.css";
import "../styles/profile.css";
import "../styles/discovery.css";
import "../styles/exchange.css";
import "../styles/chat.css";

export const metadata: Metadata = {
  title: "Повідомлення — SkillSwap",
  description: "Приватні чати з учасниками твоїх запитів і обмінів навичками.",
};

export default function MessagesPage() {
  return <Messages conversationId="" />;
}
