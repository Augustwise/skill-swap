"use client";

import Link from "next/link";
import { useState } from "react";
import { AppGate, type AppContext } from "./AppGate";
import { pageShells } from "./AppNavigation";
import { safeAppPath } from "./AppRoutes";
import { AppShell } from "./AppShell";
import { DashboardIcon as Icon } from "./DashboardIcon";
import { DiscoveryEmpty } from "./DiscoveryCards";
import { DiscoveryError } from "./DiscoveryError";
import { countLabel, formatList, fullName } from "./DiscoveryFormat";
import { ExchangeDirections, PartnerHeading, StatusBadge } from "./ExchangeCards";
import {
  exchangeHref,
  requestDetails,
  requestHref,
  requestPerspective,
  requestsHref,
  type RequestDetails,
} from "./ExchangeApi";
import {
  formatDateTime,
  historyLabels,
  sessionForms,
  statusTone,
  totalTime,
} from "./ExchangeFormat";
import { RequestActions } from "./RequestActions";
import { SkeletonDetail } from "./Skeleton";
import { useApiQuery } from "./useApiQuery";

function DecisionCard({
  details,
  viewerId,
  csrfToken,
  path,
  onChanged,
}: {
  details: RequestDetails;
  viewerId: string;
  csrfToken: string;
  path: string;
  onChanged: (notice: string) => void;
}) {
  const { request } = details;
  const view = requestPerspective(request, viewerId);
  const partner = view.partner.firstName;
  const answered = request.respondedAt ? formatDateTime(request.respondedAt) : "";
  let text: string;
  switch (request.status) {
    case "PENDING":
      text =
        view.role === "recipient"
          ? `${partner} чекає на твою відповідь. Після прийняття одразу розпочнеться активний обмін з цими умовами.`
          : `Запит очікує відповіді від ${partner}. Щоб змінити умови, відклич запит і надішли новий.`;
      break;
    case "ACCEPTED":
      text = `Запит прийнято${answered ? ` ${answered}` : ""}. Погоджені умови збережено в обміні.`;
      break;
    case "DECLINED":
      text = `Запит відхилено${answered ? ` ${answered}` : ""}. Згодом можна надіслати новий запит з іншими умовами.`;
      break;
    default:
      text = `Запит відкликано${answered ? ` ${answered}` : ""}. Він більше не очікує відповіді.`;
  }
  return (
    <section
      className={`account-card exchange-decision exchange-decision--${statusTone(request.status)}`}
      aria-labelledby="request-decision-title"
    >
      <h2 id="request-decision-title">
        {request.status === "PENDING" ? "Рішення" : "Статус запиту"}
      </h2>
      <StatusBadge status={request.status} />
      <p>{text}</p>
      <div className="exchange-decision__actions">
        <RequestActions
          request={request}
          viewerId={viewerId}
          csrfToken={csrfToken}
          path={path}
          onChanged={onChanged}
        />
        {request.exchangeId && (
          <Link
            className="dashboard-button dashboard-button--primary"
            href={exchangeHref(request.exchangeId)}
          >
            Перейти до обміну
            <Icon name="arrow-right" />
          </Link>
        )}
      </div>
    </section>
  );
}

function StatusHistory({ details, viewerId }: { details: RequestDetails; viewerId: string }) {
  return (
    <section className="account-card" aria-labelledby="request-history-title">
      <h2 id="request-history-title">Історія статусів</h2>
      <ol className="exchange-history">
        {details.history.map((change, index) => (
          <li
            className={`exchange-history__item exchange-history__item--${statusTone(change.status)}`}
            key={`${change.status}-${change.createdAt}-${index}`}
          >
            <strong>{historyLabels[change.status]}</strong>
            <span>
              {change.changedBy
                ? change.changedBy.id === viewerId
                  ? "Ти"
                  : fullName(change.changedBy)
                : "Система"}{" "}
              · <time dateTime={change.createdAt}>{formatDateTime(change.createdAt)}</time>
            </span>
          </li>
        ))}
      </ol>
    </section>
  );
}

function RequestView({
  profile,
  session,
  requestId,
  from,
}: AppContext & { requestId: string; from: string }) {
  const path = requestHref(requestId);
  const [version, setVersion] = useState(0);
  const [notice, setNotice] = useState("");
  const result = useApiQuery(`${path}#${version}`, (signal) => requestDetails(requestId, signal));
  const details = result.state.status === "ready" ? result.state.data : null;
  const view = details ? requestPerspective(details.request, profile.id) : null;
  const back = safeAppPath(
    from,
    requestsHref({ direction: view?.role === "requester" ? "outgoing" : "incoming" }),
  );

  function changed(text: string) {
    setNotice(text);
    setVersion((value) => value + 1);
  }

  return (
    <AppShell
      profile={profile}
      csrfToken={session.csrfToken}
      section="requests"
      title={pageShells.request.title}
      subtitle={pageShells.request.subtitle}
      contentId="request-content"
    >
      <Link className="discovery-back" href={back}>
        ← До запитів на обмін
      </Link>
      {notice && (
        <p className="account-card exchange-notice" role="status">
          {notice}
        </p>
      )}
      {result.state.status === "loading" && (
        <>
          <p className="sr-only" role="status">
            Завантажуємо запит…
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
            <DiscoveryEmpty icon="inbox" title="Запит не знайдено.">
              <p>Можливо, посилання неправильне або ти не береш участі в цьому запиті.</p>
              <Link className="dashboard-button dashboard-button--primary" href="/requests">
                До запитів на обмін
              </Link>
            </DiscoveryEmpty>
          }
        />
      )}
      {details && view && (
        <div className="profile-columns">
          <div className="profile-sections">
            <section className="account-card" aria-labelledby="request-partner">
              <p className="exchange-card__kind">
                {view.role === "recipient" ? "Вхідний запит" : "Надісланий запит"} ·{" "}
                <time dateTime={details.request.createdAt}>
                  {formatDateTime(details.request.createdAt)}
                </time>
              </p>
              <div id="request-partner">
                <PartnerHeading
                  partner={view.partner}
                  from={path}
                  level={2}
                  badge={<StatusBadge status={details.request.status} />}
                />
              </div>
            </section>
            <section className="account-card" aria-labelledby="request-terms-title">
              <h2 id="request-terms-title">Умови обміну</h2>
              <ExchangeDirections
                youTeach={view.youTeach}
                partnerTeaches={view.partnerTeaches}
                partnerName={view.partner.firstName}
                detailed
              />
              <dl className="exchange-facts">
                <div>
                  <dt>Формат</dt>
                  <dd>{formatList([details.request.format], view.partner.city)}</dd>
                </div>
                <div>
                  <dt>Усього занять</dt>
                  <dd>{countLabel(details.request.totalSessions, sessionForms)}</dd>
                </div>
                <div>
                  <dt>Навчання в кожному напрямку</dt>
                  <dd>{totalTime(view.youTeach)}</dd>
                </div>
              </dl>
              {details.request.status === "PENDING" && (
                <p className="profile-hint">
                  Рівні показано такими, як зараз у профілях. Після прийняття назви навичок і рівні
                  збережуться в обміні без змін.
                </p>
              )}
            </section>
            <section className="account-card" aria-labelledby="request-message-title">
              <h2 id="request-message-title">Повідомлення</h2>
              {details.request.message ? (
                <p className="profile-biography exchange-quote">{details.request.message}</p>
              ) : (
                <p className="profile-hint">Автор не додав повідомлення.</p>
              )}
            </section>
          </div>
          <aside className="profile-rail" aria-label="Рішення та історія">
            <DecisionCard
              details={details}
              viewerId={profile.id}
              csrfToken={session.csrfToken}
              path={path}
              onChanged={changed}
            />
            <StatusHistory details={details} viewerId={profile.id} />
          </aside>
        </div>
      )}
    </AppShell>
  );
}

export function RequestOverview({ requestId, from }: { requestId: string; from: string }) {
  return (
    <AppGate page="request" path={requestHref(requestId)}>
      {(context) => <RequestView {...context} requestId={requestId} from={from} />}
    </AppGate>
  );
}
