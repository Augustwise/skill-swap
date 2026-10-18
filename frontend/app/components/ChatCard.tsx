"use client";

import Link from "next/link";
import { messagesHref, type Conversation } from "./ChatApi";
import { DashboardIcon as Icon } from "./DashboardIcon";
import { formatRelative } from "./ExchangeFormat";
import { Skeleton } from "./Skeleton";
import { useApiQuery } from "./useApiQuery";

// The chat of a request or an exchange on its page: the last message and a link to the chat.
export function ChatCard({
  queryKey,
  load,
  viewerId,
}: {
  queryKey: string;
  load: (signal: AbortSignal) => Promise<Conversation>;
  viewerId: string;
}) {
  const result = useApiQuery(queryKey, load);
  const conversation = result.state.status === "ready" ? result.state.data : null;
  const last = conversation?.lastMessage;
  const unread = conversation?.unreadCount ?? 0;

  return (
    <section
      className="account-card chat-card"
      aria-labelledby="chat-card-title"
      aria-busy={result.state.status === "loading"}
    >
      <div className="dashboard-section-heading">
        <h2 id="chat-card-title">Чат</h2>
        {unread > 0 && (
          <span className="dashboard-nav-badge">
            {unread}
            <span className="sr-only"> непрочитаних</span>
          </span>
        )}
      </div>
      {result.state.status === "loading" && (
        <span className="skeleton-stack" aria-hidden="true">
          <Skeleton width="80%" />
          <Skeleton width="55%" />
          <Skeleton kind="button" />
        </span>
      )}
      {result.state.status === "error" && (
        <div className="chat-card__error" role="alert">
          <p>Не вдалося завантажити чат.</p>
          <button className="dashboard-button" type="button" onClick={result.retry}>
            Спробувати ще раз
          </button>
        </div>
      )}
      {conversation && (
        <>
          {last ? (
            <div className="chat-card__last">
              <p className="chat-card__meta">
                {last.author?.id === viewerId ? "Ти" : (last.author?.firstName ?? "SkillSwap")} ·{" "}
                <time dateTime={last.createdAt}>{formatRelative(last.createdAt)}</time>
              </p>
              <p className="chat-card__text">{last.body}</p>
            </div>
          ) : (
            <p className="profile-hint">
              Повідомлень ще немає. Обговоріть деталі обміну: зручний час, місце чи посилання на
              зустріч.
            </p>
          )}
          <Link
            className="dashboard-button dashboard-button--primary"
            href={messagesHref(conversation.id)}
          >
            <Icon name="chat-white" />
            {last ? "Відкрити чат" : "Написати повідомлення"}
          </Link>
        </>
      )}
    </section>
  );
}
