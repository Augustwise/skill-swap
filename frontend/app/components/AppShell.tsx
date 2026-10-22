"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { createContext, useContext, useRef, useState, type FormEvent, type ReactNode } from "react";
import { AccountMenu } from "./AccountMenu";
import { AuthError, authRequest, requestError } from "./AuthApi";
import { navigation, type AppSection } from "./AppNavigation";
import { DashboardIcon as Icon } from "./DashboardIcon";
import { DashboardModal } from "./DashboardModal";
import { unreadMessageCount } from "./ChatApi";
import { normalizeQuery, searchQueryString, emptyFilters } from "./DiscoveryApi";
import { pendingRequestCount } from "./ExchangeApi";
import { Logo } from "./Logo";
import { NotificationBell } from "./NotificationBell";
import { unreadNotificationCount } from "./NotificationApi";
import type { Profile } from "./OnboardingApi";
import { usePolling } from "./usePolling";
import { useIsAdmin } from "./SessionRole";

export type { AppSection };

const COUNTERS_INTERVAL = 30_000;
const CountersChanged = createContext<() => void>(() => {});

// Pages call this after an action that changes a navigation counter: a request decision,
// a sent or read message, a read notification.
export function useCountersChanged() {
  return useContext(CountersChanged);
}

const badgeLabels: Partial<Record<AppSection, string>> = {
  requests: "очікують відповіді",
  messages: "непрочитані",
};

function badgeText(count: number) {
  return count > 99 ? "99+" : String(count);
}

export function AppShell({
  profile,
  csrfToken,
  section,
  title,
  subtitle,
  contentId = "app-content",
  search = true,
  toolbar,
  canLeave,
  className = "",
  children,
}: {
  profile: Profile;
  csrfToken: string;
  section?: AppSection;
  title: string;
  subtitle: string;
  contentId?: string;
  search?: boolean;
  toolbar?: ReactNode;
  canLeave?: () => boolean;
  className?: string;
  children: ReactNode;
}) {
  const router = useRouter();
  const isAdmin = useIsAdmin();
  const [modal, setModal] = useState("");
  const [notice, setNotice] = useState("");
  const [pending, setPending] = useState(false);
  const [query, setQuery] = useState("");
  const logoutPending = useRef(false);
  const [counters, setCounters] = useState({ requests: 0, messages: 0, notifications: 0 });
  const [countersVersion, setCountersVersion] = useState(0);
  const countersChanged = () => setCountersVersion((value) => value + 1);
  const badges: Partial<Record<AppSection, number>> = {
    requests: counters.requests,
    messages: counters.messages,
  };

  usePolling(
    async (signal) => {
      const [requests, messages, notifications] = await Promise.allSettled([
        pendingRequestCount("incoming", signal),
        unreadMessageCount(signal),
        unreadNotificationCount(signal),
      ]);
      if (signal.aborted) return;
      setCounters((current) => ({
        requests: requests.status === "fulfilled" ? requests.value : current.requests,
        messages: messages.status === "fulfilled" ? messages.value : current.messages,
        notifications:
          notifications.status === "fulfilled" ? notifications.value : current.notifications,
      }));
    },
    COUNTERS_INTERVAL,
    countersVersion,
  );

  async function logout() {
    if (logoutPending.current || (canLeave && !canLeave())) return;
    logoutPending.current = true;
    setPending(true);
    setNotice("");
    try {
      await authRequest("auth/logout", { method: "POST" }, csrfToken);
      router.replace("/login");
    } catch (error) {
      if (error instanceof AuthError && error.status === 401) {
        router.replace("/login");
        return;
      }
      setNotice(requestError(error));
      logoutPending.current = false;
      setPending(false);
    }
  }

  function submitSearch(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (canLeave && !canLeave()) return;
    const params = searchQueryString({ ...emptyFilters, q: normalizeQuery(query) });
    router.push(params ? `/search?${params}` : "/search");
  }

  return (
    <div
      className={`dashboard-shell ${className}`.trim()}
      onClickCapture={(event) => {
        if (!canLeave || event.ctrlKey || event.metaKey || event.shiftKey || event.altKey) return;
        const link = event.target instanceof Element ? event.target.closest("a[href]") : null;
        const href = link?.getAttribute("href");
        if (
          href &&
          !href.startsWith("#") &&
          new URL(href, window.location.href).pathname !== window.location.pathname &&
          !canLeave()
        ) {
          event.preventDefault();
          event.stopPropagation();
        }
      }}
    >
      <a className="dashboard-skip" href={`#${contentId}`}>
        До вмісту
      </a>
      <aside className="dashboard-sidebar" aria-label="Бічна панель">
        <div className="dashboard-sidebar__brand">
          <Logo variant="app" />
        </div>
        <nav className="dashboard-nav" aria-label="Основна навігація">
          {navigation.map((item) => {
            const badge = item.section ? (badges[item.section] ?? 0) : 0;
            return item.href ? (
              <Link
                className={`dashboard-nav-item ${item.section === section ? "dashboard-nav-item--active" : ""}`}
                href={item.href}
                aria-current={item.section === section ? "page" : undefined}
                aria-label={
                  badge > 0 && item.section
                    ? `${item.label}: ${badge} ${badgeLabels[item.section]}`
                    : item.label
                }
                key={item.label}
              >
                <Icon name={item.icon} />
                <span>{item.label}</span>
                {badge > 0 && (
                  <span className="dashboard-nav-badge" aria-hidden="true">
                    {badgeText(badge)}
                  </span>
                )}
              </Link>
            ) : (
              <button
                className="dashboard-nav-item"
                type="button"
                onClick={() => setModal(item.label)}
                aria-haspopup="dialog"
                aria-label={item.label}
                key={item.label}
              >
                <Icon name={item.icon} />
                <span>{item.label}</span>
              </button>
            );
          })}
        </nav>
        <div className="dashboard-sidebar__footer">
          <Link
            className={`dashboard-nav-item ${section === "settings" ? "dashboard-nav-item--active" : ""}`}
            href="/settings"
            aria-label="Налаштування"
            aria-current={section === "settings" ? "page" : undefined}
          >
            <Icon name="settings" />
            <span>Налаштування</span>
          </Link>
          {isAdmin && (
            <Link
              className={`dashboard-nav-item ${section === "admin" ? "dashboard-nav-item--active" : ""}`}
              href="/admin?status=OPEN"
              aria-label="Модерація"
              aria-current={section === "admin" ? "page" : undefined}
            >
              <Icon name="inbox" />
              <span>Модерація</span>
            </Link>
          )}
          <Link
            className={`dashboard-nav-item ${section === "profile" ? "dashboard-nav-item--active" : ""}`}
            href="/profile"
            aria-current={section === "profile" ? "page" : undefined}
            aria-label="Мій профіль"
          >
            <Icon name="settings" />
            <span>Мій профіль</span>
          </Link>
          <button
            className="dashboard-nav-item"
            type="button"
            disabled={pending}
            onClick={() => void logout()}
            aria-label={pending ? "Виходимо…" : "Вийти"}
          >
            <Icon name="logout" />
            <span>{pending ? "Виходимо…" : "Вийти"}</span>
          </button>
          {notice && (
            <p className="dashboard-notice" role="alert">
              {notice}
            </p>
          )}
        </div>
      </aside>
      <div className="dashboard-main">
        <header className="dashboard-topbar">
          <div className="dashboard-topbar__title">
            <h1>{title}</h1>
            <p>{subtitle}</p>
          </div>
          {search && (
            <form className="dashboard-search" role="search" onSubmit={submitSearch}>
              <Icon name="search" />
              <label className="sr-only" htmlFor={`${contentId}-search`}>
                Пошук студентів за навичкою або ім’ям
              </label>
              <input
                id={`${contentId}-search`}
                type="search"
                name="q"
                placeholder="Пошук навички або людини…"
                maxLength={100}
                value={query}
                onChange={(event) => setQuery(event.target.value)}
              />
            </form>
          )}
          {toolbar}
          <NotificationBell
            csrfToken={csrfToken}
            unreadCount={counters.notifications}
            onRead={countersChanged}
          />
          <AccountMenu
            profile={profile}
            isAdmin={isAdmin}
            pending={pending}
            badges={{ "/requests": counters.requests, "/messages": counters.messages }}
            onLogout={() => void logout()}
          />
        </header>
        <main className="dashboard-content" id={contentId} tabIndex={-1}>
          <CountersChanged.Provider value={countersChanged}>{children}</CountersChanged.Provider>
        </main>
      </div>
      {modal && <DashboardModal title={modal} onClose={() => setModal("")} />}
    </div>
  );
}
