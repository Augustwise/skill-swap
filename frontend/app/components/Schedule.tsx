"use client";

import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { AppGate, type AppContext } from "./AppGate";
import { AppShell } from "./AppShell";
import { DiscoveryEmpty, Pagination } from "./DiscoveryCards";
import { DiscoveryError } from "./DiscoveryError";
import { pageCount } from "./DiscoveryApi";
import { fullName } from "./DiscoveryFormat";
import {
  listSchedule,
  readScheduleFilters,
  scheduleHref,
  scheduleLabels,
  scheduleStatuses,
  type ScheduleFilters,
} from "./ScheduleApi";
import { sessionHref } from "./SessionApi";
import { SessionFacts } from "./SessionFacts";
import { SkeletonPanelRows } from "./Skeleton";
import { useApiQuery } from "./useApiQuery";

function ScheduleView({ profile, session, filters }: AppContext & { filters: ScheduleFilters }) {
  const path = scheduleHref(filters);
  const result = useApiQuery(path, (signal) => listSchedule(filters, signal));
  const data = result.state.status === "ready" ? result.state.data : null;
  return (
    <AppShell
      profile={profile}
      csrfToken={session.csrfToken}
      section="schedule"
      title="Розклад"
      subtitle="Твої зустрічі у хронологічному порядку"
    >
      <nav className="exchange-tabs" aria-label="Статус занять">
        {scheduleStatuses.map((status) => (
          <Link
            className="exchange-tab"
            aria-current={filters.status === status ? "page" : undefined}
            key={status}
            href={scheduleHref({
              status,
              upcoming: ["SCHEDULED", "PROPOSED"].includes(status),
              page: 1,
            })}
          >
            {scheduleLabels[status]}
          </Link>
        ))}
      </nav>
      <div className="session-actions">
        <Link
          className="dashboard-button"
          href={scheduleHref({ ...filters, upcoming: !filters.upcoming, page: 1 })}
        >
          {filters.upcoming ? "Показати також минулі" : "Лише майбутні та поточні"}
        </Link>
        <button className="dashboard-button" onClick={result.retry}>
          Оновити розклад
        </button>
      </div>
      {data && (
        <p className="profile-hint" role="status">
          Часовий пояс: {data.timezone} · Занять: {data.total} ·{" "}
          {filters.upcoming ? "Майбутні та поточні" : "Усі дати"}
        </p>
      )}
      {data && data.pendingProposalsCount > 0 && (
        <p className="session-agreement">
          <Link href={scheduleHref({ status: "PROPOSED", upcoming: false, page: 1 })}>
            Пропозиції, що потребують твоєї відповіді: {data.pendingProposalsCount} →
          </Link>
        </p>
      )}
      {result.state.status === "loading" && (
        <div role="status" aria-label="Завантажуємо розклад">
          <SkeletonPanelRows count={3} variant="exchange" />
        </div>
      )}
      {result.state.status === "error" && (
        <DiscoveryError error={result.state.error} path={path} onRetry={result.retry} />
      )}
      {data && (
        <>
          {data.items.length ? (
            <ol className="session-list" aria-label="Заняття за часом">
              {data.items.map((lesson) => (
                <li className="account-card session-detail" key={lesson.id}>
                  <h2>
                    <Link className="exchange-person__link" href={sessionHref(lesson.id, path)}>
                      {lesson.title}
                    </Link>
                  </h2>
                  <p className="profile-hint">Партнер: {fullName(lesson.partner)}</p>
                  <SessionFacts lesson={lesson} timezone={data.timezone} />
                  <Link className="dashboard-button" href={sessionHref(lesson.id, path)}>
                    {lesson.canRespondToProposal ? "Переглянути пропозицію" : "Деталі заняття"} →
                  </Link>
                </li>
              ))}
            </ol>
          ) : (
            <DiscoveryEmpty icon="calendar" title="У розкладі поки немає занять.">
              <p>
                Для цих фільтрів нічого не знайдено. Запропонуй дату в активному обміні або
                переглянь інші статуси.
              </p>
              <Link
                className="dashboard-button dashboard-button--primary"
                href={filters.page > 1 ? scheduleHref({ ...filters, page: 1 }) : "/exchanges"}
              >
                {filters.page > 1 ? "До першої сторінки" : "До активних обмінів"}
              </Link>
            </DiscoveryEmpty>
          )}
          <Pagination
            page={filters.page}
            pages={pageCount(data.total, data.pageSize)}
            hrefFor={(page) => scheduleHref({ ...filters, page })}
          />
        </>
      )}
    </AppShell>
  );
}

export function Schedule() {
  const params = useSearchParams();
  const filters = readScheduleFilters(params);
  return (
    <AppGate page="schedule" path={scheduleHref(filters)}>
      {(context) => <ScheduleView {...context} filters={filters} />}
    </AppGate>
  );
}
