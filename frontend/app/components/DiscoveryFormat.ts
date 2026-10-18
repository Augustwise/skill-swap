import type { LessonFormat, Person } from "./DiscoveryApi";
import type { Reference } from "./OnboardingApi";

export const formatLabels: Record<LessonFormat, string> = {
  ONLINE: "Онлайн",
  OFFLINE: "Офлайн",
};

const avatarColors = ["purple", "blue", "pink", "green", "amber", "indigo"] as const;
const pluralRules = new Intl.PluralRules("uk-UA");
const ratingFormat = new Intl.NumberFormat("uk-UA", {
  minimumFractionDigits: 1,
  maximumFractionDigits: 1,
});
const dateFormat = new Intl.DateTimeFormat("uk-UA", {
  day: "numeric",
  month: "long",
  year: "numeric",
});

export function fullName(person: Pick<Person, "firstName" | "lastName">) {
  return `${person.firstName} ${person.lastName}`.trim();
}

export function initials(person: Pick<Person, "firstName" | "lastName">) {
  const letters = `${Array.from(person.firstName)[0] ?? ""}${Array.from(person.lastName)[0] ?? ""}`;
  return letters.toLocaleUpperCase("uk") || "СС";
}

// A stable color per student keeps avatars recognizable across pages.
export function avatarColor(id: string) {
  let hash = 0;
  for (const char of id) hash = (hash * 31 + char.charCodeAt(0)) >>> 0;
  return avatarColors[hash % avatarColors.length];
}

export function plural(count: number, [one, few, many]: readonly [string, string, string]) {
  const rule = pluralRules.select(count);
  return rule === "one" ? one : rule === "few" ? few : many;
}

export function countLabel(count: number, forms: readonly [string, string, string]) {
  return `${count} ${plural(count, forms)}`;
}

export const studentForms = ["студент", "студенти", "студентів"] as const;
export const reviewForms = ["відгук", "відгуки", "відгуків"] as const;
export const skillForms = ["навичка", "навички", "навичок"] as const;

export function formatRating(value: number) {
  return ratingFormat.format(value);
}

export function formatDate(value: string) {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "" : dateFormat.format(date);
}

export function formatList(formats: readonly LessonFormat[], city = "") {
  return formats
    .map((format) =>
      format === "OFFLINE" && city ? `${formatLabels[format]} · ${city}` : formatLabels[format],
    )
    .join(", ");
}

export function studyDetails(student: {
  university: Reference;
  faculty?: Reference | null;
  course?: number | null;
}) {
  return [
    student.university.name,
    student.faculty?.name,
    student.course ? `${student.course} курс` : "",
  ]
    .filter(Boolean)
    .join(" · ");
}
