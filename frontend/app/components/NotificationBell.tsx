"use client";

import Link from "next/link";
import { useEffect, useId, useRef, useState } from "react";
import { DashboardIcon as Icon } from "./DashboardIcon";
import {
  NotificationItem,
  NotificationSkeleton,
  useNotificationActions,
} from "./NotificationCards";
import { listNotifications, notificationErrorMessage } from "./NotificationApi";
import { useApiQuery } from "./useApiQuery";

const shownNotifications = 6;

export function NotificationBell({
  csrfToken,
  unreadCount,
  onRead,
}: {
  csrfToken: string;
  unreadCount: number;
  onRead: () => void;
}) {
  const [open, setOpen] = useState(false);
  const [version, setVersion] = useState(0);
  const container = useRef<HTMLDivElement>(null);
  const trigger = useRef<HTMLButtonElement>(null);
  const panel = useRef<HTMLElement>(null);
  const id = useId();
  // The list loads while the panel is open and reloads when the unread counter changes.
  const list = useApiQuery(open ? `notifications#${version}#${unreadCount}` : null, (signal) =>
    listNotifications({ unread: false, page: 1 }, signal),
  );
  const actions = useNotificationActions(csrfToken, () => {
    setVersion((value) => value + 1);
    onRead();
  });
  const data = list.state.status === "ready" ? list.state.data : null;
  const unread = data?.unreadCount ?? unreadCount;

  useEffect(() => {
    if (!open) return;
    function dismiss(event: PointerEvent) {
      if (event.target instanceof Node && !container.current?.contains(event.target)) {
        setOpen(false);
      }
    }
    document.addEventListener("pointerdown", dismiss);
    return () => document.removeEventListener("pointerdown", dismiss);
  }, [open]);

  return (
    <div
      className="notification-menu"
      ref={container}
      onBlur={(event) => {
        // A read button disappears after its click; only focus moving elsewhere closes the panel.
        if (event.relatedTarget && !event.currentTarget.contains(event.relatedTarget)) {
          setOpen(false);
        }
      }}
      onKeyDown={(event) => {
        if (event.key === "Escape" && open) {
          event.preventDefault();
          setOpen(false);
          trigger.current?.focus();
        }
      }}
    >
      <button
        className="dashboard-notifications"
        type="button"
        ref={trigger}
        aria-label={unreadCount > 0 ? `Сповіщення: ${unreadCount} непрочитаних` : "Сповіщення"}
        aria-expanded={open}
        aria-controls={id}
        onClick={() => setOpen((value) => !value)}
      >
        <Icon name="bell" />
        {unreadCount > 0 && (
          <span className="notification-menu__count" aria-hidden="true">
            {unreadCount > 99 ? "99+" : unreadCount}
          </span>
        )}
      </button>
      <section
        className="notification-menu__panel"
        id={id}
        ref={panel}
        tabIndex={-1}
        hidden={!open}
        aria-labelledby={`${id}-title`}
      >
        <div className="notification-menu__header">
          <h2 id={`${id}-title`}>Сповіщення</h2>
          {unread > 0 && (
            <button
              className="dashboard-section-action"
              type="button"
              disabled={actions.pending !== ""}
              onClick={() => void actions.markAll()}
            >
              {actions.pending === "all" ? "Позначаємо…" : "Прочитати всі"}
            </button>
          )}
        </div>
        {Boolean(actions.error) && (
          <p className="exchange-error" role="alert">
            {notificationErrorMessage(actions.error)}
          </p>
        )}
        <div className="notification-menu__body" aria-busy={list.state.status === "loading"}>
          {list.state.status === "loading" && <NotificationSkeleton count={3} />}
          {list.state.status === "error" && (
            <div className="notification-menu__state" role="alert">
              <p>{notificationErrorMessage(list.state.error)}</p>
              <button className="dashboard-button" type="button" onClick={list.retry}>
                Спробувати ще раз
              </button>
            </div>
          )}
          {data &&
            (data.items.length > 0 ? (
              <ul className="notification-list">
                {data.items.slice(0, shownNotifications).map((notification) => (
                  <NotificationItem
                    notification={notification}
                    pending={actions.pending !== ""}
                    onOpen={(event, item) => {
                      actions.open(event, item);
                      setOpen(false);
                    }}
                    onMarkRead={(item) =>
                      void actions.markRead(item).then(() => panel.current?.focus())
                    }
                    key={notification.id}
                  />
                ))}
              </ul>
            ) : (
              <div className="notification-menu__state">
                <span className="dashboard-empty__icon" aria-hidden="true">
                  <Icon name="bell" />
                </span>
                <p>Сповіщень поки немає. Тут з’являться нові запити, рішення й повідомлення.</p>
              </div>
            ))}
        </div>
        <Link
          className="notification-menu__footer"
          href="/notifications"
          onClick={() => setOpen(false)}
        >
          Усі сповіщення
          <Icon name="chevron-right" />
        </Link>
      </section>
    </div>
  );
}
