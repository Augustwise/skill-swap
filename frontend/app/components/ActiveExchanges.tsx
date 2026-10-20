"use client";

import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { AppGate, type AppContext } from "./AppGate";
import { AppShell } from "./AppShell";
import { ExchangeRow } from "./DashboardActivity";
import { DiscoveryEmpty, Pagination } from "./DiscoveryCards";
import { DiscoveryError } from "./DiscoveryError";
import { pageCount } from "./DiscoveryApi";
import { exchangeHref } from "./ExchangeApi";
import {
  exchangeFilters,
  exchangesHref,
  listExchanges,
  readExchangeFilters,
} from "./CompletionApi";
import { SkeletonCardGrid } from "./Skeleton";
import { useApiQuery } from "./useApiQuery";

function ExchangesView({ profile, session }: AppContext) {
  const params = useSearchParams();
  const { page, status } = readExchangeFilters(params);
  const path = exchangesHref(status, page);
  const result = useApiQuery(path, (signal) => listExchanges(status, page, signal));
  const data = result.state.status === "ready" ? result.state.data : null;
  return (
    <AppShell
      profile={profile}
      csrfToken={session.csrfToken}
      section="exchanges"
      title="Мої обміни"
      subtitle="Активне навчання та історія завершених і скасованих обмінів"
    >
      <nav className="exchange-tabs" aria-label="Статус обмінів">
        {Object.entries(exchangeFilters).map(([value, label]) => (
          <Link
            key={value}
            className="exchange-tab"
            href={exchangesHref(value as typeof status)}
            aria-current={status === value ? "page" : undefined}
          >
            {label}
          </Link>
        ))}
      </nav>
      {result.state.status === "loading" && <SkeletonCardGrid count={3} />}
      {result.state.status === "error" && (
        <DiscoveryError error={result.state.error} path={path} onRetry={result.retry} />
      )}
      {data && (
        <>
          <p className="profile-hint" role="status">
            {exchangeFilters[status]} обміни: {data.total}
          </p>
          {data.items.length ? (
            <ul className="session-list" aria-label={`${exchangeFilters[status]} обміни`}>
              {data.items.map((exchange) => (
                <li className="account-card" key={exchange.id}>
                  <ExchangeRow exchange={exchange} viewerId={profile.id} />
                  <Link className="dashboard-button" href={exchangeHref(exchange.id, path)}>
                    Умови та план занять →
                  </Link>
                </li>
              ))}
            </ul>
          ) : (
            <DiscoveryEmpty
              icon="nav-swap"
              title={
                page > 1
                  ? "На цій сторінці немає обмінів."
                  : `${exchangeFilters[status]} обміни поки відсутні.`
              }
            >
              <p>
                {status === "ACTIVE"
                  ? "Обмін з’явиться після прийняття запиту."
                  : "Тут зберігатимуться обміни після завершення або скасування."}
              </p>
              <Link
                className="dashboard-button dashboard-button--primary"
                href={
                  page > 1
                    ? exchangesHref(status)
                    : status === "ACTIVE"
                      ? "/requests"
                      : "/exchanges"
                }
              >
                {page > 1
                  ? "До першої сторінки"
                  : status === "ACTIVE"
                    ? "Переглянути запити"
                    : "До активних обмінів"}
              </Link>
            </DiscoveryEmpty>
          )}
          <Pagination
            page={page}
            pages={pageCount(data.total, data.pageSize)}
            hrefFor={(next) => exchangesHref(status, next)}
          />
        </>
      )}
    </AppShell>
  );
}

export function ActiveExchanges() {
  const params = useSearchParams();
  const { status, page } = readExchangeFilters(params);
  return (
    <AppGate page="exchanges" path={exchangesHref(status, page)}>
      {(context) => <ExchangesView {...context} />}
    </AppGate>
  );
}
