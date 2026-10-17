import { AuthError, authRequest } from "./AuthApi";
import type { Profile, ProfileFields, SkillLevel } from "./OnboardingApi";

export type SkillList = "teaching" | "learning";
export type ProfileDraft = ProfileFields & { formats: string[] };

export const profileMessages: Record<string, string> = {
  firstName: "Введи ім’я: від 1 до 100 символів.",
  lastName: "Введи прізвище: від 1 до 100 символів.",
  facultyId: "Обери факультет свого університету.",
  course: "Обери курс від 1 до 6.",
  city: "Вкажи місто до 100 символів. Для офлайн-занять місто обов’язкове.",
  bio: "Опис має містити до 600 символів.",
  formats: "Обери онлайн- та/або офлайн-формат.",
};

export const skillLevels: Record<SkillLevel, string> = {
  BEGINNER: "Початковий",
  INTERMEDIATE: "Середній",
  ADVANCED: "Просунутий",
};

const length = (value: string) => Array.from(value).length;
const invalidLine = (value: string) => /[\r\n\p{Cc}]/u.test(value);

export function validateProfile(draft: ProfileDraft) {
  const errors: Record<string, string> = {};
  for (const field of ["firstName", "lastName"] as const) {
    const value = draft[field].trim();
    if (!value || length(value) > 100 || invalidLine(value)) errors[field] = profileMessages[field];
  }
  if (draft.course && !/^[1-6]$/.test(draft.course)) errors.course = profileMessages.course;
  if (
    length(draft.city.trim()) > 100 ||
    invalidLine(draft.city.trim()) ||
    (draft.formats.includes("OFFLINE") && !draft.city.trim())
  )
    errors.city = profileMessages.city;
  if (length(draft.bio.trim()) > 600) errors.bio = profileMessages.bio;
  if (
    !draft.formats.length ||
    draft.formats.some((item) => !["ONLINE", "OFFLINE"].includes(item))
  ) {
    errors.formats = profileMessages.formats;
  }
  return errors;
}

export function profileErrors(error: unknown) {
  return error instanceof AuthError
    ? Object.fromEntries(
        Object.keys(error.fields).map((field) => [
          field,
          profileMessages[field] ?? "Перевір значення цього поля.",
        ]),
      )
    : {};
}

export async function saveProfile(draft: ProfileDraft, csrfToken: string) {
  const { profile } = await authRequest<{ profile: Profile }>(
    "me/profile",
    {
      method: "PATCH",
      body: JSON.stringify({
        ...draft,
        firstName: draft.firstName.trim(),
        lastName: draft.lastName.trim(),
        city: draft.city.trim(),
        bio: draft.bio.trim(),
        facultyId: draft.facultyId || null,
        course: draft.course ? Number(draft.course) : null,
      }),
    },
    csrfToken,
  );
  return profile;
}

export async function changeProfileSkill(
  list: SkillList,
  method: "POST" | "PATCH" | "DELETE",
  skillId: string,
  csrfToken: string,
  level?: SkillLevel,
) {
  const path = `me/${list}-skills${method === "POST" ? "" : `/${encodeURIComponent(skillId)}`}`;
  const { profile } = await authRequest<{ profile: Profile }>(
    path,
    {
      method,
      ...(method !== "DELETE" && {
        body: JSON.stringify(method === "POST" ? { skillId, level } : { level }),
      }),
    },
    csrfToken,
  );
  return profile;
}
