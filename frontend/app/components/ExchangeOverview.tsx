"use client";

import Link from "next/link";
import { useState } from "react";
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
import { ExchangeCompletion } from "./ExchangeCompletion";
import {
  cancelExchange,
  completeExchange,
  completionMessage,
  exchangesHref,
} from "./CompletionApi";
import { ExchangeCancellation } from "./ExchangeCancellation";
import { ExchangeHistory } from "./ExchangeHistory";
import { ExchangeReviews } from "./ExchangeReviews";
import { CompletionFeedback } from "./CompletionFeedback";
import { useCompletionMutation } from "./useCompletionMutation";
import { exchangeSessions, sessionHref } from "./SessionApi";
import { meetingLink } from "./SessionTime";
import { exchangeHref, requestHref, requestPerspective, type Exchange } from "./ExchangeApi";
import { formatDateTime, sessionForms, totalTime } from "./ExchangeFormat";
import { SkeletonDetail } from "./Skeleton";
import { useApiQuery } from "./useApiQuery";
import { StudentSafety } from "./StudentSafety";

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
      {progress.awaitingConfirmationSessions > 0 && (
        <p className="completion-pending">
          Очікують підтвердження проведення: {progress.awaitingConfirmationSessions}. Відкрий
          заняття в плані, щоб підтвердити участь.
        </p>
      )}
      {progress.planCompleted && <p className="profile-success">План виконано в обох напрямках.</p>}
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
          <Link
            className="exchange-person__link"
            href={sessionHref(next.id, exchangeHref(exchange.id))}
          >
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
          {exchange.status === "ACTIVE"
            ? "Погоджених майбутніх занять немає. Перевір план і підтвердження проведених занять."
            : "Нових занять у цьому обміні не заплановано. Історія занять доступна в плані."}
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
  from,
}: AppContext & { exchangeId: string; started: boolean; from?: string }) {
  const path = exchangeHref(exchangeId, from);
  const result = useApiQuery(path, (signal) => exchangeSessions(exchangeId, signal));
  const data = result.state.status === "ready" ? result.state.data : null;
  const exchange = data?.exchange;
  const view = exchange ? requestPerspective(exchange, profile.id) : null;
  const completion = useCompletionMutation(result.retry);
  const [cancelling, setCancelling] = useState(false);
  const [safetyNotice, setSafetyNotice] = useState("");

  return (
    <AppShell
      profile={profile}
      csrfToken={session.csrfToken}
      section="exchanges"
      title={pageShells.exchange.title}
      subtitle={pageShells.exchange.subtitle}
      contentId="exchange-content"
    >
      {safetyNotice && (
        <p className="account-card profile-success" role="status">
          {safetyNotice}
        </p>
      )}
      <Link
        className="discovery-back"
        href={
          from ??
          exchangesHref(
            exchange?.status === "COMPLETED"
              ? "COMPLETED"
              : exchange?.status === "CANCELLED"
                ? "CANCELLED"
                : "ACTIVE",
          )
        }
      >
        ← До списку обмінів
      </Link>
      <CompletionFeedback error={completion.error} notice={completion.notice} path={path} />
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
      {data && exchange && view && (
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
              <StudentSafety
                student={exchange.partner}
                csrfToken={session.csrfToken}
                path={path}
                onChanged={(notice) => {
                  setSafetyNotice(notice);
                  result.retry();
                }}
              />
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
            <ExchangeLessons exchange={exchange} lessons={data.items} onRefresh={result.retry} />
            <ExchangeHistory exchange={exchange} />
            {exchange.status === "COMPLETED" && (
              <ExchangeReviews
                exchange={exchange}
                csrfToken={session.csrfToken}
                path={path}
                onRefresh={result.retry}
              />
            )}
          </div>
          <aside className="profile-rail" aria-label="Прогрес і заняття">
            <ProgressCard exchange={exchange} viewerId={profile.id} />
            <ExchangeCompletion
              exchange={exchange}
              pending={completion.pending}
              onConfirm={() =>
                void completion.run(
                  () => completeExchange(exchange.id, session.csrfToken),
                  (data) => completionMessage(data.exchange, "exchange"),
                )
              }
            />
            {exchange.canCancel && (
              <section className="account-card">
                <h2>Скасування обміну</h2>
                <p className="profile-hint">
                  Якщо продовжити навчання не виходить, поясни причину партнеру.
                </p>
                <button
                  className="dashboard-button completion-danger"
                  disabled={completion.pending}
                  onClick={() => setCancelling(true)}
                >
                  Скасувати обмін
                </button>
              </section>
            )}
            <ChatCard
              queryKey={`${path}/conversation`}
              load={(signal) => exchangeConversation(exchangeId, signal)}
              viewerId={profile.id}
            />
            <NextStepsCard exchange={exchange} />
          </aside>
        </div>
      )}
      {cancelling && exchange?.canCancel && (
        <ExchangeCancellation
          pending={completion.pending}
          error={completion.error}
          path={path}
          onClose={() => setCancelling(false)}
          onSubmit={(reason) =>
            void completion
              .run(
                () => cancelExchange(exchange.id, reason, session.csrfToken),
                "Обмін скасовано. Непроведені заняття скасовано, історію збережено.",
              )
              .then((saved) => {
                if (saved) setCancelling(false);
              })
          }
        />
      )}
    </AppShell>
  );
}

export function ExchangeOverview({
  exchangeId,
  started,
  from,
}: {
  exchangeId: string;
  started: boolean;
  from?: string;
}) {
  return (
    <AppGate page="exchange" path={exchangeHref(exchangeId, from)}>
      {(context) => (
        <ExchangeView {...context} exchangeId={exchangeId} started={started} from={from} />
      )}
    </AppGate>
  );
}
