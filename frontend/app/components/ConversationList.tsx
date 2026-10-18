"use client";

import Link from "next/link";
import { useState } from "react";
import {
  listConversations,
  messagePreview,
  messagesHref,
  withFirstPage,
  withNextPage,
  type Conversation,
} from "./ChatApi";
import { listTime } from "./ChatFormat";
import { DashboardAvatar } from "./DashboardCards";
import { DiscoveryEmpty } from "./DiscoveryCards";
import { DiscoveryError } from "./DiscoveryError";
import { avatarColor, fullName, initials } from "./DiscoveryFormat";
import { requestPerspective } from "./ExchangeApi";
import { Skeleton } from "./Skeleton";
import { usePolling } from "./usePolling";

const LIST_INTERVAL = 30_000;

type ListState =
  | { status: "loading" }
  | { status: "error"; error: unknown }
  | { status: "ready"; items: Conversation[]; total: number; pages: number };

function ConversationRow({
  conversation,
  viewerId,
  active,
}: {
  conversation: Conversation;
  viewerId: string;
  active: boolean;
}) {
  const view = requestPerspective(conversation, viewerId);
  const unread = conversation.unreadCount;
  const time = conversation.lastMessageAt ?? conversation.createdAt;
  return (
    <li>
      <Link
        className={`chat-list-item ${unread > 0 ? "chat-list-item--unread" : ""}`}
        href={messagesHref(conversation.id)}
        aria-current={active ? "page" : undefined}
      >
        <DashboardAvatar
          initials={initials(conversation.partner)}
          color={avatarColor(conversation.partner.id)}
        />
        <span className="chat-list-item__body">
          <span className="chat-list-item__top">
            <strong>{fullName(conversation.partner)}</strong>
            <time dateTime={time}>{listTime(time)}</time>
          </span>
          <span className="chat-list-item__skills">
            {view.partnerTeaches.skill.name} ⇄ {view.youTeach.skill.name}
          </span>
          <span className="chat-list-item__bottom">
            <span className="chat-list-item__preview">
              {messagePreview(conversation, viewerId)}
            </span>
            {unread > 0 && (
              <span className="dashboard-nav-badge">
                <span className="sr-only">, непрочитані: </span>
                {unread > 99 ? "99+" : unread}
              </span>
            )}
          </span>
        </span>
      </Link>
    </li>
  );
}

function ListSkeleton() {
  return (
    <div className="chat-list" aria-hidden="true">
      {Array.from({ length: 4 }, (_, index) => (
        <span className="chat-list-item" key={index}>
          <Skeleton kind="avatar-40" />
          <span className="skeleton-stack">
            <Skeleton width="60%" />
            <Skeleton kind="small" width="45%" />
            <Skeleton kind="small" width="85%" />
          </span>
        </span>
      ))}
    </div>
  );
}

// Your chats, most recent activity first. The first page refreshes every 30 seconds and
// after activity in the open chat; older pages load on request.
export function ConversationList({
  viewerId,
  activeId,
  version,
}: {
  viewerId: string;
  activeId: string;
  version: number;
}) {
  const [state, setState] = useState<ListState>({ status: "loading" });
  const [attempt, setAttempt] = useState(0);
  const [more, setMore] = useState<"idle" | "loading" | "error">("idle");

  usePolling(
    async (signal) => {
      try {
        const page = await listConversations(1, signal);
        if (signal.aborted) return;
        setState((current) =>
          current.status === "ready"
            ? { ...current, items: withFirstPage(current.items, page.items), total: page.total }
            : { status: "ready", items: page.items, total: page.total, pages: 1 },
        );
      } catch (error) {
        if (signal.aborted) return;
        setState((current) => (current.status === "ready" ? current : { status: "error", error }));
      }
    },
    LIST_INTERVAL,
    `${version}#${attempt}`,
  );

  async function loadMore() {
    if (state.status !== "ready") return;
    setMore("loading");
    try {
      const page = await listConversations(state.pages + 1);
      setState((current) =>
        current.status === "ready"
          ? {
              status: "ready",
              items: withNextPage(current.items, page.items),
              total: page.total,
              pages: current.pages + 1,
            }
          : current,
      );
      setMore("idle");
    } catch {
      setMore("error");
    }
  }

  return (
    <section
      className="account-card chat-sidebar"
      aria-labelledby="conversations-title"
      aria-busy={state.status === "loading"}
    >
      <div className="chat-sidebar__heading">
        <h2 id="conversations-title">Розмови</h2>
        {state.status === "ready" && state.total > 0 && (
          <span className="exchange-tab__count">{state.total}</span>
        )}
      </div>
      {state.status === "loading" && <ListSkeleton />}
      {state.status === "error" && (
        <DiscoveryError
          error={state.error}
          path={messagesHref(activeId)}
          onRetry={() => {
            setState({ status: "loading" });
            setAttempt((value) => value + 1);
          }}
        />
      )}
      {state.status === "ready" &&
        (state.items.length > 0 ? (
          <>
            <ul className="chat-list">
              {state.items.map((conversation) => (
                <ConversationRow
                  conversation={conversation}
                  viewerId={viewerId}
                  active={conversation.id === activeId}
                  key={conversation.id}
                />
              ))}
            </ul>
            {state.items.length < state.total && (
              <div className="chat-sidebar__more">
                <button
                  className="dashboard-button"
                  type="button"
                  disabled={more === "loading"}
                  onClick={() => void loadMore()}
                >
                  {more === "loading" ? "Завантажуємо…" : "Показати ще"}
                </button>
                {more === "error" && (
                  <span className="exchange-error" role="alert">
                    Не вдалося завантажити розмови. Спробуй ще раз.
                  </span>
                )}
              </div>
            )}
          </>
        ) : (
          <DiscoveryEmpty icon="nav-chat" title="Розмов поки немає.">
            <p>Чат з’являється, щойно ти надсилаєш або отримуєш запит на обмін.</p>
            <Link className="dashboard-button dashboard-button--primary" href="/matches">
              До взаємних збігів
            </Link>
          </DiscoveryEmpty>
        ))}
    </section>
  );
}
