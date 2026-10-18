import type { ExchangeStatus, RequestStatus, RequestTerms } from "./ExchangeApi";
import { countLabel } from "./DiscoveryFormat";

export type StatusTone = "pending" | "accepted" | "active" | "completed" | "declined" | "neutral";

export const requestStatusLabels: Record<RequestStatus, string> = {
  PENDING: "Очікує",
  ACCEPTED: "Прийнято",
  DECLINED: "Відхилено",
  WITHDRAWN: "Відкликано",
};

export const requestFilterLabels: Record<RequestStatus | "", string> = {
  "": "Усі",
  PENDING: "Очікують",
  ACCEPTED: "Прийняті",
  DECLINED: "Відхилені",
  WITHDRAWN: "Відкликані",
};

export const exchangeStatusLabels: Record<ExchangeStatus, string> = {
  PLANNED: "Заплановано",
  ACTIVE: "Активний",
  PAUSED: "Призупинено",
  COMPLETED: "Завершено",
  CANCELLED: "Скасовано",
};

const tones: Record<RequestStatus | ExchangeStatus, StatusTone> = {
  PENDING: "pending",
  ACCEPTED: "accepted",
  DECLINED: "declined",
  WITHDRAWN: "neutral",
  PLANNED: "accepted",
  ACTIVE: "active",
  PAUSED: "pending",
  COMPLETED: "completed",
  CANCELLED: "declined",
};

export function statusTone(status: RequestStatus | ExchangeStatus) {
  return tones[status];
}

export const historyLabels: Record<RequestStatus, string> = {
  PENDING: "Запит надіслано",
  ACCEPTED: "Запит прийнято, обмін розпочато",
  DECLINED: "Запит відхилено",
  WITHDRAWN: "Запит відкликано",
};

export const sessionForms = ["заняття", "заняття", "занять"] as const;

export function formatDuration(minutes: number) {
  const hours = Math.floor(minutes / 60);
  const rest = minutes % 60;
  if (hours === 0) return `${rest} хв`;
  return rest === 0 ? `${hours} год` : `${hours} год ${rest} хв`;
}

export function termsLabel(terms: Pick<RequestTerms, "sessions" | "durationMinutes">) {
  return `${countLabel(terms.sessions, sessionForms)} × ${formatDuration(terms.durationMinutes)}`;
}

export function totalTime(terms: Pick<RequestTerms, "sessions" | "durationMinutes">) {
  return formatDuration(terms.sessions * terms.durationMinutes);
}

const dateTimeFormat = new Intl.DateTimeFormat("uk-UA", {
  day: "numeric",
  month: "long",
  year: "numeric",
  hour: "2-digit",
  minute: "2-digit",
});
const relativeFormat = new Intl.RelativeTimeFormat("uk-UA", { numeric: "auto" });

export function formatDateTime(value: string) {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "" : dateTimeFormat.format(date);
}

// Recent events read as "5 хвилин тому" or "вчора"; older ones show the full date.
export function formatRelative(value: string, now = new Date()) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  const minutes = Math.round((now.getTime() - date.getTime()) / 60000);
  if (minutes < 1) return "щойно";
  if (minutes < 60) return relativeFormat.format(-minutes, "minute");
  const hours = Math.round(minutes / 60);
  if (hours < 24) return relativeFormat.format(-hours, "hour");
  const days = Math.round(hours / 24);
  if (days < 7) return relativeFormat.format(-days, "day");
  return dateTimeFormat.format(date);
}
