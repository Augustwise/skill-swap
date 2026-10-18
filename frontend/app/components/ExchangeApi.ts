import { AuthError, authRequest, type AuthSession } from "./AuthApi";
import { isUuid } from "./AppRoutes";
import {
  readPage,
  type LessonFormat,
  type MatchDetails,
  type Person,
  type ResultPage,
  type StudentSummary,
  type UserSkill,
} from "./DiscoveryApi";
import type { SkillLevel } from "./OnboardingApi";

export type RequestStatus = "PENDING" | "ACCEPTED" | "DECLINED" | "WITHDRAWN";
export type RequestDirection = "incoming" | "outgoing";
export type RequestAction = "accept" | "decline" | "withdraw";
export type RequestTerms = {
  skill: { skillId: string; categoryId: string; name: string };
  teacherLevel: SkillLevel;
  learnerLevel: SkillLevel;
  sessions: number;
  durationMinutes: number;
};
export type ExchangeRequest = {
  id: string;
  status: RequestStatus;
  format: LessonFormat;
  totalSessions: number;
  message: string | null;
  requester: StudentSummary;
  recipient: StudentSummary;
  requesterTeaches: RequestTerms;
  recipientTeaches: RequestTerms;
  exchangeId: string | null;
  createdAt: string;
  respondedAt: string | null;
};
export type RequestStatusChange = {
  status: RequestStatus;
  changedBy: Person | null;
  createdAt: string;
};
export type RequestDetails = { request: ExchangeRequest; history: RequestStatusChange[] };

export type ExchangeStatus = "PLANNED" | "ACTIVE" | "PAUSED" | "COMPLETED" | "CANCELLED";
export type ExchangeProgress = {
  completedSessions: number;
  totalSessions: number;
  requesterCompletedSessions: number;
  requesterPlannedSessions: number;
  recipientCompletedSessions: number;
  recipientPlannedSessions: number;
  scheduledSessions: number;
  proposedSessions: number;
  draftSessions: number;
  cancelledSessions: number;
  percent: number;
};
export type NextSession = {
  id: string;
  title: string;
  startsAt: string;
  endsAt: string;
  meetingPlace: string | null;
  meetingUrl: string | null;
};
export type Exchange = {
  id: string;
  requestId: string | null;
  status: ExchangeStatus;
  format: LessonFormat;
  totalSessions: number;
  partner: StudentSummary;
  requester: StudentSummary;
  recipient: StudentSummary;
  requesterTeaches: RequestTerms;
  recipientTeaches: RequestTerms;
  progress: ExchangeProgress;
  nextSession: NextSession | null;
  startedAt: string | null;
  completedAt: string | null;
  cancelledAt: string | null;
  cancellationReason: string | null;
  createdAt: string;
};
export type ExchangePage = ResultPage<Exchange>;
export type Acceptance = { request: ExchangeRequest; exchange: Exchange };

export type RequestFilters = {
  direction: RequestDirection;
  status: RequestStatus | "";
  page: number;
};
export type ProposalDraft = {
  teachSkillId: string;
  learnSkillId: string;
  format: LessonFormat | "";
  teachSessions: number;
  teachDurationMinutes: number;
  learnSessions: number;
  learnDurationMinutes: number;
  message: string;
};
export type ProposalOptions = {
  teach: UserSkill[];
  learn: UserSkill[];
  formats: LessonFormat[];
};

export const MAX_SESSIONS = 20;
export const MIN_DURATION = 15;
export const MAX_DURATION = 240;
export const DURATION_STEP = 15;
export const MAX_MESSAGE_LENGTH = 500;

export const requestStatuses: readonly RequestStatus[] = [
  "PENDING",
  "ACCEPTED",
  "DECLINED",
  "WITHDRAWN",
];

export const emptyRequestFilters: RequestFilters = { direction: "incoming", status: "", page: 1 };

export function readRequestFilters(params: { get(name: string): string | null }): RequestFilters {
  const status = params.get("status") ?? "";
  return {
    direction: params.get("direction") === "outgoing" ? "outgoing" : "incoming",
    status: (requestStatuses as readonly string[]).includes(status)
      ? (status as RequestStatus)
      : "",
    page: readPage(params.get("page")),
  };
}

export function requestQueryString(filters: RequestFilters) {
  const params = new URLSearchParams();
  if (filters.direction === "outgoing") params.set("direction", "outgoing");
  if (filters.status) params.set("status", filters.status);
  if (filters.page > 1) params.set("page", String(filters.page));
  return params.toString();
}

export function requestsHref(filters: Partial<RequestFilters> = {}) {
  const query = requestQueryString({ ...emptyRequestFilters, ...filters });
  return query ? `/requests?${query}` : "/requests";
}

export function requestHref(id: string) {
  return `/requests/${encodeURIComponent(id)}`;
}

export function exchangeHref(id: string) {
  return `/exchanges/${encodeURIComponent(id)}`;
}

// The skills, formats and partner as seen by one participant of a request or an exchange.
export function requestPerspective(
  item: Pick<ExchangeRequest, "requester" | "recipient" | "requesterTeaches" | "recipientTeaches">,
  viewerId: string,
) {
  const isRequester = item.requester.id === viewerId;
  return {
    role: isRequester ? ("requester" as const) : ("recipient" as const),
    partner: isRequester ? item.recipient : item.requester,
    youTeach: isRequester ? item.requesterTeaches : item.recipientTeaches,
    partnerTeaches: isRequester ? item.recipientTeaches : item.requesterTeaches,
  };
}

export function availableActions(
  request: Pick<ExchangeRequest, "status" | "requester" | "recipient">,
  viewerId: string,
): RequestAction[] {
  if (request.status !== "PENDING") return [];
  if (request.recipient.id === viewerId) return ["accept", "decline"];
  if (request.requester.id === viewerId) return ["withdraw"];
  return [];
}

// Options come from the match details: what the student wants from you and what they teach.
export function proposalOptions(details: MatchDetails): ProposalOptions {
  return {
    teach: details.wantsToLearn,
    learn: details.canTeachYou,
    formats: details.commonFormats,
  };
}

export function initialProposal(options: ProposalOptions): ProposalDraft {
  return {
    teachSkillId: options.teach.length === 1 ? options.teach[0].skillId : "",
    learnSkillId: options.learn.length === 1 ? options.learn[0].skillId : "",
    format: options.formats.length === 1 ? options.formats[0] : "",
    teachSessions: 2,
    teachDurationMinutes: 60,
    learnSessions: 2,
    learnDurationMinutes: 60,
    message: "",
  };
}

export const durationOptions = Array.from(
  { length: (MAX_DURATION - MIN_DURATION) / DURATION_STEP + 1 },
  (_, index) => MIN_DURATION + index * DURATION_STEP,
);

export const sessionOptions = Array.from({ length: MAX_SESSIONS }, (_, index) => index + 1);

const length = (value: string) => Array.from(value).length;
const inRange = (value: number, min: number, max: number) =>
  Number.isInteger(value) && value >= min && value <= max;

export type ProposalField = "teachSkillId" | "learnSkillId" | "format" | "plan" | "message";
export type ProposalErrors = Partial<Record<ProposalField, string>>;

// Mirrors the server checks so most mistakes are explained before sending.
export function validateProposal(draft: ProposalDraft, options: ProposalOptions) {
  const errors: ProposalErrors = {};
  if (!options.teach.some((skill) => skill.skillId === draft.teachSkillId)) {
    errors.teachSkillId = "Обери навичку, якої ти навчиш.";
  }
  if (!options.learn.some((skill) => skill.skillId === draft.learnSkillId)) {
    errors.learnSkillId = "Обери навичку, якої хочеш навчитися.";
  } else if (draft.learnSkillId === draft.teachSkillId) {
    errors.learnSkillId = "Обери дві різні навички.";
  }
  if (!draft.format || !options.formats.includes(draft.format)) {
    errors.format = "Обери спільний формат занять.";
  }
  if (
    ![draft.teachSessions, draft.learnSessions].every((value) => inRange(value, 1, MAX_SESSIONS))
  ) {
    errors.plan = `Кількість занять у кожному напрямку — від 1 до ${MAX_SESSIONS}.`;
  } else if (
    ![draft.teachDurationMinutes, draft.learnDurationMinutes].every((value) =>
      inRange(value, MIN_DURATION, MAX_DURATION),
    )
  ) {
    errors.plan = `Тривалість заняття — від ${MIN_DURATION} до ${MAX_DURATION} хв.`;
  } else if (
    draft.teachSessions * draft.teachDurationMinutes !==
    draft.learnSessions * draft.learnDurationMinutes
  ) {
    errors.plan = "Сумарний час навчання в обидва боки має бути однаковим.";
  }
  if (length(draft.message.trim()) > MAX_MESSAGE_LENGTH) {
    errors.message = `Повідомлення — до ${MAX_MESSAGE_LENGTH} символів.`;
  }
  return errors;
}

export function proposalTotals(draft: ProposalDraft) {
  return {
    sessions: draft.teachSessions + draft.learnSessions,
    teachMinutes: draft.teachSessions * draft.teachDurationMinutes,
    learnMinutes: draft.learnSessions * draft.learnDurationMinutes,
  };
}

const changedSkill = "Ця навичка вже не підходить для обміну. Онови сторінку й обери іншу.";
const proposalFieldMessages: Record<string, [ProposalField, string]> = {
  teachSkillId: ["teachSkillId", changedSkill],
  learnSkillId: ["learnSkillId", changedSkill],
  format: ["format", "Цей формат більше не спільний для вас. Онови сторінку."],
  teachSessions: ["plan", `Кількість занять у кожному напрямку — від 1 до ${MAX_SESSIONS}.`],
  learnSessions: ["plan", "Сумарний час навчання в обидва боки має бути однаковим."],
  teachDurationMinutes: ["plan", `Тривалість заняття — від ${MIN_DURATION} до ${MAX_DURATION} хв.`],
  learnDurationMinutes: ["plan", `Тривалість заняття — від ${MIN_DURATION} до ${MAX_DURATION} хв.`],
  message: ["message", `Повідомлення — до ${MAX_MESSAGE_LENGTH} символів.`],
};

// Server messages are English, so each known field gets a Ukrainian explanation instead.
export function proposalErrors(error: unknown) {
  const errors: ProposalErrors = {};
  if (!(error instanceof AuthError) || error.code !== "validation_failed") return errors;
  for (const field of Object.keys(error.fields)) {
    const known = proposalFieldMessages[field];
    if (known && !errors[known[0]]) errors[known[0]] = known[1];
  }
  return errors;
}

const exchangeMessages: Record<string, string> = {
  student_not_found: "Студент недоступний: профіль приховано або хтось із вас заблокував іншого.",
  requests_closed: "Студент зараз не приймає нових запитів на обмін.",
  same_university_only: "Студент приймає запити лише від студентів свого університету.",
  duplicate_request: "Запит на цю пару навичок уже очікує відповіді.",
  request_not_found: "Запит не знайдено або ти не береш у ньому участі.",
  action_not_allowed: "Ця дія недоступна для твоєї ролі в запиті.",
  request_not_pending: "На запит уже відповіли. Ми оновили його стан.",
  request_outdated:
    "Запит більше не можна прийняти: студент або одна з навичок уже недоступні. Автор може відкликати запит.",
  exchange_not_found: "Обмін не знайдено або ти не береш у ньому участі.",
  validation_failed: "Перевір виділені поля.",
};

export function exchangeErrorMessage(error: unknown) {
  if (error instanceof AuthError) {
    return exchangeMessages[error.code] ?? error.message;
  }
  return "Не вдалося зв’язатися із сервером. Перевір з’єднання і спробуй ще раз.";
}

// A request answered elsewhere must be reloaded so the page shows its real state.
export function staleRequest(error: unknown) {
  return error instanceof AuthError && error.code === "request_not_pending";
}

async function exchangeWrite<T>(path: string, options: RequestInit, csrfToken: string) {
  try {
    return await authRequest<T>(path, { method: "POST", ...options }, csrfToken);
  } catch (error) {
    if (!(error instanceof AuthError) || error.code !== "csrf_invalid") throw error;
    const session = await authRequest<AuthSession>("auth/me");
    return authRequest<T>(path, { method: "POST", ...options }, session.csrfToken);
  }
}

export async function createRequest(recipientId: string, draft: ProposalDraft, csrfToken: string) {
  const message = draft.message.trim();
  const data = await exchangeWrite<{ request: ExchangeRequest }>(
    "exchange-requests",
    {
      body: JSON.stringify({
        recipientId,
        teachSkillId: draft.teachSkillId,
        learnSkillId: draft.learnSkillId,
        format: draft.format,
        teachSessions: draft.teachSessions,
        teachDurationMinutes: draft.teachDurationMinutes,
        learnSessions: draft.learnSessions,
        learnDurationMinutes: draft.learnDurationMinutes,
        ...(message && { message }),
      }),
    },
    csrfToken,
  );
  return data.request;
}

export function listRequests(filters: RequestFilters, signal?: AbortSignal) {
  const params = new URLSearchParams({ direction: filters.direction });
  if (filters.status) params.set("status", filters.status);
  if (filters.page > 1) params.set("page", String(filters.page));
  return authRequest<ResultPage<ExchangeRequest>>(`me/exchange-requests?${params}`, { signal });
}

export async function pendingRequestCount(direction: RequestDirection, signal?: AbortSignal) {
  const page = await listRequests({ direction, status: "PENDING", page: 1 }, signal);
  return page.total;
}

export async function requestDetails(id: string, signal?: AbortSignal) {
  if (!isUuid(id)) throw new AuthError(404, "request_not_found");
  return authRequest<RequestDetails>(`exchange-requests/${encodeURIComponent(id)}`, { signal });
}

export async function answerRequest(id: string, action: "decline" | "withdraw", csrfToken: string) {
  const data = await exchangeWrite<{ request: ExchangeRequest }>(
    `exchange-requests/${encodeURIComponent(id)}/${action}`,
    {},
    csrfToken,
  );
  return data.request;
}

export function acceptRequest(id: string, csrfToken: string) {
  return exchangeWrite<Acceptance>(
    `exchange-requests/${encodeURIComponent(id)}/accept`,
    {},
    csrfToken,
  );
}

export async function exchangeDetails(id: string, signal?: AbortSignal) {
  if (!isUuid(id)) throw new AuthError(404, "exchange_not_found");
  const data = await authRequest<{ exchange: Exchange }>(`exchanges/${encodeURIComponent(id)}`, {
    signal,
  });
  return data.exchange;
}

export function activeExchanges(signal?: AbortSignal) {
  return authRequest<ExchangePage>("me/exchanges?status=ACTIVE", { signal });
}
