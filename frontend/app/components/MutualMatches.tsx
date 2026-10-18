"use client";

import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { AppGate, type AppContext } from "./AppGate";
import { pageShells } from "./AppNavigation";
import { AppShell } from "./AppShell";
import { DiscoveryEmpty, MatchCard, Pagination } from "./DiscoveryCards";
import { DiscoveryError } from "./DiscoveryError";
import { mutualMatches, pageCount, readPage } from "./DiscoveryApi";
import { countLabel, studentForms } from "./DiscoveryFormat";
import { SkeletonCardGrid } from "./Skeleton";
import { useApiQuery } from "./useApiQuery";

const matchesHref = (page: number) => (page > 1 ? `/matches?page=${page}` : "/matches");

function MatchList({ profile, session, page }: AppContext & { page: number }) {
  const path = matchesHref(page);
  const matches = useApiQuery(path, (signal) => mutualMatches(page, signal));
  const data = matches.state.status === "ready" ? matches.state.data : null;

  return (
    <AppShell
      profile={profile}
      csrfToken={session.csrfToken}
      section="matches"
      title={pageShells.matches.title}
      subtitle={pageShells.matches.subtitle}
      contentId="matches-content"
    >
      <section className="account-card profile-tip discovery-notice" aria-label="Як працює підбір">
        <p>
          Взаємний збіг — коли студент навчає навички з твого списку «Вивчаю», хоче вивчити навичку
          з твого списку «Навчаю», і у вас є спільний формат занять. Офлайн зараховується лише для
          одного міста. Список щоразу формується з актуальних навичок.
        </p>
      </section>
      <section
        className="discovery-results"
        aria-labelledby="matches-title"
        aria-busy={matches.state.status === "loading"}
      >
        <div className="dashboard-section-heading">
          <h2 id="matches-title">Взаємні збіги</h2>
          <p className="discovery-summary" role="status">
            {matches.state.status === "loading"
              ? "Шукаємо збіги…"
              : data
                ? data.eligibleForMatching
                  ? `Знайдено ${countLabel(data.total, studentForms)}`
                  : "Профіль ще не готовий до підбору"
                : "Не вдалося завантажити збіги"}
          </p>
        </div>
        {matches.state.status === "loading" && <SkeletonCardGrid count={4} />}
        {matches.state.status === "error" && (
          <DiscoveryError error={matches.state.error} path={path} onRetry={matches.retry} />
        )}
        {data &&
          (!data.eligibleForMatching ? (
            <DiscoveryEmpty icon="nav-sparkles" title="Заповни профіль, щоб бачити взаємні збіги.">
              <p>
                Потрібні обидва списки навичок — «Навчаю» і «Вивчаю» — та формат занять. Для
                офлайн-занять вкажи місто.
              </p>
              <Link className="dashboard-button dashboard-button--primary" href="/profile">
                Перейти до профілю
              </Link>
            </DiscoveryEmpty>
          ) : data.items.length > 0 ? (
            <>
              <ul className="discovery-grid">
                {data.items.map((match) => (
                  <MatchCard match={match} from={path} key={match.student.id} />
                ))}
              </ul>
              <Pagination
                page={page}
                pages={pageCount(data.total, data.pageSize)}
                hrefFor={matchesHref}
              />
            </>
          ) : data.total > 0 ? (
            <DiscoveryEmpty icon="nav-sparkles" title="На цій сторінці збігів немає.">
              <Link className="dashboard-button" href="/matches">
                До першої сторінки
              </Link>
            </DiscoveryEmpty>
          ) : (
            <DiscoveryEmpty icon="nav-sparkles" title="Поки немає взаємних збігів.">
              <p>
                Додай більше навичок у профіль або знайди студентів через звичайний пошук —
                односторонній інтерес там теж видно.
              </p>
              <Link className="dashboard-button dashboard-button--primary" href="/search">
                Перейти до пошуку
              </Link>
            </DiscoveryEmpty>
          ))}
      </section>
    </AppShell>
  );
}

export function MutualMatches() {
  const page = readPage(useSearchParams().get("page"));
  return (
    <AppGate page="matches" path={matchesHref(page)}>
      {(context) => <MatchList {...context} page={page} />}
    </AppGate>
  );
}
