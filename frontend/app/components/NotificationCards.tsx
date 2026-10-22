"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useRef, useState, type MouseEvent } from "react";
import { DashboardIcon as Icon, type DashboardIconName } from "./DashboardIcon";
import { formatRelative } from "./ExchangeFormat";
import {
  markAllNotificationsRead,
  markNotificationRead,
  notificationHref,
  type AppNotification,
  type NotificationEvent,
} from "./NotificationApi";
import { Skeleton } from "./Skeleton";

const icons: Record<NotificationEvent, DashboardIconName> = {
  EXCHANGE_REQUEST: "inbox",
  REQUEST_RESPONSE: "handshake",
  CHAT_MESSAGE: "nav-chat",
  SESSION_UPDATED: "calendar",
  REVIEW_RECEIVED: "star",
};

export function useNotificationActions(csrfToken: string, onChanged: () => void) {
  const router = useRouter();
  const [pending, setPending] = useState("");
  const [error, setError] = useState<unknown>(null);
  const busy = useRef(false);

  async function run(id: string, action: () => Promise<unknown>) {
    if (busy.current) return false;
    busy.current = true;
    setPending(id);
    setError(null);
    try {
      await action();
      return true;
    } catch (problem) {
      setError(problem);
      return false;
    } finally {
      busy.current = false;
      setPending("");
      onChanged();
    }
  }

  function open(event: MouseEvent<HTMLAnchorElement>, notification: AppNotification) {
    if (notification.isRead || event.button !== 0) return;
    if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
    event.preventDefault();
    const href = notificationHref(notification);
    void run(notification.id, () =>
      markNotificationRead(notification.id, csrfToken).catch(() => {}),
    ).then(() => router.push(href));
  }

  return {
    pending,
    error,
    open,
    markRead: (notification: AppNotification) =>
      run(notification.id, () => markNotificationRead(notification.id, csrfToken)),
    markAll: () => run("all", () => markAllNotificationsRead(csrfToken)),
  };
}

export function NotificationItem({
  notification,
  pending,
  onOpen,
  onMarkRead,
}: {
  notification: AppNotification;
  pending: boolean;
  onOpen: (event: MouseEvent<HTMLAnchorElement>, notification: AppNotification) => void;
  onMarkRead: (notification: AppNotification) => void;
}) {
  const unread = !notification.isRead;
  return (
    <li className={`notification-item ${unread ? "notification-item--unread" : ""}`}>
      <span className="notification-item__icon" aria-hidden="true">
        <Icon name={icons[notification.eventType] ?? "bell"} />
      </span>
      <div className="notification-item__content">
        <Link
          className="notification-item__link"
          href={notificationHref(notification)}
          onClick={(event) => onOpen(event, notification)}
          aria-disabled={pending || undefined}
        >
          {unread && <span className="sr-only">Непрочитане: </span>}
          <strong>{notification.title}</strong>
          {notification.body && <span>{notification.body}</span>}
        </Link>
        <time dateTime={notification.createdAt}>{formatRelative(notification.createdAt)}</time>
      </div>
      {unread && (
        <button
          className="notification-item__read"
          type="button"
          disabled={pending}
          onClick={() => onMarkRead(notification)}
          aria-label={`Позначити прочитаним: ${notification.title}`}
          title="Позначити прочитаним"
        >
          <Icon name="check" />
        </button>
      )}
    </li>
  );
}

export function NotificationSkeleton({ count }: { count: number }) {
  return (
    <ul className="notification-list" aria-hidden="true">
      {Array.from({ length: count }, (_, index) => (
        <li className="notification-item" key={index}>
          <Skeleton kind="icon" width={36} height={36} />
          <span className="skeleton-stack">
            <Skeleton width="55%" />
            <Skeleton kind="small" width="85%" />
            <Skeleton kind="small" width={70} />
          </span>
        </li>
      ))}
    </ul>
  );
}
