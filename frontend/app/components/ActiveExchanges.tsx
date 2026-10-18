"use client";

import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { AppGate, type AppContext } from "./AppGate";
import { AppShell } from "./AppShell";
import { ExchangeRow } from "./DashboardActivity";
import { DiscoveryEmpty, Pagination } from "./DiscoveryCards";
import { DiscoveryError } from "./DiscoveryError";
import { pageCount, readPage } from "./DiscoveryApi";
import { listActiveExchanges } from "./ExchangeApi";
import { SkeletonCardGrid } from "./Skeleton";
import { useApiQuery } from "./useApiQuery";

function ExchangesView({ profile, session }: AppContext) {
  const params = useSearchParams();
  const page = readPage(params.get("page"));
  const path = page > 1 ? `/exchanges?page=${page}` : "/exchanges";
  const result = useApiQuery(path, (signal) => listActiveExchanges(page, signal));
  const data = result.state.status === "ready" ? result.state.data : null;
  return (
    <AppShell
      profile={profile}
      csrfToken={session.csrfToken}
      section="exchanges"
      title="Активні обміни"
      subtitle="Погоджені умови, прогрес і найближчі заняття"
    >
      {result.state.status === "loading" && <SkeletonCardGrid count={3} />}
      {result.state.status === "error" && (
        <DiscoveryError error={result.state.error} path={path} onRetry={result.retry} />
      )}
      {data && (
        <>
          <p className="profile-hint" role="status">
            Активних обмінів: {data.total}
          </p>
          {data.items.length ? (
            <ul className="session-list" aria-label="Активні обміни">
              {data.items.map((exchange) => (
                <li className="account-card" key={exchange.id}>
                  <ExchangeRow exchange={exchange} viewerId={profile.id} />
                  <Link className="dashboard-button" href={`/exchanges/${exchange.id}`}>
                    Умови та план занять →
                  </Link>
                </li>
              ))}
            </ul>
          ) : (
            <DiscoveryEmpty
              icon="nav-swap"
              title={page > 1 ? "На цій сторінці немає обмінів." : "Активних обмінів поки немає."}
            >
              <p>Обмін з’явиться після прийняття запиту.</p>
              <Link
                className="dashboard-button dashboard-button--primary"
                href={page > 1 ? "/exchanges" : "/requests"}
              >
                {page > 1 ? "До першої сторінки" : "Переглянути запити"}
              </Link>
            </DiscoveryEmpty>
          )}
          <Pagination
            page={page}
            pages={pageCount(data.total, data.pageSize)}
            hrefFor={(next) => `/exchanges?page=${next}`}
          />
        </>
      )}
    </AppShell>
  );
}

export function ActiveExchanges() {
  return (
    <AppGate page="exchanges" path="/exchanges">
      {(context) => <ExchangesView {...context} />}
    </AppGate>
  );
}
