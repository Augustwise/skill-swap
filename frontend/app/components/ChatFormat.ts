import type { ChatMessage } from "./ChatApi";

const timeFormat = new Intl.DateTimeFormat("uk-UA", { hour: "2-digit", minute: "2-digit" });
const dayFormat = new Intl.DateTimeFormat("uk-UA", { day: "numeric", month: "long" });
const fullDayFormat = new Intl.DateTimeFormat("uk-UA", {
  day: "numeric",
  month: "long",
  year: "numeric",
});
const shortDayFormat = new Intl.DateTimeFormat("uk-UA", { day: "numeric", month: "short" });

function parse(value: string) {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? null : date;
}

// Whole local days between two moments: 0 for today, 1 for yesterday.
function daysAgo(date: Date, now: Date) {
  const day = new Date(date.getFullYear(), date.getMonth(), date.getDate());
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  return Math.round((today.getTime() - day.getTime()) / 86_400_000);
}

export function formatTime(value: string) {
  const date = parse(value);
  return date ? timeFormat.format(date) : "";
}

export function dayLabel(value: string, now = new Date()) {
  const date = parse(value);
  if (!date) return "";
  const days = daysAgo(date, now);
  if (days === 0) return "Сьогодні";
  if (days === 1) return "Учора";
  return (date.getFullYear() === now.getFullYear() ? dayFormat : fullDayFormat).format(date);
}

export function listTime(value: string, now = new Date()) {
  const date = parse(value);
  if (!date) return "";
  const days = daysAgo(date, now);
  if (days === 0) return timeFormat.format(date);
  if (days === 1) return "учора";
  return date.getFullYear() === now.getFullYear()
    ? shortDayFormat.format(date)
    : fullDayFormat.format(date);
}

export function groupByDay(messages: ChatMessage[], now = new Date()) {
  const groups: { label: string; messages: ChatMessage[] }[] = [];
  for (const message of messages) {
    const label = dayLabel(message.createdAt, now);
    const last = groups.at(-1);
    if (last && last.label === label) last.messages.push(message);
    else groups.push({ label, messages: [message] });
  }
  return groups;
}
