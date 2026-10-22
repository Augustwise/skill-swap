"use client";

import Link from "next/link";
import { useState } from "react";
import { studentHref } from "./AppRoutes";
import { useCountersChanged } from "./AppShell";
import { AuthError } from "./AuthApi";
import { ChatComposer } from "./ChatComposer";
import { ChatThread } from "./ChatThread";
import { conversationContext, conversationDetails, messagesHref } from "./ChatApi";
import { DashboardAvatar } from "./DashboardCards";
import { DashboardIcon as Icon } from "./DashboardIcon";
import { DiscoveryEmpty } from "./DiscoveryCards";
import { DiscoveryError } from "./DiscoveryError";
import { avatarColor, fullName, initials } from "./DiscoveryFormat";
import { exchangeHref, requestPerspective } from "./ExchangeApi";
import { SkillTag, StatusBadge } from "./ExchangeCards";
import { Skeleton } from "./Skeleton";
import { useApiQuery } from "./useApiQuery";
import { useChat } from "./useChat";
import { StudentSafety } from "./StudentSafety";

export function ChatSkeleton() {
  return (
    <div className="chat-skeleton" aria-hidden="true">
      <span className="skeleton-row">
        <Skeleton kind="avatar-40" />
        <span className="skeleton-stack">
          <Skeleton width="40%" />
          <Skeleton kind="small" width="60%" />
        </span>
      </span>
      <span className="chat-skeleton__messages">
        <Skeleton kind="block" width="46%" height={44} />
        <Skeleton kind="block" width="38%" height={44} />
        <Skeleton kind="block" width="52%" height={62} />
        <Skeleton kind="block" width="30%" height={44} />
      </span>
      <Skeleton kind="field" />
    </div>
  );
}

function ChatMissing() {
  return (
    <DiscoveryEmpty icon="nav-chat" title="Розмову не знайдено.">
      <p>Можливо, посилання неправильне або ти не береш участі в цій розмові.</p>
      <Link className="dashboard-button dashboard-button--primary" href="/messages">
        До всіх розмов
      </Link>
    </DiscoveryEmpty>
  );
}

export function ChatPanel({
  conversationId,
  viewerId,
  csrfToken,
  onActivity,
}: {
  conversationId: string;
  viewerId: string;
  csrfToken: string;
  onActivity: () => void;
}) {
  const path = messagesHref(conversationId);
  const countersChanged = useCountersChanged();
  const [blocked, setBlocked] = useState(false);
  const [safetyNotice, setSafetyNotice] = useState("");
  const details = useApiQuery(path, (signal) => conversationDetails(conversationId, signal));
  const chat = useChat({
    conversationId,
    viewerId,
    csrfToken,
    onActivity: () => {
      countersChanged();
      onActivity();
    },
  });
  const conversation = details.state.status === "ready" ? details.state.data : null;

  async function send(body: string) {
    try {
      await chat.send(body);
    } catch (error) {
      if (error instanceof AuthError && error.code === "conversation_blocked") setBlocked(true);
      throw error;
    }
  }

  const failed =
    details.state.status === "error"
      ? { error: details.state.error, retry: details.retry }
      : chat.status === "error"
        ? { error: chat.error, retry: chat.retry }
        : null;

  if (failed) {
    return (
      <section className="account-card chat-panel chat-panel--state" aria-label="Чат">
        {safetyNotice && (
          <p className="profile-success" role="status">
            {safetyNotice}
          </p>
        )}
        <Link className="discovery-back chat-panel__back" href="/messages">
          ← До розмов
        </Link>
        <DiscoveryError
          error={failed.error}
          path={path}
          onRetry={failed.retry}
          notFound={<ChatMissing />}
        />
      </section>
    );
  }

  if (!conversation || chat.status === "loading") {
    return (
      <section className="account-card chat-panel" aria-busy="true" aria-label="Чат">
        {safetyNotice && (
          <p className="profile-success" role="status">
            {safetyNotice}
          </p>
        )}
        <p className="sr-only" role="status">
          Завантажуємо розмову…
        </p>
        <ChatSkeleton />
      </section>
    );
  }

  const view = requestPerspective(conversation, viewerId);
  const context = conversationContext(conversation);

  return (
    <section className="account-card chat-panel" aria-labelledby="chat-title">
      <header className="chat-panel__header">
        <Link className="discovery-back chat-panel__back" href="/messages">
          ← До розмов
        </Link>
        <DashboardAvatar
          initials={initials(conversation.partner)}
          color={avatarColor(conversation.partner.id)}
        />
        <div className="chat-panel__identity">
          <h2 id="chat-title">
            <Link
              className="exchange-person__link"
              href={studentHref(conversation.partner.id, path)}
            >
              {fullName(conversation.partner)}
            </Link>
          </h2>
          <div className="dashboard-tags" aria-label="Обмін навичками">
            <SkillTag name={view.partnerTeaches.skill.name} learn />
            <Icon name="swap" />
            <SkillTag name={view.youTeach.skill.name} />
          </div>
        </div>
        <div className="chat-panel__context">
          <StudentSafety
            student={conversation.partner}
            csrfToken={csrfToken}
            path={path}
            blockedByMe={conversation.blockedByMe}
            onChanged={(notice) => {
              setSafetyNotice(notice);
              setBlocked(false);
              details.retry();
              onActivity();
            }}
          />
          {conversation.exchangeId && conversation.exchangeStatus === "ACTIVE" && (
            <Link
              className="dashboard-button dashboard-button--primary"
              href={`${exchangeHref(conversation.exchangeId)}#exchange-lessons`}
            >
              Запланувати заняття
            </Link>
          )}
          {context.badge && <StatusBadge status={context.badge.status} kind={context.badge.kind} />}
          <Link className="dashboard-button" href={context.href}>
            {context.label}
            <Icon name="chevron-right" />
          </Link>
        </div>
      </header>
      {safetyNotice && (
        <p className="safety-notice" role="status">
          {safetyNotice}
        </p>
      )}
      {!chat.online && (
        <p className="chat-connection" role="status">
          Немає з’єднання із сервером. Пробуємо знову кожні кілька секунд — нові повідомлення
          з’являться, щойно зв’язок відновиться.
        </p>
      )}
      <ChatThread
        messages={chat.messages}
        viewerId={viewerId}
        partnerName={conversation.partner.firstName}
        hasEarlier={chat.hasEarlier}
        earlier={chat.earlier}
        onLoadEarlier={() => void chat.loadEarlier()}
      />
      {conversation.canSend && !blocked ? (
        <ChatComposer partnerName={conversation.partner.firstName} path={path} onSend={send} />
      ) : (
        <p className="chat-blocked" role="note">
          Нові повідомлення в цій розмові недоступні: хтось із вас заблокував іншого або акаунт
          призупинено. Історія листування залишається доступною.
        </p>
      )}
    </section>
  );
}
