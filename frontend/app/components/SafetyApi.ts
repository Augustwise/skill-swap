import { AuthError, authRequest, requestError, writeRequest } from "./AuthApi";
import { isUuid } from "./AppRoutes";
import { passwordError } from "./AuthValidation";
import { readPage, type Person, type ResultPage } from "./DiscoveryApi";

export type Privacy = {
  isDiscoverable: boolean;
  acceptsRequests: boolean;
  requestScope: "ANYONE" | "SAME_UNIVERSITY";
};
export type Block = { user: Person; createdAt: string };
export type ReportStatus = "OPEN" | "RESOLVED" | "DISMISSED";
export type Report = {
  id: string;
  reportedUser: Person;
  reason: string;
  status: ReportStatus;
  createdAt: string;
  resolvedAt: string | null;
};
export type SettingsTab = "privacy" | "security" | "blocks" | "reports";
export const settingsTabs: Record<SettingsTab, string> = {
  privacy: "Приватність",
  security: "Безпека акаунта",
  blocks: "Заблоковані",
  reports: "Мої скарги",
};
export const reportStatuses: Record<ReportStatus, string> = {
  OPEN: "Очікує розгляду",
  RESOLVED: "Розглянуто",
  DISMISSED: "Відхилено",
};
export function settingsTab(value: string | null | undefined): SettingsTab {
  return value && Object.hasOwn(settingsTabs, value) ? (value as SettingsTab) : "privacy";
}
export function settingsHref(tab: SettingsTab = "privacy", page = 1) {
  const query = new URLSearchParams();
  if (tab !== "privacy") query.set("tab", tab);
  if (readPage(String(page)) > 1) query.set("page", String(page));
  return `/settings${query.size ? `?${query}` : ""}`;
}
export function textError(value: string, max: number, required = true) {
  const text = value.trim();
  if (required && !text) return "Заповни це поле.";
  if (Array.from(text).length > max) return `Введи не більше ${max} символів.`;
  if (/[\u0000-\u0008\u000b\u000c\u000e-\u001f\u007f-\u009f]/u.test(text))
    return "Прибери недопустимі керівні символи.";
  return "";
}
export function invalidFields(fields: Record<string, string>) {
  return Object.fromEntries(Object.entries(fields).filter(([, value]) => value));
}
export function passwordFields(currentPassword: string, newPassword: string, confirmation: string) {
  return invalidFields({
    currentPassword: currentPassword ? "" : "Введи поточний пароль.",
    newPassword:
      passwordError(newPassword) ||
      (newPassword === currentPassword ? "Новий пароль має відрізнятися від поточного." : ""),
    confirmation: newPassword === confirmation ? "" : "Паролі не збігаються.",
  });
}
export function rejectFields(fields: Record<string, string>) {
  if (Object.keys(fields).length) throw new AuthError(422, "validation_failed", fields);
}
export function requireUuid(id: string, code = "student_not_found") {
  if (!isUuid(id)) throw new AuthError(404, code);
}
export async function privacySettings(signal?: AbortSignal) {
  return (await authRequest<{ privacy: Privacy }>("me/privacy", { signal })).privacy;
}
export async function updatePrivacy(privacy: Privacy, token: string) {
  rejectFields(
    invalidFields({
      isDiscoverable: typeof privacy.isDiscoverable === "boolean" ? "" : "Обери видимість профілю.",
      acceptsRequests:
        typeof privacy.acceptsRequests === "boolean" ? "" : "Обери доступність запитів.",
      requestScope: ["ANYONE", "SAME_UNIVERSITY"].includes(privacy.requestScope)
        ? ""
        : "Обери коло студентів.",
    }),
  );
  return (
    await writeRequest<{ privacy: Privacy }>(
      "me/privacy",
      {
        method: "PATCH",
        body: JSON.stringify(privacy),
      },
      token,
    )
  ).privacy;
}
export async function changePassword(currentPassword: string, newPassword: string, token: string) {
  rejectFields(passwordFields(currentPassword, newPassword, newPassword));
  return writeRequest(
    "auth/change-password",
    { body: JSON.stringify({ currentPassword, newPassword }) },
    token,
  );
}
export async function deleteAccount(password: string, token: string) {
  rejectFields(invalidFields({ password: password ? "" : "Введи поточний пароль." }));
  return writeRequest<void>(
    "me/account",
    { method: "DELETE", body: JSON.stringify({ password }) },
    token,
  );
}
export async function blockStudent(id: string, token: string) {
  requireUuid(id);
  return writeRequest<{ block: Block; cancelledExchanges: number; closedRequests: number }>(
    `students/${id}/block`,
    {},
    token,
  );
}
export async function unblockStudent(id: string, token: string) {
  requireUuid(id);
  return writeRequest<void>(`students/${id}/block`, { method: "DELETE" }, token);
}
export function blockedStudents(page: number, signal?: AbortSignal) {
  return authRequest<ResultPage<Block>>(`me/blocks?page=${readPage(String(page))}`, { signal });
}
export function myReports(page: number, signal?: AbortSignal) {
  return authRequest<ResultPage<Report>>(`me/reports?page=${readPage(String(page))}`, { signal });
}
export async function reportStudent(id: string, reason: string, token: string) {
  requireUuid(id);
  rejectFields(invalidFields({ reason: textError(reason, 1000) }));
  return writeRequest<{ report: Report }>(
    "reports",
    { body: JSON.stringify({ reportedUserId: id, reason: reason.trim() }) },
    token,
  );
}
export function safetyError(error: unknown): string {
  if (error instanceof AuthError) {
    const messages: Record<string, string> = {
      admin_required: "Цей розділ доступний лише адміністраторам.",
      student_not_found: "Студент недоступний. Онови сторінку.",
      user_not_found: "Акаунт не знайдено або його видалено.",
      report_not_found: "Скаргу не знайдено.",
      skill_not_found: "Навичку не знайдено. Онови каталог.",
      report_already_open:
        "У тебе вже є відкрита скарга на цього студента. Її статус є в налаштуваннях.",
      report_already_resolved: "Інший адміністратор уже розглянув цю скаргу. Онови дані.",
      self_moderation: "Не можна призупинити або відновити власний акаунт.",
      skill_already_exists: "Назва в цій категорії або ідентифікатор уже зайняті.",
      validation_failed: "Перевір заповнені поля.",
      login_locked: `Забагато невдалих спроб. ${error.retryAfter ? `Повтори через ${Math.ceil(error.retryAfter / 60)} хв.` : "Спробуй пізніше."}`,
    };
    if (messages[error.code]) return messages[error.code];
  }
  return requestError(error);
}
export function safetyFields(error: unknown) {
  if (!(error instanceof AuthError) || error.code !== "validation_failed") return {};
  const messages: Record<string, string> = {
    currentPassword: "Поточний пароль неправильний.",
    password: "Пароль неправильний.",
    newPassword: error.fields.newPassword?.includes("differ")
      ? "Новий пароль має відрізнятися від поточного."
      : "Введи пароль від 12 символів і до 72 байтів UTF-8.",
    reason: "Введи причину від 1 до 1000 символів без керівних символів.",
    name: "Введи назву до 150 символів в одному рядку.",
    slug: "Використай малі латинські літери, цифри й одинарні дефіси, до 150 символів.",
    description: "Введи опис до 500 символів.",
    categoryId: "Обери категорію з каталогу.",
    requestScope: "Обери, хто може надсилати запити.",
    isDiscoverable: "Обери видимість профілю.",
    acceptsRequests: "Обери доступність запитів.",
    decision: "Обери рішення зі списку.",
    userId: "Не можна виконати цю дію щодо себе.",
  };
  return Object.fromEntries(
    Object.keys(error.fields).map((key) => [key, messages[key] ?? "Перевір значення поля."]),
  );
}
