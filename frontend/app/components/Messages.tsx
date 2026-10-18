"use client";

import { useState } from "react";
import { AppGate, type AppContext } from "./AppGate";
import { pageShells } from "./AppNavigation";
import { AppShell } from "./AppShell";
import { messagesHref } from "./ChatApi";
import { ChatPanel } from "./ChatPanel";
import { ConversationList } from "./ConversationList";
import { DashboardIcon as Icon } from "./DashboardIcon";

function MessagesView({
  profile,
  session,
  conversationId,
}: AppContext & { conversationId: string }) {
  const [version, setVersion] = useState(0);
  return (
    <AppShell
      profile={profile}
      csrfToken={session.csrfToken}
      section="messages"
      title={pageShells.messages.title}
      subtitle={pageShells.messages.subtitle}
      contentId="messages-content"
    >
      <div className={`chat-layout ${conversationId ? "chat-layout--open" : ""}`}>
        <ConversationList viewerId={profile.id} activeId={conversationId} version={version} />
        {conversationId ? (
          <ChatPanel
            key={conversationId}
            conversationId={conversationId}
            viewerId={profile.id}
            csrfToken={session.csrfToken}
            onActivity={() => setVersion((value) => value + 1)}
          />
        ) : (
          <section className="account-card chat-panel chat-panel--empty" aria-label="Чат">
            <span className="dashboard-empty__icon" aria-hidden="true">
              <Icon name="nav-chat" />
            </span>
            <h2>Обери розмову</h2>
            <p>
              Тут ти домовляєшся з партнером про деталі обміну. Чат з’являється для кожного запиту,
              а після прийняття переходить в обмін разом з усією історією.
            </p>
          </section>
        )}
      </div>
    </AppShell>
  );
}

export function Messages({ conversationId }: { conversationId: string }) {
  return (
    <AppGate page="messages" path={messagesHref(conversationId)}>
      {(context) => <MessagesView {...context} conversationId={conversationId} />}
    </AppGate>
  );
}
