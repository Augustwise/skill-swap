"use client";

import Link from "next/link";
import { AppGate, type AppContext } from "./AppGate";
import { pageShells } from "./AppNavigation";
import { AppShell } from "./AppShell";
import { exchangeConversation } from "./ChatApi";
import { ChatCard } from "./ChatCard";
import { DashboardIcon as Icon } from "./DashboardIcon";
import { DiscoveryEmpty } from "./DiscoveryCards";
import { DiscoveryError } from "./DiscoveryError";
import { countLabel, formatList } from "./DiscoveryFormat";
import { ExchangeDirections, PartnerHeading, StatusBadge } from "./ExchangeCards";
import { ExchangeLessons } from "./ExchangeLessons";
import { sessionHref } from "./SessionApi";
import { meetingLink } from "./SessionTime";
import {
  exchangeDetails,
  exchangeHref,
  requestHref,
  requestPerspective,
  type Exchange,
} from "./ExchangeApi";
import { formatDateTime, sessionForms, totalTime } from "./ExchangeFormat";
import { SkeletonDetail } from "./Skeleton";
import { useApiQuery } from "./useApiQuery";

function ProgressCard({ exchange, viewerId }: { exchange: Exchange; viewerId: string }) {
  const { progress } = exchange;
  const youRequested = exchange.requester.id === viewerId;
  const rows = [
    {
      label: "Ти навчаєш",
      done: youRequested
        ? progress.requesterCompletedSessions
        : progress.recipientCompletedSessions,
      planned: youRequested ? progress.requesterPlannedSessions : progress.recipientPlannedSessions,
    },
    {
      label: `${exchange.partner.firstName} навчає`,
      done: youRequested
        ? progress.recipientCompletedSessions
        : progress.requesterCompletedSessions,
      planned: youRequested ? progress.recipientPlannedSessions : progress.requesterPlannedSessions,
    },
  ];
  return (
    <section className="account-card" aria-labelledby="exchange-progress-title">
      <div className="dashboard-section-heading">
        <h2 id="exchange-progress-title">Прогрес</h2>
        <strong className="exchange-progress__percent">{progress.percent}%</strong>
      </div>
      <progress
        className="exchange-progress"
        value={progress.completedSessions}
        max={Math.max(progress.totalSessions, 1)}
        aria-label={`Проведено ${progress.completedSessions} з ${countLabel(progress.totalSessions, sessionForms)}`}
      />
      <p className="profile-hint">
        Проведено {progress.completedSessions} з {countLabel(progress.totalSessions, sessionForms)}.
        Заняття зараховується, коли обидва учасники підтвердять, що воно відбулося.
      </p>
      <dl className="exchange-facts exchange-facts--compact">
        {rows.map((row) => (
          <div key={row.label}>
            <dt>{row.label}</dt>
            <dd>
              {row.done} з {countLabel(row.planned, sessionForms)}
            </dd>
          </div>
        ))}
      </dl>
    </section>
  );
}

function NextStepsCard({ exchange }: { exchange: Exchange }) {
  const next = exchange.nextSession;
  return (
    <section className="account-card" aria-labelledby="exchange-next-title">
      <h2 id="exchange-next-title">Найближче заняття</h2>
      {next ? (
        <div className="exchange-next">
          <Link className="exchange-person__link" href={sessionHref(next.id)}>
            <strong>{next.title}</strong>
          </Link>
          <p>
            <Icon name="clock" />
            <time dateTime={next.startsAt}>{formatDateTime(next.startsAt)}</time>
          </p>
          {next.meetingPlace && (
            <p>
              <Icon name="pin" />
              {next.meetingPlace}
            </p>
          )}
          {meetingLink(next.meetingUrl) && (
            <p>
              <Icon name="video" />
              <a href={meetingLink(next.meetingUrl)!} target="_blank" rel="noopener noreferrer">
                Посилання на зустріч
              </a>
            </p>
          )}
        </div>
      ) : (
        <p className="profile-hint">
          Погоджених майбутніх занять немає. Обери заняття в плані та запропонуй час партнеру.
        </p>
      )}
    </section>
  );
}

function ExchangeView({
  profile,
  session,
  exchangeId,
  started,
}: AppContext & { exchangeId: string; started: boolean }) {
  const path = exchangeHref(exchangeId);
  const result = useApiQuery(path, (signal) => exchangeDetails(exchangeId, signal));
  const exchange = result.state.status === "ready" ? result.state.data : null;
  const view = exchange ? requestPerspective(exchange, profile.id) : null;

  return (
    <AppShell
      profile={profile}
      csrfToken={session.csrfToken}
      section="exchanges"
      title={pageShells.exchange.title}
      subtitle={pageShells.exchange.subtitle}
      contentId="exchange-content"
    >
      <Link className="discovery-back" href="/exchanges">
        ← До активних обмінів
      </Link>
      {started && exchange?.status === "ACTIVE" && (
        <section className="account-card profile-tip exchange-started" role="status">
          <span className="exchange-started__icon" aria-hidden="true">
            <Icon name="handshake" />
          </span>
          <div>
            <h2>Обмін розпочато!</h2>
            <p>
              Умови збережено, а чат запиту тепер належить обміну разом з усією історією. Далі ви
              разом погоджуєте дати занять — кожне заняття зараховується після підтвердження обох
              учасників.
            </p>
          </div>
        </section>
      )}
      {result.state.status === "loading" && (
        <>
          <p className="sr-only" role="status">
            Завантажуємо обмін…
          </p>
          <SkeletonDetail />
        </>
      )}
      {result.state.status === "error" && (
        <DiscoveryError
          error={result.state.error}
          path={path}
          onRetry={result.retry}
          notFound={
            <DiscoveryEmpty icon="nav-swap" title="Обмін не знайдено.">
              <p>Можливо, посилання неправильне або ти не береш участі в цьому обміні.</p>
              <Link className="dashboard-button dashboard-button--primary" href="/requests">
                До запитів на обмін
              </Link>
            </DiscoveryEmpty>
          }
        />
      )}
      {exchange && view && (
        <div className="profile-columns">
          <div className="profile-sections">
            <section className="account-card" aria-labelledby="exchange-partner">
              <p className="exchange-card__kind">
                Обмін
                {exchange.startedAt && (
                  <>
                    {" "}
                    · розпочато{" "}
                    <time dateTime={exchange.startedAt}>{formatDateTime(exchange.startedAt)}</time>
                  </>
                )}
              </p>
              <div id="exchange-partner">
                <PartnerHeading
                  partner={exchange.partner}
                  from={path}
                  level={2}
                  badge={<StatusBadge status={exchange.status} kind="exchange" />}
                />
              </div>
            </section>
            <section className="account-card" aria-labelledby="exchange-terms-title">
              <h2 id="exchange-terms-title">Погоджені умови</h2>
              <ExchangeDirections
                youTeach={view.youTeach}
                partnerTeaches={view.partnerTeaches}
                partnerName={exchange.partner.firstName}
                detailed
              />
              <dl className="exchange-facts">
                <div>
                  <dt>Формат</dt>
                  <dd>{formatList([exchange.format], exchange.partner.city)}</dd>
                </div>
                <div>
                  <dt>Усього занять</dt>
                  <dd>{countLabel(exchange.totalSessions, sessionForms)}</dd>
                </div>
                <div>
                  <dt>Навчання в кожному напрямку</dt>
                  <dd>{totalTime(view.youTeach)}</dd>
                </div>
              </dl>
              <p className="profile-hint">
                Назви навичок і рівні зафіксовано під час прийняття запиту: зміни профілю не
                переписують погоджені умови.
              </p>
              {exchange.requestId && (
                <Link className="exchange-person__link" href={requestHref(exchange.requestId)}>
                  Початковий запит та історія →
                </Link>
              )}
            </section>
            {exchange.cancellationReason && (
              <section className="account-card" aria-labelledby="exchange-cancel-title">
                <h2 id="exchange-cancel-title">Причина скасування</h2>
                <p className="profile-biography">{exchange.cancellationReason}</p>
              </section>
            )}
            <ExchangeLessons exchangeId={exchangeId} />
          </div>
          <aside className="profile-rail" aria-label="Прогрес і заняття">
            <ProgressCard exchange={exchange} viewerId={profile.id} />
            <ChatCard
              queryKey={`${path}/conversation`}
              load={(signal) => exchangeConversation(exchangeId, signal)}
              viewerId={profile.id}
            />
            <NextStepsCard exchange={exchange} />
          </aside>
        </div>
      )}
    </AppShell>
  );
}

export function ExchangeOverview({
  exchangeId,
  started,
}: {
  exchangeId: string;
  started: boolean;
}) {
  return (
    <AppGate page="exchange" path={exchangeHref(exchangeId)}>
      {(context) => <ExchangeView {...context} exchangeId={exchangeId} started={started} />}
    </AppGate>
  );
}
