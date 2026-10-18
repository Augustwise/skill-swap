"use client";

import Link from "next/link";
import { DiscoveryError } from "./DiscoveryError";
import { exchangeHref } from "./ExchangeApi";
import { exchangeSessions, sessionHref } from "./SessionApi";
import { SessionFacts } from "./SessionFacts";
import { SkeletonPanelRows } from "./Skeleton";
import { useApiQuery } from "./useApiQuery";

export function ExchangeLessons({ exchangeId }: { exchangeId: string }) {
  const result = useApiQuery(exchangeId, (signal) => exchangeSessions(exchangeId, signal));
  const data = result.state.status === "ready" ? result.state.data : null;
  return (
    <section
      className="account-card"
      aria-labelledby="exchange-lessons"
      aria-busy={result.state.status === "loading"}
    >
      <div className="dashboard-section-heading">
        <h2 id="exchange-lessons">План занять</h2>
        <button className="dashboard-button" onClick={result.retry}>
          Оновити
        </button>
      </div>
      {result.state.status === "loading" && <SkeletonPanelRows count={3} variant="exchange" />}
      {result.state.status === "error" && (
        <DiscoveryError
          error={result.state.error}
          path={exchangeHref(exchangeId)}
          onRetry={result.retry}
        />
      )}
      {data &&
        (data.items.length ? (
          <ol className="session-list session-plan">
            {data.items.map((lesson) => (
              <li key={lesson.id}>
                <span className="session-number" aria-label={`Заняття ${lesson.planOrder}`}>
                  {lesson.planOrder}
                </span>
                <div className="session-plan__body">
                  <h3>{lesson.title}</h3>
                  <SessionFacts lesson={lesson} />
                  <Link className="dashboard-button" href={sessionHref(lesson.id)}>
                    {lesson.status === "DRAFT" && data.exchange.status === "ACTIVE"
                      ? "Узгодити дату"
                      : "Деталі заняття"}{" "}
                    →
                  </Link>
                </div>
              </li>
            ))}
          </ol>
        ) : (
          <p className="profile-hint">У цьому обміні ще немає занять.</p>
        ))}
    </section>
  );
}
