"use client";

import Link from "next/link";
import { useState } from "react";
import { DashboardAvatar, DashboardEmptyState, SectionAction } from "./DashboardCards";
import { DashboardIcon as Icon } from "./DashboardIcon";
import type { OpenDashboardModal } from "./DashboardModal";
import { DiscoveryError } from "./DiscoveryError";
import { avatarColor, countLabel, fullName, initials } from "./DiscoveryFormat";
import { SkillTag, StatusBadge } from "./ExchangeCards";
import {
  exchangeHref,
  listRequests,
  requestHref,
  requestPerspective,
  requestsHref,
  type Exchange,
  type ExchangePage,
} from "./ExchangeApi";
import { formatDateTime, formatRelative, sessionForms } from "./ExchangeFormat";
import { RequestActions } from "./RequestActions";
import { SkeletonPanelRows } from "./Skeleton";
import { useApiQuery, type QueryState } from "./useApiQuery";

const shownRequests = 3;
const shownExchanges = 2;

export function DashboardRequests({
  viewerId,
  csrfToken,
}: {
  viewerId: string;
  csrfToken: string;
}) {
  const [version, setVersion] = useState(0);
  const [notice, setNotice] = useState("");
  const requests = useApiQuery(`me/exchange-requests?incoming#${version}`, (signal) =>
    listRequests({ direction: "incoming", status: "PENDING", page: 1 }, signal),
  );
  const data = requests.state.status === "ready" ? requests.state.data : null;

  return (
    <section
      id="dashboard-requests"
      className="dashboard-panel dashboard-requests"
      aria-labelledby="requests-title"
      aria-busy={requests.state.status === "loading"}
    >
      <div className="dashboard-section-heading">
        <h2 id="requests-title">Нові запити</h2>
        <Link className="dashboard-section-action" href={requestsHref({ status: "PENDING" })}>
          {data && data.total > 0 ? `Усі запити · ${data.total}` : "Усі запити"}
          <Icon name="chevron-right" />
        </Link>
      </div>
      {notice && (
        <p className="dashboard-demo-note" role="status">
          {notice}
        </p>
      )}
      {requests.state.status === "loading" && <SkeletonPanelRows count={2} variant="request" />}
      {requests.state.status === "error" && (
        <DiscoveryError error={requests.state.error} path="/main" onRetry={requests.retry} />
      )}
      {data &&
        (data.items.length === 0 ? (
          <DashboardEmptyState
            icon="inbox"
            message="Нових запитів немає. Тут з’являться пропозиції обміну від інших студентів."
          />
        ) : (
          data.items.slice(0, shownRequests).map((request) => {
            const view = requestPerspective(request, viewerId);
            return (
              <article className="dashboard-request" key={request.id}>
                <div className="dashboard-request__person">
                  <DashboardAvatar
                    initials={initials(view.partner)}
                    color={avatarColor(view.partner.id)}
                    size={32}
                  />
                  <div>
                    <h3>
                      <Link className="exchange-person__link" href={requestHref(request.id)}>
                        {fullName(view.partner)}
                      </Link>
                    </h3>
                    <p>{formatRelative(request.createdAt)}</p>
                  </div>
                  <StatusBadge status={request.status} />
                </div>
                <div className="dashboard-tags" aria-label="Обмін навичками">
                  <SkillTag name={view.partnerTeaches.skill.name} learn />
                  <Icon name="swap" />
                  <SkillTag name={view.youTeach.skill.name} />
                </div>
                <div className="dashboard-request__actions">
                  <RequestActions
                    request={request}
                    viewerId={viewerId}
                    csrfToken={csrfToken}
                    path="/main"
                    onChanged={(text) => {
                      setNotice(text);
                      setVersion((value) => value + 1);
                    }}
                  />
                </div>
              </article>
            );
          })
        ))}
    </section>
  );
}

function ExchangeRow({ exchange, viewerId }: { exchange: Exchange; viewerId: string }) {
  const view = requestPerspective(exchange, viewerId);
  const { progress } = exchange;
  return (
    <article className="dashboard-exchange">
      <DashboardAvatar
        initials={initials(exchange.partner)}
        color={avatarColor(exchange.partner.id)}
      />
      <div className="dashboard-exchange__person">
        <div className="dashboard-exchange__heading">
          <h3>
            <Link className="exchange-person__link" href={exchangeHref(exchange.id)}>
              {fullName(exchange.partner)}
            </Link>
          </h3>
          <StatusBadge status={exchange.status} kind="exchange" />
        </div>
        <div className="dashboard-tags">
          <SkillTag name={`Ти: ${view.youTeach.skill.name}`} />
          <Icon name="swap" />
          <SkillTag
            name={`${exchange.partner.firstName}: ${view.partnerTeaches.skill.name}`}
            learn
          />
        </div>
      </div>
      <div className="dashboard-exchange__progress">
        <span>
          {progress.completedSessions} з {countLabel(progress.totalSessions, sessionForms)}
        </span>
        <progress
          value={progress.completedSessions}
          max={Math.max(progress.totalSessions, 1)}
          aria-label={`Прогрес обміну з ${fullName(exchange.partner)}`}
        />
        <p>
          <Icon name="clock" />
          {exchange.nextSession
            ? formatDateTime(exchange.nextSession.startsAt)
            : "Заняття ще не заплановані"}
        </p>
      </div>
    </article>
  );
}

export function DashboardExchanges({
  exchanges,
  viewerId,
  onOpenModal,
  onRetry,
}: {
  exchanges: QueryState<ExchangePage>;
  viewerId: string;
  onOpenModal: OpenDashboardModal;
  onRetry: () => void;
}) {
  const data = exchanges.status === "ready" ? exchanges.data : null;
  return (
    <section
      className="dashboard-panel dashboard-exchanges"
      aria-labelledby="exchanges-title"
      aria-busy={exchanges.status === "loading"}
    >
      <div className="dashboard-section-heading">
        <h2 id="exchanges-title">Активні обміни</h2>
        <SectionAction arrow onOpenModal={onOpenModal}>
          Усі обміни
        </SectionAction>
      </div>
      {exchanges.status === "loading" && <SkeletonPanelRows count={1} variant="exchange" />}
      {exchanges.status === "error" && (
        <DiscoveryError error={exchanges.error} path="/main" onRetry={onRetry} />
      )}
      {data &&
        (data.items.length === 0 ? (
          <DashboardEmptyState
            icon="nav-swap"
            message="Активних обмінів поки немає. Обмін розпочнеться, щойно хтось прийме запит."
          />
        ) : (
          data.items
            .slice(0, shownExchanges)
            .map((exchange) => (
              <ExchangeRow exchange={exchange} viewerId={viewerId} key={exchange.id} />
            ))
        ))}
    </section>
  );
}
