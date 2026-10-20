import type { DashboardIconName } from "./DashboardIcon";

export type AppSection =
  "main" | "search" | "matches" | "requests" | "exchanges" | "messages" | "schedule" | "profile";
export type SkeletonLayout =
  | "dashboard"
  | "search"
  | "matches"
  | "requests"
  | "detail"
  | "messages"
  | "notifications"
  | "profile";

export const navigation: {
  label: string;
  icon: DashboardIconName;
  section?: AppSection;
  href?: string;
}[] = [
  { label: "Головна", icon: "dashboard", section: "main", href: "/main" },
  { label: "Пошук навичок", icon: "nav-search", section: "search", href: "/search" },
  { label: "Мої збіги", icon: "nav-sparkles", section: "matches", href: "/matches" },
  { label: "Запити на обмін", icon: "inbox", section: "requests", href: "/requests" },
  { label: "Активні обміни", icon: "nav-swap", section: "exchanges", href: "/exchanges" },
  { label: "Повідомлення", icon: "nav-chat", section: "messages", href: "/messages" },
  { label: "Календар", icon: "calendar", section: "schedule", href: "/schedule" },
];

// One title per page, shared by the page itself and its loading skeleton.
export const pageShells = {
  main: {
    section: "main",
    title: "Головна",
    subtitle: "Твоя активність за тиждень",
    subject: "головну",
    layout: "dashboard",
  },
  search: {
    section: "search",
    title: "Пошук навичок",
    subtitle: "Знайди студента за навичкою або ім’ям",
    subject: "пошук",
    layout: "search",
  },
  matches: {
    section: "matches",
    title: "Мої збіги",
    subtitle: "Студенти, з якими можна обмінятися навичками в обидва боки",
    subject: "збіги",
    layout: "matches",
  },
  student: {
    section: undefined,
    title: "Профіль студента",
    subtitle: "Навички, рівні, формат занять і відгуки",
    subject: "профіль студента",
    layout: "detail",
  },
  requests: {
    section: "requests",
    title: "Запити на обмін",
    subtitle: "Вхідні й надіслані пропозиції обміну навичками",
    subject: "запити",
    layout: "requests",
  },
  request: {
    section: "requests",
    title: "Запит на обмін",
    subtitle: "Умови обміну, повідомлення та історія статусів",
    subject: "запит",
    layout: "detail",
  },
  exchange: {
    section: "exchanges",
    title: "Обмін навичками",
    subtitle: "Погоджені умови та прогрес обміну",
    subject: "обмін",
    layout: "detail",
  },
  exchanges: {
    section: "exchanges",
    title: "Мої обміни",
    subtitle: "Активне навчання та історія завершених і скасованих обмінів",
    subject: "обміни",
    layout: "requests",
  },
  lesson: {
    section: undefined,
    title: "Заняття",
    subtitle: "Тема, час і погодження зустрічі",
    subject: "заняття",
    layout: "detail",
  },
  schedule: {
    section: "schedule",
    title: "Розклад",
    subtitle: "Твої зустрічі у хронологічному порядку",
    subject: "розклад",
    layout: "requests",
  },
  messages: {
    section: "messages",
    title: "Повідомлення",
    subtitle: "Приватні чати з учасниками твоїх запитів і обмінів",
    subject: "повідомлення",
    layout: "messages",
  },
  notifications: {
    section: undefined,
    title: "Сповіщення",
    subtitle: "Запити, рішення, нові повідомлення та зміни занять",
    subject: "сповіщення",
    layout: "notifications",
  },
  profile: {
    section: "profile",
    title: "Мій профіль",
    subtitle: "Дані, які бачать студенти у твоєму профілі",
    subject: "профіль",
    layout: "profile",
  },
} as const satisfies Record<
  string,
  {
    section: AppSection | undefined;
    title: string;
    subtitle: string;
    subject: string;
    layout: SkeletonLayout;
  }
>;

export type AppPage = keyof typeof pageShells;
