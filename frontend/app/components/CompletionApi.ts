import { AuthError, authRequest, writeRequest } from "./AuthApi";
import { isUuid } from "./AppRoutes";
import { readPage, type Person } from "./DiscoveryApi";
import type { Exchange, ExchangePage, ExchangeStatus } from "./ExchangeApi";
import type { LessonSession } from "./SessionApi";

export type CompletionState = {
  status: string;
  completionConfirmedByMe: boolean;
  completionConfirmedByPartner: boolean;
  canConfirmCompletion: boolean;
};
export type ExchangeHistoryEntry = {
  status: ExchangeStatus;
  changedBy: Person | null;
  reason: string | null;
  createdAt: string;
};
export type ExchangeHistory = { exchange: Exchange; history: ExchangeHistoryEntry[] };
export type ExchangeFilter = "ACTIVE" | "COMPLETED" | "CANCELLED";
export const exchangeFilters: Record<ExchangeFilter, string> = {
  ACTIVE: "Активні",
  COMPLETED: "Завершені",
  CANCELLED: "Скасовані",
};

export function readExchangeFilters(params: { get(name: string): string | null }) {
  const status = params.get("status");
  return {
    status:
      status && Object.hasOwn(exchangeFilters, status) ? (status as ExchangeFilter) : "ACTIVE",
    page: readPage(params.get("page")),
  };
}

export function exchangesHref(status: ExchangeFilter = "ACTIVE", page = 1) {
  const query = new URLSearchParams();
  if (status !== "ACTIVE") query.set("status", status);
  if (page > 1) query.set("page", String(page));
  return query.size ? `/exchanges?${query}` : "/exchanges";
}

export function listExchanges(status: ExchangeFilter, page: number, signal?: AbortSignal) {
  return authRequest<ExchangePage>(`me/exchanges?status=${status}&page=${page}`, { signal });
}

export function exchangeHistory(id: string, signal?: AbortSignal) {
  if (!isUuid(id)) return Promise.reject(new AuthError(404, "exchange_not_found"));
  return authRequest<ExchangeHistory>(`exchanges/${id}`, { signal });
}

export function confirmLesson(id: string, csrfToken: string) {
  return writeRequest<{ session: LessonSession; exchange: Exchange }>(
    `sessions/${encodeURIComponent(id)}/confirm`,
    {},
    csrfToken,
  );
}

export function completeExchange(id: string, csrfToken: string) {
  return writeRequest<{ exchange: Exchange }>(
    `exchanges/${encodeURIComponent(id)}/complete`,
    {},
    csrfToken,
  );
}

export function cancelExchange(id: string, reason: string, csrfToken: string) {
  return writeRequest<{ exchange: Exchange }>(
    `exchanges/${encodeURIComponent(id)}/cancel`,
    { body: JSON.stringify({ reason: reason.trim() }) },
    csrfToken,
  );
}

export function cancellationError(reason: string) {
  const value = reason.trim();
  if (
    !value ||
    Array.from(value).length > 500 ||
    /[\u0000-\u0008\u000b\u000c\u000e-\u001f\u007f]/.test(value)
  ) {
    return "Вкажи причину від 1 до 500 символів без керівних символів.";
  }
  return "";
}

// The server decides eligibility, including time, membership and blocks.
export function completionMessage(item: CompletionState, kind: "lesson" | "exchange") {
  if (item.status === "COMPLETED") {
    return kind === "lesson" ? "Заняття зараховано обом учасникам." : "Обмін успішно завершено!";
  }
  if (item.status === "CANCELLED") return "Скасовано. Історію збережено.";
  if (item.completionConfirmedByMe) return "Ти підтвердив. Очікуємо підтвердження партнера.";
  if (item.completionConfirmedByPartner) return "Партнер уже підтвердив. Тепер твоя черга.";
  if (item.canConfirmCompletion) {
    return kind === "lesson"
      ? "Час заняття минув. Підтвердь, що воно відбулося."
      : "План виконано в обох напрямках. Підтвердь завершення обміну.";
  }
  return kind === "lesson"
    ? "Підтвердити проведення можна після завершення погодженого часу заняття."
    : "Для завершення обміну виконайте план занять в обох напрямках.";
}

const messages: Record<string, string> = {
  session_not_found: "Заняття не знайдено або ти не береш у ньому участі.",
  session_not_confirmable: "Це заняття вже не можна підтвердити. Онови дані.",
  session_not_finished: "Заняття ще не закінчилося. Підтвердь проведення після його завершення.",
  exchange_not_found: "Обмін не знайдено або ти не береш у ньому участі.",
  exchange_not_active: "Обмін уже не активний. Ми оновили його стан.",
  exchange_blocked: "Дія недоступна: учасника заблоковано або його акаунт недоступний.",
  plan_not_completed: "Спершу обидва учасники мають підтвердити всі заняття в обох напрямках.",
  validation_failed: "Вкажи причину скасування від 1 до 500 символів.",
};

export function completionErrorMessage(error: unknown) {
  if (error instanceof AuthError) return messages[error.code] ?? error.message;
  return "Не вдалося зв’язатися із сервером. Перевір з’єднання і спробуй ще раз.";
}
