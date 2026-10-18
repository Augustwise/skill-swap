"use client";

import Link from "next/link";
import { DashboardEmptyState } from "./DashboardCards";
import { DiscoveryError } from "./DiscoveryError";
import { listSchedule } from "./ScheduleApi";
import { sessionHref } from "./SessionApi";
import { sessionTime } from "./SessionTime";
import { SkeletonPanelRows } from "./Skeleton";
import { useApiQuery } from "./useApiQuery";

export function UpcomingLessons() {
  const result = useApiQuery("upcoming-lessons", (signal) =>
    listSchedule({ status: "SCHEDULED", upcoming: true, page: 1 }, signal),
  );
  const data = result.state.status === "ready" ? result.state.data : null;
  return (
    <section className="dashboard-panel" aria-labelledby="upcoming-lessons">
      <div className="dashboard-section-heading">
        <h2 id="upcoming-lessons">Найближчі заняття</h2>
        <Link className="dashboard-section-action" href="/schedule">
          Розклад →
        </Link>
      </div>
      {result.state.status === "loading" && <SkeletonPanelRows count={2} variant="exchange" />}
      {result.state.status === "error" && (
        <DiscoveryError error={result.state.error} path="/main" onRetry={result.retry} />
      )}
      {data &&
        (data.items.length ? (
          <ul className="session-list">
            {data.items.slice(0, 3).map((lesson) => (
              <li key={lesson.id}>
                <Link className="exchange-person__link" href={sessionHref(lesson.id)}>
                  {lesson.title}
                </Link>
                {lesson.startsAt && (
                  <p className="profile-hint">
                    <time dateTime={lesson.startsAt}>
                      {sessionTime(lesson.startsAt, data.timezone)}
                    </time>
                  </p>
                )}
              </li>
            ))}
          </ul>
        ) : (
          <DashboardEmptyState
            icon="calendar"
            message="Погодьте час у плані активного обміну — заняття з’явиться тут."
          />
        ))}
    </section>
  );
}
