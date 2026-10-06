export type Reference = { id: string; name: string };
export type AuthSession = {
  user: { email: string; emailVerified: boolean };
  csrfToken: string;
};
export type Skill = Reference & { categoryId: string };
export type SkillLevel = "BEGINNER" | "INTERMEDIATE" | "ADVANCED";
export type SelectedSkill = Skill & { level: SkillLevel };
export type Profile = {
  id: string;
  firstName: string;
  lastName: string;
  university: Reference;
  faculty: Reference | null;
  course: number | null;
  city: string;
  bio: string;
  formats: string[];
  teachingSkills: (Omit<SelectedSkill, "id"> & { skillId: string })[];
  learningSkills: (Omit<SelectedSkill, "id"> & { skillId: string })[];
};
export type ProfileFields = {
  firstName: string;
  lastName: string;
  facultyId: string;
  course: string;
  city: string;
  bio: string;
};

export class OnboardingError extends Error {
  constructor(
    public status: number,
    public fields: Record<string, string> = {},
    public code?: string,
  ) {
    super(
      status === 401
        ? "Сесія завершилася. Увійди в акаунт, щоб продовжити."
        : status === 403
          ? "Не вдалося підтвердити сесію. Онови сторінку й спробуй ще раз."
          : "Не вдалося зберегти дані. Спробуй ще раз.",
    );
  }
}

export async function onboardingRequest<T>(
  path: string,
  options: RequestInit = {},
  csrfToken?: string,
): Promise<T> {
  const response = await fetch(`/api/v1/${path}`, {
    ...options,
    credentials: "same-origin",
    headers: {
      "Content-Type": "application/json",
      ...(csrfToken ? { "X-CSRF-Token": csrfToken } : {}),
    },
  });
  if (!response.ok) {
    const problem = await response.json().catch(() => null);
    throw new OnboardingError(response.status, problem?.error?.fields, problem?.error?.code);
  }
  return response.json() as Promise<T>;
}

export function profileFields(profile: Profile): ProfileFields {
  return {
    firstName: profile.firstName,
    lastName: profile.lastName,
    facultyId: profile.faculty?.id ?? "",
    course: profile.course?.toString() ?? "",
    city: profile.city,
    bio: profile.bio,
  };
}

export function selectedSkills(items: Profile["teachingSkills"]): SelectedSkill[] {
  return items.map(({ skillId, ...skill }) => ({ ...skill, id: skillId }));
}
