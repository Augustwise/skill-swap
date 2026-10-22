import { AuthError, authRequest } from "./AuthApi";
import { isUuid } from "./AppRoutes";
import type { Profile, Reference, SkillLevel } from "./OnboardingApi";

export type LessonFormat = "ONLINE" | "OFFLINE";
export type UserSkill = { skillId: string; categoryId: string; name: string; level: SkillLevel };
export type Person = { id: string; firstName: string; lastName: string };
export type StudentSummary = Person & { university: Reference; city: string };
export type StudentCard = {
  student: StudentSummary;
  teachingSkills: UserSkill[];
  formats: LessonFormat[];
  mutual: boolean;
};
export type MatchDetails = {
  canTeachYou: UserSkill[];
  wantsToLearn: UserSkill[];
  commonFormats: LessonFormat[];
};
export type MutualMatch = MatchDetails & { student: StudentSummary };
export type ResultPage<T> = { items: T[]; page: number; pageSize: number; total: number };
export type MatchPage = ResultPage<MutualMatch> & { eligibleForMatching: boolean };
export type Review = {
  id: string;
  exchangeId: string;
  author: Person;
  rating: number;
  comment: string;
  createdAt: string;
};
export type StudentProfile = Person & {
  university: Reference;
  faculty: Reference | null;
  course: number | null;
  city: string;
  bio: string;
  formats: LessonFormat[];
  teachingSkills: UserSkill[];
  learningSkills: UserSkill[];
  averageRating: number | null;
  reviewCount: number;
  reviews: Review[];
  mutual: MatchDetails | null;
};
export type SkillCategory = Reference & { slug: string };

export type SearchFilters = {
  q: string;
  categoryId: string;
  level: SkillLevel | "";
  format: LessonFormat | "";
  mutual: boolean;
  page: number;
};

export const PAGE_SIZE = 20;
export const MAX_PAGE = 500;
export const MAX_QUERY_LENGTH = 100;

const levels: readonly string[] = ["BEGINNER", "INTERMEDIATE", "ADVANCED"];
const formats: readonly string[] = ["ONLINE", "OFFLINE"];

export const emptyFilters: SearchFilters = {
  q: "",
  categoryId: "",
  level: "",
  format: "",
  mutual: false,
  page: 1,
};

export function normalizeQuery(value: string) {
  return Array.from(value.replace(/\p{Cc}+/gu, " ").trim())
    .slice(0, MAX_QUERY_LENGTH)
    .join("")
    .trim();
}

export function readPage(value: string | null) {
  const page = Number(value);
  return Number.isInteger(page) && page >= 1 && page <= MAX_PAGE ? page : 1;
}

// Invalid values from a hand-edited URL are dropped instead of failing the whole search.
export function readSearchFilters(params: { get(name: string): string | null }): SearchFilters {
  const categoryId = params.get("categoryId") ?? "";
  const level = params.get("level") ?? "";
  const format = params.get("format") ?? "";
  return {
    q: normalizeQuery(params.get("q") ?? ""),
    categoryId: isUuid(categoryId) ? categoryId.toLowerCase() : "",
    level: levels.includes(level) ? (level as SkillLevel) : "",
    format: formats.includes(format) ? (format as LessonFormat) : "",
    mutual: params.get("mutual") === "true",
    page: readPage(params.get("page")),
  };
}

export function searchQueryString(filters: SearchFilters) {
  const params = new URLSearchParams();
  const q = normalizeQuery(filters.q);
  if (q) params.set("q", q);
  if (filters.categoryId) params.set("categoryId", filters.categoryId);
  if (filters.level) params.set("level", filters.level);
  if (filters.format) params.set("format", filters.format);
  if (filters.mutual) params.set("mutual", "true");
  if (filters.page > 1) params.set("page", String(filters.page));
  return params.toString();
}

export function hasActiveFilters(filters: SearchFilters) {
  return Boolean(
    filters.q || filters.categoryId || filters.level || filters.format || filters.mutual,
  );
}

export function pageCount(total: number, pageSize = PAGE_SIZE) {
  return Math.max(1, Math.ceil(total / pageSize));
}

// Mirrors the server rule: both skill lists, a format, and a city for offline lessons.
export function eligibleForMatching(profile: Profile) {
  return (
    profile.teachingSkills.length > 0 &&
    profile.learningSkills.length > 0 &&
    profile.formats.length > 0 &&
    (!profile.formats.includes("OFFLINE") || profile.city.trim() !== "")
  );
}

// Explains why a student is not a mutual match; the server stays the source of truth.
export function matchOverview(
  viewer: Profile,
  student: Pick<StudentProfile, "teachingSkills" | "learningSkills" | "formats" | "city">,
): MatchDetails {
  const wanted = new Set(viewer.learningSkills.map((skill) => skill.skillId));
  const offered = new Set(viewer.teachingSkills.map((skill) => skill.skillId));
  const city = (value: string) => value.trim().toLocaleLowerCase("uk");
  const sameCity = city(viewer.city) !== "" && city(viewer.city) === city(student.city);
  return {
    canTeachYou: student.teachingSkills.filter((skill) => wanted.has(skill.skillId)),
    wantsToLearn: student.learningSkills.filter((skill) => offered.has(skill.skillId)),
    commonFormats: student.formats.filter(
      (format) => viewer.formats.includes(format) && (format !== "OFFLINE" || sameCity),
    ),
  };
}

export type DiscoveryProblem = "login" | "unverified" | "notFound" | "invalid" | "unavailable";

export function discoveryProblem(error: unknown): DiscoveryProblem {
  if (error instanceof AuthError) {
    if (error.status === 401) return "login";
    if (error.status === 403 && error.code === "email_not_verified") return "unverified";
    if (error.status === 404) return "notFound";
    if (error.status === 400 || error.status === 422) return "invalid";
  }
  return "unavailable";
}

export function searchStudents(filters: SearchFilters, signal?: AbortSignal) {
  const query = searchQueryString(filters);
  return authRequest<ResultPage<StudentCard>>(`students${query ? `?${query}` : ""}`, { signal });
}

export function mutualMatches(page: number, signal?: AbortSignal) {
  return authRequest<MatchPage>(`me/matches${page > 1 ? `?page=${page}` : ""}`, { signal });
}

export async function studentProfile(id: string, signal?: AbortSignal) {
  const data = await authRequest<{ student: StudentProfile }>(
    `students/${encodeURIComponent(id)}`,
    { signal },
  );
  return data.student;
}

export async function skillCategories(signal?: AbortSignal) {
  const data = await authRequest<{ items: SkillCategory[] }>("skill-categories", { signal });
  return data.items;
}
