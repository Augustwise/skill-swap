import { AuthError, authRequest, writeRequest } from "./AuthApi";
import { isUuid } from "./AppRoutes";
import type { LessonFormat, Person, ResultPage, StudentSummary } from "./DiscoveryApi";
import type { Exchange } from "./ExchangeApi";

export type SessionStatus = "DRAFT" | "PROPOSED" | "SCHEDULED" | "COMPLETED" | "CANCELLED";
export type SessionProposal = {
  id: string;
  sessionId: string;
  status: "PENDING" | "ACCEPTED" | "DECLINED" | "SUPERSEDED";
  proposedBy: Person;
  startsAt: string;
  endsAt: string;
  format: LessonFormat;
  meetingPlace: string | null;
  meetingUrl: string | null;
  respondedBy: Person | null;
  respondedAt: string | null;
  createdAt: string;
};
export type LessonSession = {
  id: string;
  exchangeId: string;
  planOrder: number;
  title: string;
  description: string | null;
  status: SessionStatus;
  format: LessonFormat;
  skill: { skillId: string; categoryId: string; name: string };
  plannedDurationMinutes: number;
  teacher: StudentSummary;
  learner: StudentSummary;
  partner: StudentSummary;
  role: "TEACHER" | "LEARNER";
  startsAt: string | null;
  endsAt: string | null;
  durationMinutes: number | null;
  meetingPlace: string | null;
  meetingUrl: string | null;
  activeProposal: SessionProposal | null;
  scheduleConfirmedByMe: boolean;
  scheduleConfirmedByPartner: boolean;
  canRespondToProposal: boolean;
  canCancel: boolean;
  cancellationReason: string | null;
};
export type SessionDetails = { session: LessonSession; proposals: SessionProposal[] };
export type ExchangeSessions = { exchange: Exchange; items: LessonSession[] };
export type SchedulePage = ResultPage<LessonSession> & {
  timezone: string;
  pendingProposalsCount: number;
};
export type SessionDraft = {
  title: string;
  description: string;
  teacherId: string;
  startsAt: string;
  endsAt: string;
  format: LessonFormat;
  meetingPlace: string;
  meetingUrl: string;
};

export const sessionHref = (id: string) => `/sessions/${encodeURIComponent(id)}`;

export function exchangeSessions(id: string, signal?: AbortSignal) {
  if (!isUuid(id)) return Promise.reject(new AuthError(404, "exchange_not_found"));
  return authRequest<ExchangeSessions>(`exchanges/${id}/sessions`, { signal });
}

export function sessionDetails(id: string, signal?: AbortSignal) {
  if (!isUuid(id)) return Promise.reject(new AuthError(404, "session_not_found"));
  return authRequest<SessionDetails>(`sessions/${id}`, { signal });
}

export function proposeSession(id: string, draft: SessionDraft, csrfToken: string) {
  return writeRequest<{ session: LessonSession; proposal: SessionProposal }>(
    `sessions/${encodeURIComponent(id)}/proposals`,
    { body: JSON.stringify(draft) },
    csrfToken,
  );
}

export function decideSession(id: string, action: "accept" | "decline", csrfToken: string) {
  return writeRequest<{ session: LessonSession }>(
    `sessions/${encodeURIComponent(id)}/${action}`,
    {},
    csrfToken,
  );
}

export function cancelSession(id: string, reason: string, csrfToken: string) {
  return writeRequest<{ session: LessonSession }>(
    `sessions/${encodeURIComponent(id)}/cancel`,
    { body: JSON.stringify({ reason: reason.trim() }) },
    csrfToken,
  );
}

const errors: Record<string, string> = {
  session_not_found: "Заняття не знайдено або ти не береш у ньому участі.",
  exchange_not_found: "Обмін не знайдено або ти не береш у ньому участі.",
  exchange_not_active: "Обмін уже не активний. Онови сторінку, щоб побачити зміни.",
  exchange_blocked: "Планування недоступне: учасника заблоковано або його акаунт недоступний.",
  plan_limit_reached: "Усі заняття цього викладача вже заплановані.",
  session_not_schedulable: "Це заняття більше не можна запланувати. Онови його дані.",
  session_not_proposed: "Пропозиція вже змінилася. Перевір актуальний час перед погодженням.",
  session_not_cancellable: "Заняття вже не можна скасувати. Онови його дані.",
  action_not_allowed: "На цю пропозицію може відповісти лише партнер її автора.",
  session_in_past: "Запропонований час уже минув. Обери майбутню дату.",
  session_conflict: "У цей час хтось із вас уже має погоджене заняття. Обери інший час.",
  validation_failed: "Перевір тему, викладача, час і місце зустрічі.",
};

export function sessionErrorMessage(error: unknown) {
  if (error instanceof AuthError) return errors[error.code] ?? error.message;
  return "Не вдалося зв’язатися із сервером. Перевір з’єднання і спробуй ще раз.";
}
