"use client";

import Link from "next/link";
import { useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { Logo } from "./Logo";
import { OnboardingError, onboardingRequest, type Profile } from "./OnboardingApi";
import type { DashboardMatch } from "./DashboardData";
import { DashboardIcon as Icon, type DashboardIconName } from "./DashboardIcon";
import { DashboardModal, type DashboardModalContent } from "./DashboardModal";
import {
  DashboardAvatar,
  DashboardExchanges,
  DashboardEmptyState,
  DashboardMatchCard,
  DashboardRequests,
  DashboardSessions,
  DashboardStats,
} from "./DashboardCards";

const navigation: { label: string; icon: DashboardIconName }[] = [
  { label: "Головна", icon: "dashboard" },
  { label: "Пошук навичок", icon: "nav-search" },
  { label: "Мої збіги", icon: "nav-sparkles" },
  { label: "Запити на обмін", icon: "inbox" },
  { label: "Активні обміни", icon: "nav-swap" },
  { label: "Повідомлення", icon: "nav-chat" },
  { label: "Календар", icon: "calendar" },
];

// Activity endpoints are not available yet; do not substitute design fixtures for user data.
const availableMatches: DashboardMatch[] = [];

export function Dashboard() {
  const router = useRouter();
  const [profile, setProfile] = useState<Profile | null>(null);
  const [status, setStatus] = useState<"loading" | "ready" | "login" | "error">("loading");
  const [attempt, setAttempt] = useState(0);
  const [csrfToken, setCsrfToken] = useState("");
  const [query, setQuery] = useState("");
  const [loggingOut, setLoggingOut] = useState(false);
  const logoutPending = useRef(false);
  const [notice, setNotice] = useState("");
  const [modal, setModal] = useState<DashboardModalContent | null>(null);

  useEffect(() => {
    const controller = new AbortController();
    async function load() {
      try {
        const [session, data] = await Promise.all([
          onboardingRequest<{ csrfToken: string }>("auth/me", { signal: controller.signal }),
          onboardingRequest<{ profile: Profile }>("me/profile", { signal: controller.signal }),
        ]);
        if (controller.signal.aborted) return;
        setCsrfToken(session.csrfToken);
        setProfile(data.profile);
        setStatus("ready");
      } catch (error) {
        if (!controller.signal.aborted) {
          setStatus(error instanceof OnboardingError && error.status === 401 ? "login" : "error");
        }
      }
    }
    void load();
    return () => controller.abort();
  }, [attempt]);

  async function logout() {
    if (logoutPending.current) return;
    logoutPending.current = true;
    setLoggingOut(true);
    setNotice("");
    try {
      const response = await fetch("/api/v1/auth/logout", {
        method: "POST",
        credentials: "same-origin",
        headers: { "X-CSRF-Token": csrfToken },
      });
      if (!response.ok && response.status !== 401) throw new Error("Logout failed");
      router.replace("/login");
    } catch {
      setNotice("Не вдалося вийти з акаунта. Спробуй ще раз.");
      logoutPending.current = false;
      setLoggingOut(false);
    }
  }

  if (status !== "ready" || !profile) {
    return (
      <main className="dashboard-message-page">
        <div className="dashboard-message" role={status === "loading" ? "status" : "alert"}>
          <Logo variant="app" />
          <h1>
            {status === "loading"
              ? "Завантажуємо головну…"
              : status === "login"
                ? "Увійди, щоб продовжити"
                : "Не вдалося завантажити головну"}
          </h1>
          <p>
            {status === "loading"
              ? "Готуємо твій профіль."
              : status === "login"
                ? "Твоя сесія завершилася. Увійди в акаунт, щоб відкрити головну."
                : "Перевір з’єднання із сервером і спробуй ще раз."}
          </p>
          {status === "login" && (
            <Link className="dashboard-button dashboard-button--primary" href="/login?next=/main">
              Увійти
            </Link>
          )}
          {status === "error" && (
            <button
              className="dashboard-button dashboard-button--primary"
              type="button"
              onClick={() => {
                setStatus("loading");
                setAttempt((value) => value + 1);
              }}
            >
              Спробувати ще раз
            </button>
          )}
        </div>
      </main>
    );
  }

  const initials = `${profile.firstName.charAt(0)}${profile.lastName.charAt(0)}`.toLocaleUpperCase(
    "uk",
  );
  const name = `${profile.firstName} ${profile.lastName}`.trim();
  const university = `${profile.university.name}${profile.course ? ` · ${profile.course} курс` : ""}`;
  const search = query.trim().toLocaleLowerCase("uk");
  const matches = availableMatches.filter((match) =>
    [match.name, match.university, ...match.teaching, ...match.learning].some((value) =>
      value.toLocaleLowerCase("uk").includes(search),
    ),
  );

  return (
    <div className="dashboard-shell" id="dashboard-top">
      <a className="dashboard-skip" href="#dashboard-content">
        До вмісту
      </a>
      <aside className="dashboard-sidebar" aria-label="Бічна панель">
        <div className="dashboard-sidebar__brand">
          <Logo variant="app" />
        </div>
        <nav className="dashboard-nav" aria-label="Основна навігація">
          {navigation.map((item, index) =>
            index === 0 ? (
              <Link
                className="dashboard-nav-item dashboard-nav-item--active"
                href="/main"
                aria-current="page"
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
                onClick={() => setModal({ title: item.label })}
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
          <button
            className="dashboard-nav-item"
            type="button"
            onClick={() => setModal({ title: "Налаштування" })}
            aria-haspopup="dialog"
            aria-label="Налаштування"
          >
            <Icon name="settings" />
            <span>Налаштування</span>
          </button>
          <button
            className="dashboard-nav-item"
            type="button"
            onClick={() => void logout()}
            disabled={loggingOut}
            aria-label={loggingOut ? "Виходимо…" : "Вийти"}
          >
            <Icon name="logout" />
            <span>{loggingOut ? "Виходимо…" : "Вийти"}</span>
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
            <h1>Головна</h1>
            <p>Твоя активність за тиждень</p>
          </div>
          <label className="dashboard-search">
            <Icon name="search" />
            <span className="sr-only">Пошук серед рекомендованих збігів</span>
            <input
              type="search"
              placeholder="Пошук навички або людини…"
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              aria-controls="dashboard-match-list"
            />
          </label>
          <a
            className="dashboard-notifications"
            href="#dashboard-requests"
            aria-label="Переглянути запити"
          >
            <Icon name="bell" />
          </a>
          <div className="dashboard-user">
            <DashboardAvatar initials={initials || "СС"} />
            <div className="dashboard-user__identity">
              <p title={name}>{name}</p>
              <span title={university}>{university}</span>
            </div>
            <Icon name="chevron-down" />
          </div>
        </header>
        <main className="dashboard-content" id="dashboard-content" tabIndex={-1}>
          <section className="dashboard-hero" aria-labelledby="dashboard-greeting">
            <span className="dashboard-hero__icon">
              <Icon name="hero-sparkles" />
            </span>
            <div>
              <h2 id="dashboard-greeting">
                {profile.firstName ? `Привіт, ${profile.firstName}!` : "Вітаємо у SkillSwap!"}
              </h2>
              <p>Додай навички у своєму профілі, щоб підготуватися до обміну знаннями.</p>
            </div>
            <Link className="dashboard-hero__action" href="/onboarding">
              Заповнити профіль
              <Icon name="arrow-right" />
            </Link>
          </section>
          <DashboardStats />
          <div className="dashboard-columns">
            <div className="dashboard-left">
              <section
                id="dashboard-matches"
                className="dashboard-matches"
                aria-labelledby="matches-title"
              >
                <div className="dashboard-section-heading">
                  <h2 id="matches-title">Рекомендовані збіги</h2>
                  <button
                    className="dashboard-section-action"
                    type="button"
                    onClick={() => setModal({ title: "Усі збіги" })}
                    aria-haspopup="dialog"
                  >
                    Усі збіги
                    <Icon name="chevron-right" />
                  </button>
                </div>
                <div className="dashboard-match-grid" id="dashboard-match-list">
                  {matches.map((match) => (
                    <DashboardMatchCard match={match} key={match.id} onOpenModal={setModal} />
                  ))}
                  {matches.length === 0 && (
                    <DashboardEmptyState
                      icon="nav-sparkles"
                      message={
                        search
                          ? `Збігів за запитом «${query.trim()}» не знайдено.`
                          : "Тут з’являться рекомендовані збіги, коли будуть доступні дані."
                      }
                    />
                  )}
                </div>
                <span className="sr-only" role="status">
                  {search ? `Знайдено збігів: ${matches.length}` : ""}
                </span>
              </section>
              <DashboardExchanges onOpenModal={setModal} />
            </div>
            <div className="dashboard-right">
              <DashboardSessions onOpenModal={setModal} />
              <DashboardRequests onOpenModal={setModal} />
            </div>
          </div>
        </main>
      </div>
      {modal && <DashboardModal {...modal} onClose={() => setModal(null)} />}
    </div>
  );
}
