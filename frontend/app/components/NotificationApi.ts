import { AuthError, authRequest, writeRequest } from "./AuthApi";
import { isUuid } from "./AppRoutes";
import { messagesHref } from "./ChatApi";
import { readPage, type Person, type ResultPage } from "./DiscoveryApi";
import { exchangeHref, requestHref } from "./ExchangeApi";
import { sessionHref } from "./SessionApi";

export type NotificationEvent =
  "EXCHANGE_REQUEST" | "REQUEST_RESPONSE" | "CHAT_MESSAGE" | "SESSION_UPDATED";
export type NotificationTarget = {
  kind: string;
  id: string;
  requestId: string | null;
  exchangeId: string | null;
  conversationId: string | null;
  sessionId: string | null;
};
export type AppNotification = {
  id: string;
  eventType: NotificationEvent;
  title: string;
  body: string | null;
  actor: Person | null;
  target: NotificationTarget;
  requestId: string | null;
  exchangeId: string | null;
  conversationId: string | null;
  messageId: string | null;
  sessionId: string | null;
  isRead: boolean;
  readAt: string | null;
  createdAt: string;
};
export type NotificationPage = ResultPage<AppNotification> & { unreadCount: number };
export type NotificationFilters = { unread: boolean; page: number };

export const emptyNotificationFilters: NotificationFilters = { unread: false, page: 1 };

export function readNotificationFilters(params: {
  get(name: string): string | null;
}): NotificationFilters {
  return { unread: params.get("filter") === "unread", page: readPage(params.get("page")) };
}

export function notificationsHref(filters: Partial<NotificationFilters> = {}) {
  const { unread, page } = { ...emptyNotificationFilters, ...filters };
  const params = new URLSearchParams();
  if (unread) params.set("filter", "unread");
  if (page > 1) params.set("page", String(page));
  const query = params.toString();
  return query ? `/notifications?${query}` : "/notifications";
}

export function notificationHref(notification: Pick<AppNotification, "target">) {
  const { kind, id, exchangeId } = notification.target;
  switch (kind) {
    case "EXCHANGE_REQUEST":
      if (isUuid(id)) return requestHref(id);
      break;
    case "EXCHANGE":
      if (isUuid(id)) return exchangeHref(id);
      break;
    case "CONVERSATION":
      if (isUuid(id)) return messagesHref(id);
      break;
    case "SESSION":
      if (isUuid(id)) return sessionHref(id);
      if (exchangeId && isUuid(exchangeId)) return exchangeHref(exchangeId);
      break;
  }
  return "/notifications";
}

export function listNotifications(filters: NotificationFilters, signal?: AbortSignal) {
  const params = new URLSearchParams();
  if (filters.unread) params.set("unreadOnly", "true");
  if (filters.page > 1) params.set("page", String(filters.page));
  const query = params.toString();
  return authRequest<NotificationPage>(`me/notifications${query ? `?${query}` : ""}`, { signal });
}

export async function unreadNotificationCount(signal?: AbortSignal) {
  const page = await listNotifications({ unread: true, page: 1 }, signal);
  return page.unreadCount;
}

export function markNotificationRead(id: string, csrfToken: string) {
  if (!isUuid(id)) return Promise.reject(new AuthError(404, "notification_not_found"));
  return writeRequest<{ notification: AppNotification; unreadCount: number }>(
    `me/notifications/${encodeURIComponent(id)}/read`,
    {},
    csrfToken,
  );
}

export function markAllNotificationsRead(csrfToken: string) {
  return writeRequest<{ markedCount: number; unreadCount: number }>(
    "me/notifications/read-all",
    {},
    csrfToken,
  );
}

export function notificationErrorMessage(error: unknown) {
  if (error instanceof AuthError) {
    return error.code === "notification_not_found"
      ? "Сповіщення не знайдено. Ми оновили список."
      : error.message;
  }
  return "Не вдалося зв’язатися із сервером. Перевір з’єднання і спробуй ще раз.";
}
