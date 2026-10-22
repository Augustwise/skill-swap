import { AuthError, authRequest, writeRequest } from "./AuthApi";
import { isUuid } from "./AppRoutes";
import {
  pageCount,
  type LessonFormat,
  type Person,
  type ResultPage,
  type StudentSummary,
} from "./DiscoveryApi";
import {
  exchangeHref,
  requestHref,
  type ExchangeStatus,
  type RequestStatus,
  type RequestTerms,
} from "./ExchangeApi";

export type MessagePreview = { id: string; author: Person | null; body: string; createdAt: string };
export type Conversation = {
  id: string;
  type: "REQUEST" | "EXCHANGE";
  requestId: string | null;
  exchangeId: string | null;
  requestStatus: RequestStatus | null;
  exchangeStatus: ExchangeStatus | null;
  format: LessonFormat;
  partner: StudentSummary;
  requester: StudentSummary;
  recipient: StudentSummary;
  requesterTeaches: RequestTerms;
  recipientTeaches: RequestTerms;
  lastMessage: MessagePreview | null;
  lastMessageAt: string | null;
  unreadCount: number;
  canSend: boolean;
  blockedByMe: boolean;
  createdAt: string;
};
export type ChatMessage = {
  id: string;
  conversationId: string;
  author: Person | null;
  type: "TEXT" | "FILE" | "SESSION_PROPOSAL" | "SYSTEM";
  body: string;
  createdAt: string;
  editedAt: string | null;
  // When the other participant read it.
  readAt: string | null;
};
export type MessagePage = ResultPage<ChatMessage> & { pollIntervalSeconds: number };
export type MessagePart = { text: string; href?: string };

export const MAX_CHAT_MESSAGE_LENGTH = 2000;
export const MESSAGE_PAGE_SIZE = 20;
export const POLL_INTERVAL_SECONDS = 5;
const MAX_POLL_PAGES = 5;

export function messagesHref(conversationId?: string) {
  return conversationId ? `/messages/${encodeURIComponent(conversationId)}` : "/messages";
}

// The request or exchange the chat belongs to; an accepted request's chat moves to its exchange.
export function conversationContext(conversation: Conversation) {
  if (conversation.exchangeId) {
    return {
      label: "Обмін",
      href: exchangeHref(conversation.exchangeId),
      badge: conversation.exchangeStatus
        ? { status: conversation.exchangeStatus, kind: "exchange" as const }
        : null,
    };
  }
  return {
    label: "Запит на обмін",
    href: conversation.requestId ? requestHref(conversation.requestId) : "/requests",
    badge: conversation.requestStatus
      ? { status: conversation.requestStatus, kind: "request" as const }
      : null,
  };
}

export function messagePreview(conversation: Conversation, viewerId: string) {
  const last = conversation.lastMessage;
  if (!last) return "Повідомлень ще немає";
  const text = last.body.replace(/\s+/g, " ").trim();
  return last.author?.id === viewerId ? `Ти: ${text}` : text;
}

export function messageLength(text: string) {
  return Array.from(text.trim()).length;
}

// Mirrors the server: the trimmed text must have 1–2000 characters.
export function messageError(text: string) {
  const length = messageLength(text);
  if (length === 0) return "Напиши повідомлення.";
  if (length > MAX_CHAT_MESSAGE_LENGTH) {
    return `Повідомлення — до ${MAX_CHAT_MESSAGE_LENGTH} символів.`;
  }
  return "";
}

function chronological(a: ChatMessage, b: ChatMessage) {
  const time = Date.parse(a.createdAt) - Date.parse(b.createdAt);
  if (time !== 0) return time;
  return a.id < b.id ? -1 : a.id > b.id ? 1 : 0;
}

// Adds new messages and replaces known ones (with a fresh read time) in chronological order.
export function mergeMessages(current: ChatMessage[], incoming: ChatMessage[]) {
  if (incoming.length === 0) return current;
  const byId = new Map(current.map((message) => [message.id, message]));
  for (const message of incoming) byId.set(message.id, message);
  return [...byId.values()].sort(chronological);
}

// Polling starts before your earliest unread message among the recent ones, so its read
// mark refreshes together with new messages; otherwise only newer messages are fetched.
export function pollAnchor(messages: ChatMessage[], viewerId: string) {
  const start = Math.max(messages.length - MESSAGE_PAGE_SIZE, 1);
  for (let index = start; index < messages.length; index++) {
    const message = messages[index];
    if (message.author?.id === viewerId && !message.readAt) return messages[index - 1].id;
  }
  return messages.at(-1)?.id ?? "";
}

export function isIncomingUnread(message: ChatMessage, viewerId: string) {
  return message.author !== null && message.author.id !== viewerId && !message.readAt;
}

// The page to open on: the newest messages, without a nearly empty last page.
export function firstShownPage(total: number, pageSize = MESSAGE_PAGE_SIZE) {
  const pages = pageCount(total, pageSize);
  const lastCount = total - (pages - 1) * pageSize;
  return pages > 1 && lastCount < pageSize / 2 ? pages - 1 : pages;
}

const linkPattern = /(?:https?:\/\/|www\.)[^\s<>"]+/gi;
const trailingPunctuation = ".,!?;:'\"»]}";

function trimLink(link: string) {
  let value = link;
  while (value) {
    const last = value.at(-1) ?? "";
    const unbalanced = last === ")" && value.split(")").length > value.split("(").length;
    if (!trailingPunctuation.includes(last) && !unbalanced) break;
    value = value.slice(0, -1);
  }
  return value;
}

function safeHref(link: string) {
  const href = /^www\./i.test(link) ? `https://${link}` : link;
  try {
    const url = new URL(href);
    return (url.protocol === "http:" || url.protocol === "https:") && url.hostname ? href : "";
  } catch {
    return "";
  }
}

// Splits a message into plain text and http(s) links; nothing else becomes markup.
export function messageParts(body: string): MessagePart[] {
  const parts: MessagePart[] = [];
  let position = 0;
  for (const match of body.matchAll(linkPattern)) {
    const link = trimLink(match[0]);
    const href = link.length > 4 ? safeHref(link) : "";
    if (!href) continue;
    if (match.index > position) parts.push({ text: body.slice(position, match.index) });
    parts.push({ text: link, href });
    position = match.index + link.length;
  }
  if (position < body.length) parts.push({ text: body.slice(position) });
  return parts;
}

// A fresh first page goes on top; chats from older pages stay below it.
export function withFirstPage(current: Conversation[], first: Conversation[]) {
  const fresh = new Set(first.map((conversation) => conversation.id));
  return [...first, ...current.filter((conversation) => !fresh.has(conversation.id))];
}

export function withNextPage(current: Conversation[], next: Conversation[]) {
  const known = new Set(current.map((conversation) => conversation.id));
  return [...current, ...next.filter((conversation) => !known.has(conversation.id))];
}

export function listConversations(page: number, signal?: AbortSignal) {
  return authRequest<ResultPage<Conversation>>(
    `me/conversations${page > 1 ? `?page=${page}` : ""}`,
    { signal },
  );
}

// Unread messages in your most recently active chats, for the navigation badge.
export async function unreadMessageCount(signal?: AbortSignal) {
  const page = await listConversations(1, signal);
  return page.items.reduce((sum, conversation) => sum + conversation.unreadCount, 0);
}

async function readConversation(path: string, id: string, code: string, signal?: AbortSignal) {
  if (!isUuid(id)) throw new AuthError(404, code);
  const data = await authRequest<{ conversation: Conversation }>(path, { signal });
  return data.conversation;
}

export function conversationDetails(id: string, signal?: AbortSignal) {
  return readConversation(
    `conversations/${encodeURIComponent(id)}`,
    id,
    "conversation_not_found",
    signal,
  );
}

export function requestConversation(requestId: string, signal?: AbortSignal) {
  return readConversation(
    `exchange-requests/${encodeURIComponent(requestId)}/conversation`,
    requestId,
    "request_not_found",
    signal,
  );
}

export function exchangeConversation(exchangeId: string, signal?: AbortSignal) {
  return readConversation(
    `exchanges/${encodeURIComponent(exchangeId)}/conversation`,
    exchangeId,
    "exchange_not_found",
    signal,
  );
}

function messagesPath(conversationId: string) {
  return `conversations/${encodeURIComponent(conversationId)}/messages`;
}

export async function listMessages(conversationId: string, page: number, signal?: AbortSignal) {
  if (!isUuid(conversationId)) throw new AuthError(404, "conversation_not_found");
  return authRequest<MessagePage>(
    `${messagesPath(conversationId)}${page > 1 ? `?page=${page}` : ""}`,
    { signal },
  );
}

// The first screen of a chat: its newest messages and the page they start on.
export async function latestMessages(conversationId: string, signal?: AbortSignal) {
  const first = await listMessages(conversationId, 1, signal);
  const start = firstShownPage(first.total, first.pageSize);
  const last = pageCount(first.total, first.pageSize);
  let items = start === 1 ? first.items : [];
  for (let page = Math.max(start, 2); page <= last; page++) {
    items = mergeMessages(items, (await listMessages(conversationId, page, signal)).items);
  }
  return { items, firstPage: start, pollIntervalSeconds: first.pollIntervalSeconds };
}

// One polling step: messages after the anchor, page by page while the server has more.
export async function newMessages(
  conversationId: string,
  current: ChatMessage[],
  viewerId: string,
  signal?: AbortSignal,
) {
  let after = pollAnchor(current, viewerId);
  let items: ChatMessage[] = [];
  let pollIntervalSeconds = POLL_INTERVAL_SECONDS;
  for (let step = 0; step < MAX_POLL_PAGES; step++) {
    const page = after
      ? await authRequest<MessagePage>(
          `${messagesPath(conversationId)}?after=${encodeURIComponent(after)}`,
          { signal },
        )
      : await listMessages(conversationId, 1, signal);
    items = mergeMessages(items, page.items);
    pollIntervalSeconds = page.pollIntervalSeconds || POLL_INTERVAL_SECONDS;
    if (page.items.length === 0 || page.total <= page.items.length) break;
    after = page.items[page.items.length - 1].id;
  }
  return { items, pollIntervalSeconds };
}

export async function sendMessage(conversationId: string, body: string, csrfToken: string) {
  const data = await writeRequest<{ message: ChatMessage }>(
    messagesPath(conversationId),
    { body: JSON.stringify({ body: body.trim() }) },
    csrfToken,
  );
  return data.message;
}

export function markConversationRead(conversationId: string, csrfToken: string) {
  return writeRequest<{ conversation: Conversation; readCount: number }>(
    `conversations/${encodeURIComponent(conversationId)}/read`,
    {},
    csrfToken,
  );
}

const chatMessages: Record<string, string> = {
  conversation_not_found: "Розмову не знайдено або ти не береш у ній участі.",
  conversation_blocked:
    "Нові повідомлення в цій розмові недоступні: хтось із вас заблокував іншого або акаунт призупинено. Історія залишається доступною.",
  request_not_found: "Запит не знайдено або ти не береш у ньому участі.",
  exchange_not_found: "Обмін не знайдено або ти не береш у ньому участі.",
  validation_failed: `Повідомлення має містити від 1 до ${MAX_CHAT_MESSAGE_LENGTH} символів без службових символів.`,
};

export function chatErrorMessage(error: unknown) {
  if (error instanceof AuthError) return chatMessages[error.code] ?? error.message;
  return "Не вдалося зв’язатися із сервером. Перевір з’єднання і спробуй ще раз.";
}
