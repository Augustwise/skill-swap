import Link from "next/link";
import type { ReactNode } from "react";
import { studentHref } from "./AppRoutes";
import { DashboardAvatar } from "./DashboardCards";
import { DashboardIcon as Icon, type DashboardIconName } from "./DashboardIcon";
import { avatarColor, countLabel, formatList, fullName, initials } from "./DiscoveryFormat";
import {
  requestHref,
  requestPerspective,
  type ExchangeRequest,
  type ExchangeStatus,
  type RequestStatus,
  type RequestTerms,
} from "./ExchangeApi";
import {
  exchangeStatusLabels,
  formatRelative,
  requestStatusLabels,
  sessionForms,
  statusTone,
  termsLabel,
  type StatusTone,
} from "./ExchangeFormat";
import { skillLevels } from "./ProfileApi";

const toneDots: Record<StatusTone, DashboardIconName> = {
  pending: "pending-dot",
  accepted: "accepted-dot",
  active: "active-dot",
  completed: "completed-dot",
  declined: "declined-dot",
  neutral: "neutral-dot",
};

export function StatusBadge({
  status,
  kind = "request",
}: {
  status: RequestStatus | ExchangeStatus;
  kind?: "request" | "exchange";
}) {
  const tone = statusTone(status);
  const label =
    kind === "exchange"
      ? exchangeStatusLabels[status as ExchangeStatus]
      : requestStatusLabels[status as RequestStatus];
  return (
    <span className={`dashboard-status dashboard-status--${tone}`}>
      <Icon name={toneDots[tone]} />
      {label}
    </span>
  );
}

export function SkillTag({ name, learn = false }: { name: string; learn?: boolean }) {
  return (
    <span className={`dashboard-skill ${learn ? "dashboard-skill--learn" : ""}`}>
      <Icon name={learn ? "learn-dot" : "teach-dot"} />
      {name}
    </span>
  );
}

// One direction of an exchange: the skill, both levels and the agreed lessons.
export function DirectionTerms({
  title,
  terms,
  learn = false,
  teacher,
  learner,
  detailed = false,
}: {
  title: string;
  terms: RequestTerms;
  learn?: boolean;
  teacher: string;
  learner: string;
  detailed?: boolean;
}) {
  return (
    <div className={`exchange-direction ${learn ? "exchange-direction--learn" : ""}`}>
      <h4>{title}</h4>
      <SkillTag name={terms.skill.name} learn={learn} />
      <p className="exchange-direction__plan">{termsLabel(terms)}</p>
      {detailed && (
        <dl className="exchange-direction__levels">
          <div>
            <dt>Викладач</dt>
            <dd>
              {teacher} · {skillLevels[terms.teacherLevel].toLocaleLowerCase("uk")}
            </dd>
          </div>
          <div>
            <dt>Учень</dt>
            <dd>
              {learner} · {skillLevels[terms.learnerLevel].toLocaleLowerCase("uk")}
            </dd>
          </div>
        </dl>
      )}
    </div>
  );
}

export function ExchangeDirections({
  youTeach,
  partnerTeaches,
  partnerName,
  detailed = false,
}: {
  youTeach: RequestTerms;
  partnerTeaches: RequestTerms;
  partnerName: string;
  detailed?: boolean;
}) {
  return (
    <div className="exchange-directions">
      <DirectionTerms
        title="Ти навчиш"
        terms={youTeach}
        teacher="Ти"
        learner={partnerName}
        detailed={detailed}
      />
      <span className="exchange-directions__swap" aria-hidden="true">
        <Icon name="swap" />
      </span>
      <DirectionTerms
        title={`${partnerName} навчить тебе`}
        terms={partnerTeaches}
        learn
        teacher={partnerName}
        learner="Ти"
        detailed={detailed}
      />
    </div>
  );
}

export function PartnerHeading({
  partner,
  meta,
  badge,
  from,
  level = 3,
}: {
  partner: ExchangeRequest["requester"];
  meta?: string;
  badge?: ReactNode;
  from?: string;
  level?: 2 | 3;
}) {
  const Heading = level === 2 ? "h2" : "h3";
  const place = [partner.university.name, partner.city].filter(Boolean).join(" · ");
  return (
    <div className="dashboard-match__person exchange-person">
      <DashboardAvatar
        initials={initials(partner)}
        color={avatarColor(partner.id)}
        size={level === 2 ? 48 : 40}
      />
      <div className="dashboard-match__identity">
        <Heading>
          {from ? (
            <Link className="exchange-person__link" href={studentHref(partner.id, from)}>
              {fullName(partner)}
            </Link>
          ) : (
            fullName(partner)
          )}
        </Heading>
        <p>{[place, meta].filter(Boolean).join(" · ")}</p>
      </div>
      {badge}
    </div>
  );
}

export function RequestCard({
  request,
  viewerId,
  from,
  children,
}: {
  request: ExchangeRequest;
  viewerId: string;
  from: string;
  children?: ReactNode;
}) {
  const view = requestPerspective(request, viewerId);
  const incoming = view.role === "recipient";
  return (
    <li className="dashboard-match exchange-card">
      <PartnerHeading
        partner={view.partner}
        meta={formatRelative(request.createdAt)}
        badge={<StatusBadge status={request.status} />}
      />
      <p className="exchange-card__kind">
        {incoming ? "Вхідний запит" : "Надісланий запит"} · {formatList([request.format])} · разом{" "}
        {countLabel(request.totalSessions, sessionForms)}
      </p>
      <ExchangeDirections
        youTeach={view.youTeach}
        partnerTeaches={view.partnerTeaches}
        partnerName={view.partner.firstName}
      />
      {request.message && <p className="exchange-card__message">{request.message}</p>}
      <div className="dashboard-match__actions exchange-card__actions">
        {children}
        <Link
          className="dashboard-button"
          href={`${requestHref(request.id)}?from=${encodeURIComponent(from)}`}
          aria-label={`Деталі запиту: ${fullName(view.partner)}`}
        >
          Деталі
          <Icon name="chevron-right" />
        </Link>
      </div>
    </li>
  );
}
