"use client";

import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { useState } from "react";
import { AppGate, type AppContext } from "./AppGate";
import { pageShells } from "./AppNavigation";
import { AppShell, useCountersChanged } from "./AppShell";
import { DiscoveryEmpty, Pagination } from "./DiscoveryCards";
import { DiscoveryError } from "./DiscoveryError";
import { pageCount } from "./DiscoveryApi";
import { countLabel } from "./DiscoveryFormat";
import {
  NotificationItem,
  NotificationSkeleton,
  useNotificationActions,
} from "./NotificationCards";
import {
  listNotifications,
  notificationErrorMessage,
  notificationsHref,
  readNotificationFilters,
  type NotificationFilters,
} from "./NotificationApi";
import { useApiQuery } from "./useApiQuery";

const notificationForms = ["сповіщення", "сповіщення", "сповіщень"] as const;

function NotificationFeed({
  csrfToken,
  filters,
}: {
  csrfToken: string;
  filters: NotificationFilters;
}) {
  const path = notificationsHref(filters);
  const [version, setVersion] = useState(0);
  const countersChanged = useCountersChanged();
  const result = useApiQuery(`${path}#${version}`, (signal) => listNotifications(filters, signal));
  const actions = useNotificationActions(csrfToken, () => {
    setVersion((value) => value + 1);
    countersChanged();
  });
  const data = result.state.status === "ready" ? result.state.data : null;
  const unread = data?.unreadCount ?? 0;

  return (
    <>
      <nav className="exchange-tabs" aria-label="Фільтр сповіщень">
        <Link
          className="exchange-tab"
          href={notificationsHref()}
          aria-current={filters.unread ? undefined : "page"}
        >
          Усі
        </Link>
        <Link
          className="exchange-tab"
          href={notificationsHref({ unread: true })}
          aria-current={filters.unread ? "page" : undefined}
        >
          Непрочитані
          {unread > 0 && (
            <span className="exchange-tab__count">
              <span className="sr-only">: </span>
              {unread}
            </span>
          )}
        </Link>
      </nav>
      <section
        className="account-card notification-feed"
        aria-labelledby="notifications-title"
        aria-busy={result.state.status === "loading"}
      >
        <div className="dashboard-section-heading">
          <div>
            <h2 id="notifications-title">
              {filters.unread ? "Непрочитані сповіщення" : "Усі сповіщення"}
            </h2>
            <p className="profile-hint" role="status">
              {result.state.status === "loading"
                ? "Завантажуємо сповіщення…"
                : data
                  ? `${countLabel(data.total, notificationForms)} · непрочитаних: ${unread}`
                  : "Не вдалося завантажити сповіщення"}
            </p>
          </div>
          <button
            className="dashboard-button"
            type="button"
            disabled={unread === 0 || actions.pending !== ""}
            onClick={() => void actions.markAll()}
          >
            {actions.pending === "all" ? "Позначаємо…" : "Позначити всі прочитаними"}
          </button>
        </div>
        {Boolean(actions.error) && (
          <p className="exchange-error" role="alert">
            {notificationErrorMessage(actions.error)}
          </p>
        )}
        {result.state.status === "loading" && <NotificationSkeleton count={5} />}
        {result.state.status === "error" && (
          <DiscoveryError
            error={result.state.error}
            path={path}
            onRetry={result.retry}
            resetHref="/notifications"
          />
        )}
        {data &&
          (data.items.length > 0 ? (
            <ul className="notification-list">
              {data.items.map((notification) => (
                <NotificationItem
                  notification={notification}
                  pending={actions.pending !== ""}
                  onOpen={actions.open}
                  onMarkRead={(item) => void actions.markRead(item)}
                  key={notification.id}
                />
              ))}
            </ul>
          ) : data.total > 0 ? (
            <DiscoveryEmpty icon="bell" title="На цій сторінці сповіщень немає.">
              <Link className="dashboard-button" href={notificationsHref({ ...filters, page: 1 })}>
                До першої сторінки
              </Link>
            </DiscoveryEmpty>
          ) : filters.unread ? (
            <DiscoveryEmpty icon="bell" title="Непрочитаних сповіщень немає.">
              <Link className="dashboard-button" href={notificationsHref()}>
                Показати всі
              </Link>
            </DiscoveryEmpty>
          ) : (
            <DiscoveryEmpty icon="bell" title="Сповіщень поки немає.">
              <p>
                Тут з’являться нові запити на обмін, відповіді на твої запити, повідомлення в чатах
                і зміни занять.
              </p>
            </DiscoveryEmpty>
          ))}
      </section>
      {data && (
        <Pagination
          page={filters.page}
          pages={pageCount(data.total, data.pageSize)}
          hrefFor={(page) => notificationsHref({ ...filters, page })}
        />
      )}
    </>
  );
}

function NotificationsView({
  profile,
  session,
  filters,
}: AppContext & { filters: NotificationFilters }) {
  return (
    <AppShell
      profile={profile}
      csrfToken={session.csrfToken}
      title={pageShells.notifications.title}
      subtitle={pageShells.notifications.subtitle}
      contentId="notifications-content"
    >
      <NotificationFeed csrfToken={session.csrfToken} filters={filters} />
    </AppShell>
  );
}

export function Notifications() {
  const filters = readNotificationFilters(useSearchParams());
  return (
    <AppGate page="notifications" path={notificationsHref(filters)}>
      {(context) => <NotificationsView {...context} filters={filters} />}
    </AppGate>
  );
}
