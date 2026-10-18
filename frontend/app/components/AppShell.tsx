"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useRef, useState, type FormEvent, type ReactNode } from "react";
import { AccountMenu } from "./AccountMenu";
import { AuthError, authRequest, requestError } from "./AuthApi";
import { DashboardIcon as Icon, type DashboardIconName } from "./DashboardIcon";
import { DashboardModal } from "./DashboardModal";
import { normalizeQuery, searchQueryString, emptyFilters } from "./DiscoveryApi";
import { Logo } from "./Logo";
import type { Profile } from "./OnboardingApi";

export type AppSection = "main" | "search" | "matches" | "profile";

const navigation: {
  label: string;
  icon: DashboardIconName;
  section?: AppSection;
  href?: string;
}[] = [
  { label: "Головна", icon: "dashboard", section: "main", href: "/main" },
  { label: "Пошук навичок", icon: "nav-search", section: "search", href: "/search" },
  { label: "Мої збіги", icon: "nav-sparkles", section: "matches", href: "/matches" },
  { label: "Запити на обмін", icon: "inbox" },
  { label: "Активні обміни", icon: "nav-swap" },
  { label: "Повідомлення", icon: "nav-chat" },
  { label: "Календар", icon: "calendar" },
];

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
                aria-label={item.label}
                key={item.label}
              >
                <Icon name={item.icon} />
                <span>{item.label}</span>
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
          <AccountMenu profile={profile} pending={pending} onLogout={() => void logout()} />
        </header>
        <main className="dashboard-content" id={contentId} tabIndex={-1}>
          {children}
        </main>
      </div>
      {modal && <DashboardModal title={modal} onClose={() => setModal("")} />}
    </div>
  );
}
