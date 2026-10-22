import { authRequest, writeRequest } from "./AuthApi";
import { isUuid } from "./AppRoutes";
import { readPage, type Person, type ResultPage } from "./DiscoveryApi";
import {
  invalidFields,
  requireUuid,
  rejectFields,
  textError,
  type Report,
  type ReportStatus,
} from "./SafetyApi";

export type ModerationAction = {
  id: string;
  reportId: string | null;
  moderator: Person;
  subject: Person | null;
  skill: { id: string; name: string } | null;
  action:
    | "DISMISS_REPORT"
    | "RESOLVE_REPORT"
    | "SUSPEND_ACCOUNT"
    | "RESTORE_ACCOUNT"
    | "CREATE_SKILL"
    | "UPDATE_SKILL";
  reason: string;
  createdAt: string;
};
export type AdminReport = Report & {
  reporter: Person;
  reportedUser: Person & { accountStatus: "ACTIVE" | "SUSPENDED" };
  decision: ModerationAction | null;
};
export type ReportDetails = { report: AdminReport; history: ModerationAction[] };
export type CatalogSkill = {
  id: string;
  categoryId: string;
  categoryName: string;
  name: string;
  slug: string;
  description: string;
  isActive: boolean;
};
export type SkillDraft = Omit<CatalogSkill, "id" | "categoryName">;
export type AdminTab = "reports" | "skills" | "history";
export const adminTabs: Record<AdminTab, string> = {
  reports: "Скарги",
  skills: "Каталог навичок",
  history: "Журнал дій",
};
export const actionLabels: Record<ModerationAction["action"], string> = {
  DISMISS_REPORT: "Скаргу відхилено",
  RESOLVE_REPORT: "Порушення підтверджено",
  SUSPEND_ACCOUNT: "Акаунт призупинено",
  RESTORE_ACCOUNT: "Акаунт відновлено",
  CREATE_SKILL: "Навичку створено",
  UPDATE_SKILL: "Навичку оновлено",
};
export type Decision = "DISMISS" | "RESOLVE" | "SUSPEND";
export const decisions: Record<Decision, string> = {
  DISMISS: "Відхилити скаргу",
  RESOLVE: "Підтвердити порушення",
  SUSPEND: "Підтвердити й призупинити акаунт",
};
export function adminTab(value: string | null | undefined): AdminTab {
  return value && Object.hasOwn(adminTabs, value) ? (value as AdminTab) : "reports";
}
export function reportStatus(value: string | null | undefined): ReportStatus | "" {
  return value && ["OPEN", "RESOLVED", "DISMISSED"].includes(value) ? (value as ReportStatus) : "";
}
export function adminHref(
  tab: AdminTab = "reports",
  params: { page?: number; status?: string; q?: string; userId?: string } = {},
) {
  const query = new URLSearchParams();
  if (tab !== "reports") query.set("tab", tab);
  if (tab === "reports") query.set("status", reportStatus(params.status));
  if (tab === "skills" && params.q?.trim())
    query.set("q", Array.from(params.q.trim()).slice(0, 100).join(""));
  if (tab === "history" && params.userId) {
    requireUuid(params.userId, "user_not_found");
    query.set("userId", params.userId);
  }
  const page = readPage(String(params.page ?? 1));
  if (page > 1) query.set("page", String(page));
  return `/admin${query.size ? `?${query}` : ""}`;
}
export function adminReports(status: string, page: number, signal?: AbortSignal) {
  return authRequest<ResultPage<AdminReport>>(
    `admin/reports?status=${reportStatus(status)}&page=${readPage(String(page))}`,
    { signal },
  );
}
export async function adminReport(id: string, signal?: AbortSignal) {
  requireUuid(id, "report_not_found");
  return authRequest<ReportDetails>(`admin/reports/${id}`, { signal });
}
export async function resolveReport(id: string, decision: Decision, reason: string, token: string) {
  requireUuid(id, "report_not_found");
  rejectFields(
    invalidFields({
      reason: textError(reason, 1000),
      decision: Object.hasOwn(decisions, decision) ? "" : "Обери рішення.",
    }),
  );
  return writeRequest<{ report: AdminReport }>(
    `admin/reports/${id}/resolve`,
    { body: JSON.stringify({ decision, reason: reason.trim() }) },
    token,
  );
}
export async function moderateAccount(
  id: string,
  action: "suspend" | "restore",
  reason: string,
  token: string,
) {
  requireUuid(id, "user_not_found");
  rejectFields(invalidFields({ reason: textError(reason, 1000) }));
  return writeRequest<{
    user: Person & { accountStatus: "ACTIVE" | "SUSPENDED" };
    action: ModerationAction | null;
  }>(`admin/users/${id}/${action}`, { body: JSON.stringify({ reason: reason.trim() }) }, token);
}
export function moderationLog(page: number, userId = "", signal?: AbortSignal) {
  if (userId) requireUuid(userId, "user_not_found");
  return authRequest<ResultPage<ModerationAction>>(
    `admin/moderation-actions?page=${readPage(String(page))}${userId ? `&userId=${userId}` : ""}`,
    { signal },
  );
}
export function catalogSkills(q: string, page: number, signal?: AbortSignal) {
  const query = new URLSearchParams({
    q: Array.from(q.trim()).slice(0, 100).join(""),
    page: String(readPage(String(page))),
  });
  return authRequest<ResultPage<CatalogSkill>>(`admin/skills?${query}`, { signal });
}
export function skillFields(draft: SkillDraft) {
  return invalidFields({
    categoryId: isUuid(draft.categoryId) ? "" : "Обери категорію.",
    name:
      textError(draft.name, 150) ||
      (/[\r\n\t]/.test(draft.name.trim()) ? "Введи назву в одному рядку." : ""),
    slug:
      /^[a-z0-9]+(-[a-z0-9]+)*$/.test(draft.slug.trim()) && draft.slug.trim().length <= 150
        ? ""
        : "Латинські літери, цифри й одинарні дефіси, до 150 символів.",
    description: textError(draft.description, 500, false),
  });
}
export async function saveSkill(id: string | null, draft: SkillDraft, token: string) {
  if (id) requireUuid(id, "skill_not_found");
  rejectFields(skillFields(draft));
  return writeRequest<{ skill: CatalogSkill }>(
    id ? `admin/skills/${id}` : "admin/skills",
    {
      method: id ? "PATCH" : "POST",
      body: JSON.stringify({
        ...draft,
        name: draft.name.trim(),
        slug: draft.slug.trim(),
        description: draft.description.trim(),
      }),
    },
    token,
  );
}
