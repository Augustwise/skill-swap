import assert from "node:assert/strict";
import { test } from "node:test";
import { loadSource } from "./load-source.mjs";

const chat = loadSource("app/components/ChatApi.ts");
const format = loadSource("app/components/ChatFormat.ts");
const notifications = loadSource("app/components/NotificationApi.ts");
const routes = loadSource("app/components/AppRoutes.ts");
const { AuthError } = loadSource("app/components/AuthApi.ts");

const olha = "60000000-0000-0000-0000-000000000001";
const taras = "60000000-0000-0000-0000-000000000002";
const requestId = "70000000-0000-0000-0000-000000000001";
const conversationId = "72000000-0000-0000-0000-000000000001";
const exchangeId = "80000000-0000-0000-0000-000000000001";
const notificationId = "74000000-0000-0000-0000-000000000001";

const id = (n) => `73000000-0000-0000-0000-${String(n).padStart(12, "0")}`;
const message = (n, author, readAt = null) => ({
  id: id(n),
  conversationId,
  author: author ? { id: author, firstName: "А", lastName: "Б" } : null,
  type: "TEXT",
  body: `Повідомлення ${n}`,
  createdAt: new Date(Date.UTC(2026, 9, 18, 10, n)).toISOString(),
  editedAt: null,
  readAt,
});
const person = (personId, firstName) => ({
  id: personId,
  firstName,
  lastName: "Тест",
  university: { id: "u", name: "КПІ" },
  city: "Київ",
});
const terms = (name) => ({
  skill: { skillId: name, categoryId: "c", name },
  teacherLevel: "ADVANCED",
  learnerLevel: "BEGINNER",
  sessions: 2,
  durationMinutes: 60,
});
const conversation = (overrides = {}) => ({
  id: conversationId,
  type: "REQUEST",
  requestId,
  exchangeId: null,
  requestStatus: "PENDING",
  exchangeStatus: null,
  format: "OFFLINE",
  partner: person(taras, "Тарас"),
  requester: person(taras, "Тарас"),
  recipient: person(olha, "Ольга"),
  requesterTeaches: terms("Photoshop"),
  recipientTeaches: terms("Гітара"),
  lastMessage: null,
  lastMessageAt: null,
  unreadCount: 0,
  canSend: true,
  createdAt: "2026-10-18T10:00:00Z",
  ...overrides,
});

function jsonResponse(body, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

const messagePage = (items, total = items.length) => ({
  items,
  page: 1,
  pageSize: 20,
  total,
  pollIntervalSeconds: 5,
});

test("a message is trimmed and limited to 2000 characters, counted as the server does", () => {
  assert.equal(chat.messageError("   \n  "), "Напиши повідомлення.");
  assert.equal(chat.messageError(" Привіт! "), "");
  assert.equal(chat.messageLength("  🎸🎸  "), 2);
  assert.equal(chat.messageError("🎸".repeat(2000)), "");
  assert.equal(chat.messageError("a".repeat(2001)), "Повідомлення — до 2000 символів.");
});

test("messages merge without duplicates, keep time order and take fresh read marks", () => {
  const first = message(1, olha);
  const second = message(2, taras);
  assert.equal(chat.mergeMessages([first], []).length, 1);
  const read = { ...first, readAt: "2026-10-18T10:05:00Z" };
  const merged = chat.mergeMessages([first, second], [message(3, olha), read]);
  assert.deepEqual(
    merged.map((item) => item.id),
    [id(1), id(2), id(3)],
  );
  assert.equal(merged[0].readAt, "2026-10-18T10:05:00Z");
  const sameTime = { ...message(4, olha), createdAt: second.createdAt };
  assert.deepEqual(
    chat.mergeMessages([sameTime], [second]).map((item) => item.id),
    [id(2), id(4)],
  );
});

test("polling asks for newer messages and refreshes your recent unread ones", () => {
  assert.equal(chat.pollAnchor([], olha), "");
  const read = [message(1, olha, "x"), message(2, taras), message(3, olha, "x")];
  assert.equal(chat.pollAnchor(read, olha), id(3));
  const unread = [message(1, taras), message(2, olha, "x"), message(3, olha), message(4, taras)];
  assert.equal(chat.pollAnchor(unread, olha), id(2));
  assert.equal(chat.pollAnchor([message(1, olha), message(2, taras)], olha), id(2));
  const long = Array.from({ length: 30 }, (_, index) => message(index + 1, olha));
  assert.equal(chat.pollAnchor(long, olha), id(10));
  assert.equal(chat.isIncomingUnread(message(1, taras), olha), true);
  assert.equal(chat.isIncomingUnread(message(1, olha), olha), false);
  assert.equal(chat.isIncomingUnread(message(1, null), olha), false);
});

test("a chat opens on its newest page without a nearly empty last page", () => {
  assert.equal(chat.firstShownPage(0), 1);
  assert.equal(chat.firstShownPage(20), 1);
  assert.equal(chat.firstShownPage(21), 1);
  assert.equal(chat.firstShownPage(35), 2);
  assert.equal(chat.firstShownPage(45), 2);
  assert.equal(chat.firstShownPage(60), 3);
});

test("only http and https addresses become links", () => {
  assert.deepEqual(chat.messageParts("Без посилань"), [{ text: "Без посилань" }]);
  assert.deepEqual(chat.messageParts("Зустріч: https://meet.example.com/abc."), [
    { text: "Зустріч: " },
    { text: "https://meet.example.com/abc", href: "https://meet.example.com/abc" },
    { text: "." },
  ]);
  assert.deepEqual(chat.messageParts("(див. www.example.com/a_(b))"), [
    { text: "(див. " },
    { text: "www.example.com/a_(b)", href: "https://www.example.com/a_(b)" },
    { text: ")" },
  ]);
  for (const text of ["javascript:alert(1)", "https://", "file:///etc/passwd", "<b>x</b>"]) {
    assert.deepEqual(chat.messageParts(text), [{ text }], text);
  }
});

test("a conversation shows its request or exchange and a preview from your side", () => {
  const pending = conversation();
  assert.deepEqual(chat.conversationContext(pending), {
    label: "Запит на обмін",
    href: `/requests/${requestId}`,
    badge: { status: "PENDING", kind: "request" },
  });
  assert.deepEqual(
    chat.conversationContext(
      conversation({ type: "EXCHANGE", exchangeId, exchangeStatus: "ACTIVE" }),
    ),
    {
      label: "Обмін",
      href: `/exchanges/${exchangeId}`,
      badge: { status: "ACTIVE", kind: "exchange" },
    },
  );
  assert.equal(chat.messagePreview(pending, olha), "Повідомлень ще немає");
  const lastMessage = { id: id(1), author: { id: olha }, body: "Привіт,\n  Тарасе", createdAt: "" };
  assert.equal(chat.messagePreview(conversation({ lastMessage }), olha), "Ти: Привіт, Тарасе");
  assert.equal(chat.messagePreview(conversation({ lastMessage }), taras), "Привіт, Тарасе");
  assert.equal(chat.messagesHref(), "/messages");
  assert.equal(chat.messagesHref(conversationId), `/messages/${conversationId}`);
});

test("a refreshed first page goes on top and older pages keep their chats", () => {
  const a = conversation({ id: "a" });
  const b = conversation({ id: "b" });
  const c = conversation({ id: "c" });
  assert.deepEqual(
    chat.withFirstPage([a, b, c], [c, a]).map((item) => item.id),
    ["c", "a", "b"],
  );
  assert.deepEqual(
    chat.withNextPage([a, b], [b, c]).map((item) => item.id),
    ["a", "b", "c"],
  );
});

test("a long chat opens with its last pages and polling follows the server cursor", async (t) => {
  const calls = [];
  t.mock.method(globalThis, "fetch", async (url) => {
    calls.push(url.replace(`/api/v1/conversations/${conversationId}/messages`, ""));
    if (url.endsWith("/messages")) return jsonResponse(messagePage([message(1, taras)], 45));
    if (url.endsWith("page=2")) return jsonResponse(messagePage([message(21, taras)], 45));
    if (url.endsWith("page=3")) return jsonResponse(messagePage([message(41, olha)], 45));
    if (url.includes(`after=${id(41)}`)) {
      const items = Array.from({ length: 20 }, (_, index) => message(42 + index, taras));
      return jsonResponse(messagePage(items, 21));
    }
    return jsonResponse(messagePage([message(62, taras)], 1));
  });
  const latest = await chat.latestMessages(conversationId);
  assert.equal(latest.firstPage, 2);
  assert.deepEqual(
    latest.items.map((item) => item.id),
    [id(21), id(41)],
  );
  const fresh = await chat.newMessages(conversationId, [message(41, olha, "x")], olha);
  assert.equal(fresh.items.length, 21);
  assert.equal(fresh.pollIntervalSeconds, 5);
  assert.deepEqual(calls, ["", "?page=2", "?page=3", `?after=${id(41)}`, `?after=${id(61)}`]);
});

test("an empty chat polls its first page, and broken ids never reach the server", async (t) => {
  const calls = [];
  t.mock.method(globalThis, "fetch", async (url) => {
    calls.push(url);
    return jsonResponse(messagePage([]));
  });
  assert.deepEqual((await chat.newMessages(conversationId, [], olha)).items, []);
  assert.deepEqual(calls, [`/api/v1/conversations/${conversationId}/messages`]);
  await assert.rejects(chat.listMessages("../me", 1), (error) => error.status === 404);
  await assert.rejects(
    chat.conversationDetails("x"),
    (error) => error.code === "conversation_not_found",
  );
  await assert.rejects(
    chat.requestConversation("x"),
    (error) => error.code === "request_not_found",
  );
  await assert.rejects(
    chat.exchangeConversation("x"),
    (error) => error.code === "exchange_not_found",
  );
  assert.equal(calls.length, 1);
});

test("conversation reads call the sprint 6 endpoints and sum unread messages", async (t) => {
  const calls = [];
  t.mock.method(globalThis, "fetch", async (url) => {
    calls.push(url);
    if (url.includes("me/conversations")) {
      return jsonResponse({
        items: [conversation({ unreadCount: 2 }), conversation({ id: "b", unreadCount: 3 })],
        page: 1,
        pageSize: 20,
        total: 2,
      });
    }
    return jsonResponse({ conversation: conversation() });
  });
  assert.equal(await chat.unreadMessageCount(), 5);
  await chat.listConversations(2);
  await chat.conversationDetails(conversationId);
  await chat.requestConversation(requestId);
  await chat.exchangeConversation(exchangeId);
  assert.deepEqual(calls, [
    "/api/v1/me/conversations",
    "/api/v1/me/conversations?page=2",
    `/api/v1/conversations/${conversationId}`,
    `/api/v1/exchange-requests/${requestId}/conversation`,
    `/api/v1/exchanges/${exchangeId}/conversation`,
  ]);
});

test("a message is sent trimmed with the CSRF token, refreshed once when stale", async (t) => {
  const calls = [];
  t.mock.method(globalThis, "fetch", async (url, options) => {
    calls.push({
      url,
      method: options.method,
      token: new Headers(options.headers).get("X-CSRF-Token"),
      body: options.body,
    });
    if (url.endsWith("auth/me")) return jsonResponse({ user: {}, csrfToken: "fresh" });
    if (calls.length === 1) {
      return jsonResponse({ error: { code: "csrf_invalid", message: "CSRF" } }, 403);
    }
    if (url.endsWith("/read")) return jsonResponse({ conversation: conversation(), readCount: 1 });
    return jsonResponse({ message: message(5, olha) }, 201);
  });
  const sent = await chat.sendMessage(conversationId, "  Привіт!\n", "stale");
  assert.equal(sent.id, id(5));
  await chat.markConversationRead(conversationId, "fresh");
  const messages = `/api/v1/conversations/${conversationId}/messages`;
  assert.deepEqual(calls, [
    { url: messages, method: "POST", token: "stale", body: '{"body":"Привіт!"}' },
    { url: "/api/v1/auth/me", method: undefined, token: null, body: undefined },
    { url: messages, method: "POST", token: "fresh", body: '{"body":"Привіт!"}' },
    {
      url: `/api/v1/conversations/${conversationId}/read`,
      method: "POST",
      token: "fresh",
      body: undefined,
    },
  ]);
});

test("chat and notification errors are explained in Ukrainian", () => {
  assert.match(chat.chatErrorMessage(new AuthError(409, "conversation_blocked")), /Історія/);
  assert.match(chat.chatErrorMessage(new AuthError(404, "conversation_not_found")), /не знайдено/);
  assert.match(chat.chatErrorMessage(new AuthError(422, "validation_failed")), /2000/);
  assert.match(chat.chatErrorMessage(new AuthError(401, "unauthenticated")), /Сесія/);
  assert.match(chat.chatErrorMessage(new TypeError("fetch failed")), /з’єднання/);
  assert.match(
    notifications.notificationErrorMessage(new AuthError(404, "notification_not_found")),
    /оновили/,
  );
});

test("chat times read as today, yesterday or a date", () => {
  const now = new Date(2026, 9, 18, 15, 0);
  const at = (day, hour = 9, month = 9, year = 2026) =>
    new Date(year, month, day, hour, 5).toISOString();
  assert.equal(format.formatTime(at(18)), "09:05");
  assert.equal(format.dayLabel(at(18), now), "Сьогодні");
  assert.equal(format.dayLabel(at(17, 23), now), "Учора");
  assert.equal(format.dayLabel(at(2), now), "2 жовтня");
  assert.match(format.dayLabel(at(30, 9, 11, 2025), now), /30 грудня 2025/);
  assert.equal(format.listTime(at(18), now), "09:05");
  assert.equal(format.listTime(at(17), now), "учора");
  assert.match(format.listTime(at(2), now), /^2 жовт/);
  assert.equal(format.dayLabel("not a date", now), "");
  const groups = format.groupByDay(
    [
      { ...message(1, olha), createdAt: at(17) },
      { ...message(2, taras), createdAt: at(18) },
      { ...message(3, olha), createdAt: at(18, 10) },
    ],
    now,
  );
  assert.deepEqual(
    groups.map((group) => [group.label, group.messages.length]),
    [
      ["Учора", 1],
      ["Сьогодні", 2],
    ],
  );
});

test("notification filters round-trip through the URL", () => {
  assert.deepEqual(
    notifications.readNotificationFilters(new URLSearchParams("filter=unread&page=3")),
    { unread: true, page: 3 },
  );
  assert.deepEqual(
    notifications.readNotificationFilters(new URLSearchParams("filter=all&page=-1")),
    notifications.emptyNotificationFilters,
  );
  assert.equal(notifications.notificationsHref(), "/notifications");
  assert.equal(
    notifications.notificationsHref({ unread: true, page: 2 }),
    "/notifications?filter=unread&page=2",
  );
});

test("a notification opens its request, exchange or chat", () => {
  const target = (kind, targetId, extra = {}) => ({
    target: {
      kind,
      id: targetId,
      requestId: null,
      exchangeId: null,
      conversationId: null,
      sessionId: null,
      ...extra,
    },
  });
  assert.equal(
    notifications.notificationHref(target("EXCHANGE_REQUEST", requestId)),
    `/requests/${requestId}`,
  );
  assert.equal(
    notifications.notificationHref(target("EXCHANGE", exchangeId)),
    `/exchanges/${exchangeId}`,
  );
  assert.equal(
    notifications.notificationHref(target("CONVERSATION", conversationId)),
    `/messages/${conversationId}`,
  );
  assert.equal(
    notifications.notificationHref(target("SESSION", notificationId, { exchangeId })),
    `/exchanges/${exchangeId}`,
  );
  assert.equal(notifications.notificationHref(target("SESSION", notificationId)), "/notifications");
  assert.equal(notifications.notificationHref(target("CONVERSATION", "../x")), "/notifications");
  assert.equal(notifications.notificationHref(target("UNKNOWN", requestId)), "/notifications");
});

test("notification reads and read marks call the sprint 6 endpoints", async (t) => {
  const calls = [];
  t.mock.method(globalThis, "fetch", async (url, options) => {
    calls.push({ url, method: options.method });
    if (url.endsWith("/read-all")) return jsonResponse({ markedCount: 2, unreadCount: 0 });
    if (url.endsWith("/read")) return jsonResponse({ notification: {}, unreadCount: 1 });
    return jsonResponse({ items: [], page: 1, pageSize: 20, total: 4, unreadCount: 2 });
  });
  assert.equal(await notifications.unreadNotificationCount(), 2);
  await notifications.listNotifications({ unread: false, page: 2 });
  await notifications.listNotifications({ unread: false, page: 1 });
  assert.equal((await notifications.markNotificationRead(notificationId, "t")).unreadCount, 1);
  assert.equal((await notifications.markAllNotificationsRead("t")).markedCount, 2);
  await assert.rejects(
    notifications.markNotificationRead("../all", "t"),
    (error) => error.code === "notification_not_found",
  );
  assert.deepEqual(calls, [
    { url: "/api/v1/me/notifications?unreadOnly=true", method: undefined },
    { url: "/api/v1/me/notifications?page=2", method: undefined },
    { url: "/api/v1/me/notifications", method: undefined },
    { url: `/api/v1/me/notifications/${notificationId}/read`, method: "POST" },
    { url: "/api/v1/me/notifications/read-all", method: "POST" },
  ]);
});

test("messages and notifications pages are allowed as return paths", () => {
  assert.equal(routes.safeAppPath("/messages", "/main"), "/messages");
  assert.equal(
    routes.safeAppPath(`/messages/${conversationId}`, "/main"),
    `/messages/${conversationId}`,
  );
  assert.equal(
    routes.safeAppPath("/notifications?filter=unread", "/main"),
    "/notifications?filter=unread",
  );
  for (const unsafe of ["/messages/not-a-uuid", `/messages/${conversationId}/x`, "//evil.test"])
    assert.equal(routes.safeAppPath(unsafe, "/main"), "/main", unsafe);
});
