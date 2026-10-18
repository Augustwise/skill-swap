"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  createContext,
  useContext,
  useEffect,
  useRef,
  useState,
  type FormEvent,
  type ReactNode,
} from "react";
import { AccountMenu } from "./AccountMenu";
import { AuthError, authRequest, requestError } from "./AuthApi";
import { navigation, type AppSection } from "./AppNavigation";
import { DashboardIcon as Icon } from "./DashboardIcon";
import { DashboardModal } from "./DashboardModal";
import { normalizeQuery, searchQueryString, emptyFilters } from "./DiscoveryApi";
import { pendingRequestCount } from "./ExchangeApi";
import { Logo } from "./Logo";
import type { Profile } from "./OnboardingApi";

export type { AppSection };

const RequestsChanged = createContext<() => void>(() => {});

// Pages call this after answering or sending a request so the navigation counter stays current.
export function useRequestsChanged() {
  return useContext(RequestsChanged);
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
  const [modal, setModal] = useState("");
  const [notice, setNotice] = useState("");
  const [pending, setPending] = useState(false);
  const [query, setQuery] = useState("");
  const logoutPending = useRef(false);
  const [incoming, setIncoming] = useState(0);
  const [requestsVersion, setRequestsVersion] = useState(0);

  useEffect(() => {
    const controller = new AbortController();
    pendingRequestCount("incoming", controller.signal).then(setIncoming, () => {});
    return () => controller.abort();
  }, [requestsVersion]);

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
          {navigation.map((item) =>
            item.href ? (
              <Link
                className={`dashboard-nav-item ${item.section === section ? "dashboard-nav-item--active" : ""}`}
                href={item.href}
                aria-current={item.section === section ? "page" : undefined}
                aria-label={
                  item.section === "requests" && incoming > 0
                    ? `${item.label}: ${incoming} очікують відповіді`
                    : item.label
                }
                key={item.label}
              >
                <Icon name={item.icon} />
                <span>{item.label}</span>
                {item.section === "requests" && incoming > 0 && (
                  <span className="dashboard-nav-badge" aria-hidden="true">
                    {incoming > 99 ? "99+" : incoming}
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
            ),
          )}
        </nav>
        <div className="dashboard-sidebar__footer">
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
          <AccountMenu
            profile={profile}
            pending={pending}
            incoming={incoming}
            onLogout={() => void logout()}
          />
        </header>
        <main className="dashboard-content" id={contentId} tabIndex={-1}>
          <RequestsChanged.Provider value={() => setRequestsVersion((value) => value + 1)}>
            {children}
          </RequestsChanged.Provider>
        </main>
      </div>
      {modal && <DashboardModal title={modal} onClose={() => setModal("")} />}
    </div>
  );
}
