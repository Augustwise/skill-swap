import type { SessionDraft } from "./SessionApi";

// Inputs use the browser's named time zone; the API always receives an explicit UTC offset.
export function localDateTime(value: string | null) {
  if (!value) return "";
  const date = new Date(value);
  if (!Number.isFinite(date.getTime())) return "";
  const part = (number: number) => String(number).padStart(2, "0");
  return `${date.getFullYear()}-${part(date.getMonth() + 1)}-${part(date.getDate())}T${part(date.getHours())}:${part(date.getMinutes())}`;
}

export function timestamp(value: string) {
  if (!/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$/.test(value)) return null;
  const date = new Date(value);
  // Reject impossible dates and local clock times skipped during a DST transition.
  return Number.isFinite(date.getTime()) && localDateTime(date.toISOString()) === value
    ? date.toISOString()
    : null;
}

export function meetingLink(value: string | null) {
  if (!value || /[\s\\]/.test(value) || !/^https?:\/\//i.test(value)) return null;
  try {
    const url = new URL(value);
    return ["http:", "https:"].includes(url.protocol) && !url.username && !url.password
      ? url.href
      : null;
  } catch {
    return null;
  }
}

export function sessionDraftErrors(draft: SessionDraft, teacherIds: string[], now = Date.now()) {
  const errors: Partial<Record<keyof SessionDraft, string>> = {};
  const length = (value: string) => Array.from(value.trim()).length;
  if (!length(draft.title) || length(draft.title) > 255 || /[\r\n\p{Cc}]/u.test(draft.title))
    errors.title = "Вкажи тему від 1 до 255 символів в одному рядку.";
  if (length(draft.description) > 1000) errors.description = "Опис — до 1000 символів.";
  if (!teacherIds.includes(draft.teacherId)) errors.teacherId = "Обери викладача цього обміну.";
  const start = timestamp(draft.startsAt);
  const end = timestamp(draft.endsAt);
  if (!start || Date.parse(start) <= now) errors.startsAt = "Обери коректний майбутній час.";
  if (!end) errors.endsAt = "Вкажи коректний час завершення.";
  if (start && end) {
    const minutes = (Date.parse(end) - Date.parse(start)) / 60000;
    if (minutes < 15 || minutes > 240)
      errors.endsAt = "Завершення має бути через 15–240 хвилин після початку.";
  }
  if (draft.format === "ONLINE") {
    if (!meetingLink(draft.meetingUrl.trim()) || length(draft.meetingUrl) > 500)
      errors.meetingUrl = "Вкажи http(s) посилання на зустріч до 500 символів.";
  } else if (draft.format === "OFFLINE") {
    if (!length(draft.meetingPlace) || length(draft.meetingPlace) > 180)
      errors.meetingPlace = "Вкажи місце зустрічі до 180 символів.";
  } else errors.format = "Обери формат заняття.";
  return errors;
}

export function sessionPayload(draft: SessionDraft): SessionDraft {
  const startsAt = timestamp(draft.startsAt);
  const endsAt = timestamp(draft.endsAt);
  if (!startsAt || !endsAt) throw new Error("Некоректний час заняття.");
  return {
    ...draft,
    title: draft.title.trim(),
    description: draft.description.trim(),
    startsAt,
    endsAt,
    meetingUrl: draft.format === "ONLINE" ? draft.meetingUrl.trim() : "",
    meetingPlace: draft.format === "OFFLINE" ? draft.meetingPlace.trim() : "",
  };
}

export function sessionTime(value: string, timezone?: string) {
  return new Intl.DateTimeFormat("uk-UA", {
    dateStyle: "medium",
    timeStyle: "short",
    ...(timezone && { timeZone: timezone }),
  }).format(new Date(value));
}
