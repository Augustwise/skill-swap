"use client";

import Link from "next/link";
import { useState } from "react";
import { AppGate, type AppContext } from "./AppGate";
import { AppShell } from "./AppShell";
import { DashboardIcon as Icon } from "./DashboardIcon";
import { DashboardModal, type DashboardModalContent } from "./DashboardModal";
import {
  DashboardExchanges,
  DashboardEmptyState,
  DashboardRequests,
  DashboardSessions,
  DashboardStats,
} from "./DashboardCards";
import { MatchCard } from "./DiscoveryCards";
import { DiscoveryError } from "./DiscoveryError";
import { eligibleForMatching, mutualMatches } from "./DiscoveryApi";
import { useApiQuery } from "./useApiQuery";

const recommendedCount = 2;

function RecommendedMatches() {
  const matches = useApiQuery("me/matches", (signal) => mutualMatches(1, signal));
  const data = matches.state.status === "ready" ? matches.state.data : null;
  return (
    <section
      id="dashboard-matches"
      className="dashboard-matches"
      aria-labelledby="matches-title"
      aria-busy={matches.state.status === "loading"}
    >
      <div className="dashboard-section-heading">
        <h2 id="matches-title">Рекомендовані збіги</h2>
        <Link className="dashboard-section-action" href="/matches">
          {data && data.total > 0 ? `Усі збіги · ${data.total}` : "Усі збіги"}
          <Icon name="chevron-right" />
        </Link>
      </div>
      {matches.state.status === "loading" && <div className="discovery-skeleton" />}
      {matches.state.status === "error" && (
        <DiscoveryError error={matches.state.error} path="/main" onRetry={matches.retry} />
      )}
      {data &&
        (data.items.length > 0 ? (
          <ul className="dashboard-match-grid discovery-grid--plain">
            {data.items.slice(0, recommendedCount).map((match) => (
              <MatchCard match={match} from="/main" key={match.student.id} />
            ))}
          </ul>
        ) : (
          <DashboardEmptyState
            icon="nav-sparkles"
            message={
              data.eligibleForMatching
                ? "Поки немає взаємних збігів. Спробуй звичайний пошук — там видно й односторонній інтерес."
                : "Додай навички в обидва списки й обери формат занять, щоб бачити взаємні збіги."
            }
          />
        ))}
    </section>
  );
}

function DashboardContent({ profile, session }: AppContext) {
  const [modal, setModal] = useState<DashboardModalContent | null>(null);
  const ready = eligibleForMatching(profile);
  return (
    <AppShell
      profile={profile}
      csrfToken={session.csrfToken}
      section="main"
      title="Головна"
      subtitle="Твоя активність за тиждень"
      contentId="dashboard-content"
      toolbar={
        <a
          className="dashboard-notifications"
          href="#dashboard-requests"
          aria-label="Переглянути запити"
        >
          <Icon name="bell" />
        </a>
      }
    >
      <section className="dashboard-hero" aria-labelledby="dashboard-greeting">
        <span className="dashboard-hero__icon">
          <Icon name="hero-sparkles" />
        </span>
        <div>
          <h2 id="dashboard-greeting">
            {profile.firstName ? `Привіт, ${profile.firstName}!` : "Вітаємо у SkillSwap!"}
          </h2>
          <p>
            {ready
              ? "Переглянь взаємні збіги або знайди студента за потрібною навичкою."
              : "Додай навички у своєму профілі, щоб підготуватися до обміну знаннями."}
          </p>
        </div>
        <Link className="dashboard-hero__action" href={ready ? "/search" : "/profile"}>
          {ready ? "Знайти партнера" : "Заповнити профіль"}
          <Icon name="arrow-right" />
        </Link>
      </section>
      <DashboardStats />
      <div className="dashboard-columns">
        <div className="dashboard-left">
          <RecommendedMatches />
          <DashboardExchanges onOpenModal={setModal} />
        </div>
        <div className="dashboard-right">
          <DashboardSessions onOpenModal={setModal} />
          <DashboardRequests onOpenModal={setModal} />
        </div>
      </div>
      {modal && <DashboardModal {...modal} onClose={() => setModal(null)} />}
    </AppShell>
  );
}

export function Dashboard() {
  return (
    <AppGate subject="головну" path="/main">
      {(context) => <DashboardContent {...context} />}
    </AppGate>
  );
}
