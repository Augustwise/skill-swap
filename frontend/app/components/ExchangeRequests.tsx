"use client";

import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { useState } from "react";
import { AppGate, type AppContext } from "./AppGate";
import { pageShells } from "./AppNavigation";
import { AppShell } from "./AppShell";
import { DiscoveryEmpty, Pagination } from "./DiscoveryCards";
import { DiscoveryError } from "./DiscoveryError";
import { pageCount } from "./DiscoveryApi";
import { countLabel } from "./DiscoveryFormat";
import { RequestCard } from "./ExchangeCards";
import {
  listRequests,
  pendingRequestCount,
  readRequestFilters,
  requestStatuses,
  requestsHref,
  type RequestDirection,
  type RequestFilters,
} from "./ExchangeApi";
import { requestFilterLabels, requestStatusLabels } from "./ExchangeFormat";
import { RequestActions } from "./RequestActions";
import { SkeletonCardGrid } from "./Skeleton";
import { useApiQuery } from "./useApiQuery";

const requestForms = ["запит", "запити", "запитів"] as const;

const directions: Record<RequestDirection, { label: string; intro: string; empty: string }> = {
  incoming: {
    label: "Вхідні",
    intro:
      "Запити, які тобі надіслали інші студенти. Прийми запит, щоб розпочати обмін, або відхили його.",
    empty: "Вхідних запитів поки немає.",
  },
  outgoing: {
    label: "Надіслані",
    intro:
      "Твої пропозиції іншим студентам. Поки запит очікує відповіді, його можна відкликати, щоб надіслати інші умови.",
    empty: "Надісланих запитів поки немає.",
  },
};

function RequestTabs({ filters, version }: { filters: RequestFilters; version: number }) {
  const counts = useApiQuery(`requests/pending#${version}`, (signal) =>
    Promise.all([pendingRequestCount("incoming", signal), pendingRequestCount("outgoing", signal)]),
  );
  return (
    <nav className="exchange-tabs" aria-label="Напрямок запитів">
      {(["incoming", "outgoing"] as const).map((direction, index) => {
        const count = counts.state.status === "ready" ? counts.state.data[index] : 0;
        return (
          <Link
            className="exchange-tab"
            href={requestsHref({ direction })}
            aria-current={filters.direction === direction ? "page" : undefined}
            key={direction}
          >
            {directions[direction].label}
            {count > 0 && (
              <span className="exchange-tab__count">
                <span className="sr-only">, очікують відповіді: </span>
                {count}
              </span>
            )}
          </Link>
        );
      })}
    </nav>
  );
}

function RequestList({ profile, session, filters }: AppContext & { filters: RequestFilters }) {
  const path = requestsHref(filters);
  const [version, setVersion] = useState(0);
  const [notice, setNotice] = useState({ path, text: "" });
  const requests = useApiQuery(`${path}#${version}`, (signal) => listRequests(filters, signal));
  const data = requests.state.status === "ready" ? requests.state.data : null;
  const direction = directions[filters.direction];

  function changed(text: string) {
    setNotice({ path, text });
    setVersion((value) => value + 1);
  }

  return (
    <AppShell
      profile={profile}
      csrfToken={session.csrfToken}
      section="requests"
      title={pageShells.requests.title}
      subtitle={pageShells.requests.subtitle}
      contentId="requests-content"
    >
      <div className="exchange-toolbar">
        <RequestTabs filters={filters} version={version} />
        <p className="exchange-intro">{direction.intro}</p>
        <div className="exchange-filters" role="group" aria-label="Статус запиту">
          {(["", ...requestStatuses] as const).map((status) => (
            <Link
              className="exchange-filter"
              href={requestsHref({ direction: filters.direction, status })}
              aria-current={filters.status === status ? "true" : undefined}
              key={status || "all"}
            >
              {requestFilterLabels[status]}
            </Link>
          ))}
        </div>
      </div>
      {notice.path === path && notice.text && (
        <p className="account-card exchange-notice" role="status">
          {notice.text}
        </p>
      )}
      <section
        className="discovery-results"
        aria-labelledby="requests-title"
        aria-busy={requests.state.status === "loading"}
      >
        <div className="dashboard-section-heading">
          <h2 id="requests-title">
            {direction.label}
            {filters.status && ` · ${requestFilterLabels[filters.status].toLocaleLowerCase("uk")}`}
          </h2>
          <p className="discovery-summary" role="status">
            {requests.state.status === "loading"
              ? "Завантажуємо запити…"
              : data
                ? `Знайдено ${countLabel(data.total, requestForms)}`
                : "Не вдалося завантажити запити"}
          </p>
        </div>
        {requests.state.status === "loading" && <SkeletonCardGrid count={4} variant="request" />}
        {requests.state.status === "error" && (
          <DiscoveryError
            error={requests.state.error}
            path={path}
            onRetry={requests.retry}
            resetHref="/requests"
          />
        )}
        {data &&
          (data.items.length > 0 ? (
            <>
              <ul className="discovery-grid exchange-grid">
                {data.items.map((request) => (
                  <RequestCard request={request} viewerId={profile.id} from={path} key={request.id}>
                    <RequestActions
                      request={request}
                      viewerId={profile.id}
                      csrfToken={session.csrfToken}
                      path={path}
                      onChanged={changed}
                    />
                  </RequestCard>
                ))}
              </ul>
              <Pagination
                page={filters.page}
                pages={pageCount(data.total, data.pageSize)}
                hrefFor={(page) => requestsHref({ ...filters, page })}
              />
            </>
          ) : data.total > 0 ? (
            <DiscoveryEmpty icon="inbox" title="На цій сторінці запитів немає.">
              <Link className="dashboard-button" href={requestsHref({ ...filters, page: 1 })}>
                До першої сторінки
              </Link>
            </DiscoveryEmpty>
          ) : filters.status ? (
            <DiscoveryEmpty
              icon="inbox"
              title={`Немає запитів зі статусом «${requestStatusLabels[filters.status]}».`}
            >
              <Link
                className="dashboard-button"
                href={requestsHref({ direction: filters.direction })}
              >
                Показати всі
              </Link>
            </DiscoveryEmpty>
          ) : (
            <DiscoveryEmpty icon="inbox" title={direction.empty}>
              <p>
                {filters.direction === "incoming"
                  ? "Коли хтось запропонує тобі обмін, запит з’явиться тут."
                  : "Відкрий профіль студента зі взаємних збігів і запропонуй обмін."}
              </p>
              <Link className="dashboard-button dashboard-button--primary" href="/matches">
                Переглянути взаємні збіги
              </Link>
            </DiscoveryEmpty>
          ))}
      </section>
    </AppShell>
  );
}

export function ExchangeRequests() {
  const filters = readRequestFilters(useSearchParams());
  return (
    <AppGate page="requests" path={requestsHref(filters)}>
      {(context) => <RequestList {...context} filters={filters} />}
    </AppGate>
  );
}
