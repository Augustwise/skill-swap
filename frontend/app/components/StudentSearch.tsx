"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { AppGate, type AppContext } from "./AppGate";
import { AppShell } from "./AppShell";
import { DiscoveryEmpty, Pagination, StudentResultCard } from "./DiscoveryCards";
import { DiscoveryError } from "./DiscoveryError";
import {
  eligibleForMatching,
  pageCount,
  readSearchFilters,
  searchQueryString,
  searchStudents,
  skillCategories,
  type SearchFilters as Filters,
} from "./DiscoveryApi";
import { countLabel, studentForms } from "./DiscoveryFormat";
import { SearchFilters } from "./SearchFilters";
import { useApiQuery } from "./useApiQuery";

const searchHref = (filters: Filters) => {
  const query = searchQueryString(filters);
  return query ? `/search?${query}` : "/search";
};

function SearchResults({ profile, session, filters }: AppContext & { filters: Filters }) {
  const router = useRouter();
  const path = searchHref(filters);
  const results = useApiQuery(path, (signal) => searchStudents(filters, signal));
  const categories = useApiQuery("skill-categories", skillCategories);
  const showEligibility = filters.mutual && !eligibleForMatching(profile);

  let summary = "Шукаємо студентів…";
  if (results.state.status === "ready") {
    const { total } = results.state.data;
    summary = `Знайдено ${countLabel(total, studentForms)}${filters.q ? ` за запитом «${filters.q}»` : ""}`;
  } else if (results.state.status === "error") summary = "Не вдалося виконати пошук";

  return (
    <AppShell
      profile={profile}
      csrfToken={session.csrfToken}
      section="search"
      title="Пошук навичок"
      subtitle="Знайди студента за навичкою або ім’ям"
      contentId="search-content"
      search={false}
    >
      <SearchFilters
        key={path}
        filters={filters}
        categories={categories.state}
        onRetryCategories={categories.retry}
        onApply={(next) => router.push(searchHref({ ...next, page: 1 }), { scroll: false })}
      />
      {showEligibility && (
        <div className="account-card profile-tip discovery-notice">
          <p>
            Взаємні збіги з’являться, коли в профілі будуть обидва списки навичок і формат занять
            (для офлайну — ще й місто).
          </p>
          <Link className="dashboard-button" href="/profile">
            Заповнити профіль
          </Link>
        </div>
      )}
      <section
        className="discovery-results"
        aria-labelledby="search-results-title"
        aria-busy={results.state.status === "loading"}
      >
        <div className="dashboard-section-heading">
          <h2 id="search-results-title">Результати</h2>
          <p className="discovery-summary" role="status">
            {summary}
          </p>
        </div>
        {results.state.status === "loading" && <div className="discovery-skeleton" />}
        {results.state.status === "error" && (
          <DiscoveryError
            error={results.state.error}
            path={path}
            onRetry={results.retry}
            resetHref="/search"
          />
        )}
        {results.state.status === "ready" &&
          (results.state.data.items.length > 0 ? (
            <>
              <ul className="discovery-grid">
                {results.state.data.items.map((card) => (
                  <StudentResultCard card={card} from={path} key={card.student.id} />
                ))}
              </ul>
              <Pagination
                page={filters.page}
                pages={pageCount(results.state.data.total, results.state.data.pageSize)}
                hrefFor={(page) => searchHref({ ...filters, page })}
              />
            </>
          ) : results.state.data.total > 0 ? (
            <DiscoveryEmpty icon="nav-search" title="На цій сторінці результатів немає.">
              <Link className="dashboard-button" href={searchHref({ ...filters, page: 1 })}>
                До першої сторінки
              </Link>
            </DiscoveryEmpty>
          ) : (
            <DiscoveryEmpty
              icon="nav-search"
              title={
                filters.q
                  ? `За запитом «${filters.q}» нікого не знайдено.`
                  : "За обраними фільтрами нікого не знайдено."
              }
            >
              <p>Спробуй іншу назву навички, ім’я або зміни фільтри.</p>
              {searchQueryString({ ...filters, page: 1 }) && (
                <Link className="dashboard-button" href="/search">
                  Скинути фільтри
                </Link>
              )}
            </DiscoveryEmpty>
          ))}
      </section>
    </AppShell>
  );
}

export function StudentSearch() {
  const filters = readSearchFilters(useSearchParams());
  return (
    <AppGate subject="пошук" path={searchHref(filters)}>
      {(context) => <SearchResults {...context} filters={filters} />}
    </AppGate>
  );
}
