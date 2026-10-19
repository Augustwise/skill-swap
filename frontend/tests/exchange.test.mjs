import assert from "node:assert/strict";
import { test } from "node:test";
import { loadSource } from "./load-source.mjs";

const exchange = loadSource("app/components/ExchangeApi.ts");
const format = loadSource("app/components/ExchangeFormat.ts");
const routes = loadSource("app/components/AppRoutes.ts");
const { AuthError } = loadSource("app/components/AuthApi.ts");

const olha = "60000000-0000-0000-0000-000000000001";
const andrii = "60000000-0000-0000-0000-000000000002";
const stranger = "60000000-0000-0000-0000-000000000003";
const requestId = "70000000-0000-0000-0000-000000000001";
const exchangeId = "80000000-0000-0000-0000-000000000001";
const guitar = "30000000-0000-0000-0000-000000000001";
const photoshop = "30000000-0000-0000-0000-000000000002";

const person = (id, firstName) => ({
  id,
  firstName,
  lastName: "Тест",
  university: { id: "u", name: "КПІ" },
  city: "Київ",
});
const terms = (skillId, name, sessions = 2, durationMinutes = 60) => ({
  skill: { skillId, categoryId: "c", name },
  teacherLevel: "ADVANCED",
  learnerLevel: "BEGINNER",
  sessions,
  durationMinutes,
});
const request = (status = "PENDING") => ({
  id: requestId,
  status,
  format: "ONLINE",
  totalSessions: 4,
  message: null,
  requester: person(olha, "Ольга"),
  recipient: person(andrii, "Андрій"),
  requesterTeaches: terms(guitar, "Гітара"),
  recipientTeaches: terms(photoshop, "Photoshop"),
  exchangeId: null,
  createdAt: "2026-10-18T10:00:00Z",
  respondedAt: null,
});
const skill = (skillId, name, level = "BEGINNER") => ({ skillId, categoryId: "c", name, level });
const options = {
  teach: [skill(guitar, "Гітара")],
  learn: [skill(photoshop, "Photoshop", "ADVANCED")],
  formats: ["ONLINE"],
};

function jsonResponse(body, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

test("request filters round-trip through the URL and drop invalid values", () => {
  const filters = exchange.readRequestFilters(
    new URLSearchParams("direction=outgoing&status=DECLINED&page=2"),
  );
  assert.deepEqual(filters, { direction: "outgoing", status: "DECLINED", page: 2 });
  assert.equal(exchange.requestQueryString(filters), "direction=outgoing&status=DECLINED&page=2");
  assert.deepEqual(
    exchange.readRequestFilters(new URLSearchParams("direction=sideways&status=LOST&page=0")),
    exchange.emptyRequestFilters,
  );
  assert.equal(exchange.requestsHref(), "/requests");
  assert.equal(exchange.requestsHref({ status: "PENDING" }), "/requests?status=PENDING");
  assert.equal(exchange.requestHref(requestId), `/requests/${requestId}`);
  assert.equal(exchange.exchangeHref(exchangeId), `/exchanges/${exchangeId}`);
});

test("each participant sees their own side of the request and only their actions", () => {
  const asRequester = exchange.requestPerspective(request(), olha);
  assert.equal(asRequester.role, "requester");
  assert.equal(asRequester.partner.id, andrii);
  assert.equal(asRequester.youTeach.skill.name, "Гітара");
  assert.equal(asRequester.partnerTeaches.skill.name, "Photoshop");
  const asRecipient = exchange.requestPerspective(request(), andrii);
  assert.equal(asRecipient.role, "recipient");
  assert.equal(asRecipient.youTeach.skill.name, "Photoshop");

  assert.deepEqual(exchange.availableActions(request(), andrii), ["accept", "decline"]);
  assert.deepEqual(exchange.availableActions(request(), olha), ["withdraw"]);
  assert.deepEqual(exchange.availableActions(request(), stranger), []);
  for (const status of ["ACCEPTED", "DECLINED", "WITHDRAWN"]) {
    assert.deepEqual(exchange.availableActions(request(status), andrii), [], status);
    assert.deepEqual(exchange.availableActions(request(status), olha), [], status);
  }
});

test("proposal options come from the match and single choices are preselected", () => {
  const fromMatch = exchange.proposalOptions({
    canTeachYou: options.learn,
    wantsToLearn: options.teach,
    commonFormats: ["ONLINE"],
  });
  assert.deepEqual(fromMatch, options);
  const draft = exchange.initialProposal(options);
  assert.equal(draft.teachSkillId, guitar);
  assert.equal(draft.learnSkillId, photoshop);
  assert.equal(draft.format, "ONLINE");
  assert.deepEqual(exchange.validateProposal(draft, options), {});
  const open = exchange.initialProposal({ ...options, formats: ["ONLINE", "OFFLINE"] });
  assert.equal(open.format, "");
  assert.equal(exchange.durationOptions[0], 15);
  assert.equal(exchange.durationOptions.at(-1), 240);
  assert.equal(exchange.sessionOptions.length, 20);
});

test("proposal validation mirrors the server limits and the equal total time", () => {
  const draft = exchange.initialProposal(options);
  const invalid = exchange.validateProposal(
    { ...draft, teachSkillId: "", learnSkillId: photoshop, format: "OFFLINE" },
    options,
  );
  assert.deepEqual(Object.keys(invalid).sort(), ["format", "teachSkillId"]);
  assert.equal(
    exchange.validateProposal({ ...draft, learnSessions: 3 }, options).plan,
    "Сумарний час навчання в обидва боки має бути однаковим.",
  );
  assert.deepEqual(
    exchange.validateProposal({ ...draft, learnSessions: 4, learnDurationMinutes: 30 }, options),
    {},
  );
  assert.match(exchange.validateProposal({ ...draft, teachSessions: 21 }, options).plan, /1 до 20/);
  assert.match(
    exchange.validateProposal(
      { ...draft, teachDurationMinutes: 10, learnDurationMinutes: 10 },
      options,
    ).plan,
    /15 до 240/,
  );
  assert.equal(
    exchange.validateProposal({ ...draft, message: "🎸".repeat(500) }, options).message,
    undefined,
  );
  assert.ok(exchange.validateProposal({ ...draft, message: "🎸".repeat(501) }, options).message);
  assert.deepEqual(
    exchange.proposalTotals({ ...draft, learnSessions: 4, learnDurationMinutes: 30 }),
    {
      sessions: 6,
      teachMinutes: 120,
      learnMinutes: 120,
    },
  );
});

test("server validation and conflicts are explained in Ukrainian", () => {
  const failed = new AuthError(422, "validation_failed", {
    teachSkillId: "The student does not want to learn this skill",
    learnSessions: "Both directions must have the same total time",
    message: "Message must be at most 500 characters",
    recipientId: "Choose a student",
  });
  assert.deepEqual(Object.keys(exchange.proposalErrors(failed)).sort(), [
    "message",
    "plan",
    "teachSkillId",
  ]);
  assert.deepEqual(exchange.proposalErrors(new AuthError(409, "duplicate_request")), {});
  assert.equal(
    exchange.exchangeErrorMessage(new AuthError(409, "duplicate_request")),
    "Запит на цю пару навичок уже очікує відповіді.",
  );
  assert.match(exchange.exchangeErrorMessage(new AuthError(409, "request_outdated")), /відкликати/);
  assert.match(exchange.exchangeErrorMessage(new AuthError(401, "unauthenticated")), /Сесія/);
  assert.match(exchange.exchangeErrorMessage(new TypeError("offline")), /з’єднання/);
  assert.equal(exchange.staleRequest(new AuthError(409, "request_not_pending")), true);
  assert.equal(exchange.staleRequest(new AuthError(409, "duplicate_request")), false);
});

test("a request is sent once with the CSRF token, a trimmed message and both directions", async (t) => {
  const calls = [];
  t.mock.method(globalThis, "fetch", async (url, options) => {
    calls.push({ url, ...options, headers: Object.fromEntries(options.headers) });
    return jsonResponse({ request: request() }, 201);
  });
  const draft = { ...exchange.initialProposal(options), message: "  Привіт!  " };
  const created = await exchange.createRequest(andrii, draft, "token-1");
  assert.equal(created.id, requestId);
  assert.equal(calls.length, 1);
  assert.equal(calls[0].url, "/api/v1/exchange-requests");
  assert.equal(calls[0].method, "POST");
  assert.equal(calls[0].headers["x-csrf-token"], "token-1");
  assert.deepEqual(JSON.parse(calls[0].body), {
    recipientId: andrii,
    teachSkillId: guitar,
    learnSkillId: photoshop,
    format: "ONLINE",
    teachSessions: 2,
    teachDurationMinutes: 60,
    learnSessions: 2,
    learnDurationMinutes: 60,
    message: "Привіт!",
  });
  await exchange.createRequest(andrii, { ...draft, message: "   " }, "token-1");
  assert.equal("message" in JSON.parse(calls[1].body), false);
});

test("a stale CSRF token is refreshed once before a decision is retried", async (t) => {
  const calls = [];
  t.mock.method(globalThis, "fetch", async (url, options) => {
    calls.push({ url, token: new Headers(options.headers).get("X-CSRF-Token") });
    if (url.endsWith("auth/me")) return jsonResponse({ user: {}, csrfToken: "fresh" });
    if (calls.length === 1) {
      return jsonResponse({ error: { code: "csrf_invalid", message: "CSRF" } }, 403);
    }
    return jsonResponse({ request: request("DECLINED") });
  });
  const declined = await exchange.answerRequest(requestId, "decline", "stale");
  assert.equal(declined.status, "DECLINED");
  assert.deepEqual(calls, [
    { url: `/api/v1/exchange-requests/${requestId}/decline`, token: "stale" },
    { url: "/api/v1/auth/me", token: null },
    { url: `/api/v1/exchange-requests/${requestId}/decline`, token: "fresh" },
  ]);
});

test("request and exchange reads call the sprint 5 endpoints", async (t) => {
  const calls = [];
  t.mock.method(globalThis, "fetch", async (url, options) => {
    calls.push({ url, method: options.method });
    if (url.includes("/accept"))
      return jsonResponse({ request: request("ACCEPTED"), exchange: {} });
    if (url.includes("/exchanges/"))
      return jsonResponse({ exchange: { id: exchangeId }, sessions: [] });
    if (url.includes("/exchange-requests/"))
      return jsonResponse({ request: request(), history: [] });
    return jsonResponse({ items: [], page: 1, pageSize: 20, total: 3 });
  });
  await exchange.listRequests({ direction: "outgoing", status: "", page: 1 });
  await exchange.listRequests({ direction: "incoming", status: "ACCEPTED", page: 2 });
  assert.equal(await exchange.pendingRequestCount("incoming"), 3);
  await exchange.requestDetails(requestId);
  await exchange.acceptRequest(requestId, "token");
  await exchange.answerRequest(requestId, "withdraw", "token");
  assert.equal((await exchange.exchangeDetails(exchangeId)).id, exchangeId);
  await exchange.activeExchanges();
  assert.deepEqual(calls, [
    { url: "/api/v1/me/exchange-requests?direction=outgoing", method: undefined },
    {
      url: "/api/v1/me/exchange-requests?direction=incoming&status=ACCEPTED&page=2",
      method: undefined,
    },
    { url: "/api/v1/me/exchange-requests?direction=incoming&status=PENDING", method: undefined },
    { url: `/api/v1/exchange-requests/${requestId}`, method: undefined },
    { url: `/api/v1/exchange-requests/${requestId}/accept`, method: "POST" },
    { url: `/api/v1/exchange-requests/${requestId}/withdraw`, method: "POST" },
    { url: `/api/v1/exchanges/${exchangeId}`, method: undefined },
    { url: "/api/v1/me/exchanges?status=ACTIVE", method: undefined },
  ]);
  await assert.rejects(exchange.requestDetails("../me/profile"), (error) => error.status === 404);
  await assert.rejects(exchange.exchangeDetails("not-a-uuid"), (error) => error.status === 404);
  assert.equal(calls.length, 8);
});

test("statuses, durations and times read naturally in Ukrainian", () => {
  assert.equal(format.requestStatusLabels.WITHDRAWN, "Відкликано");
  assert.equal(format.exchangeStatusLabels.ACTIVE, "Активний");
  assert.equal(format.statusTone("PENDING"), "pending");
  assert.equal(format.statusTone("ACCEPTED"), "accepted");
  assert.equal(format.statusTone("ACTIVE"), "active");
  assert.equal(format.statusTone("DECLINED"), "declined");
  assert.equal(format.statusTone("WITHDRAWN"), "neutral");
  assert.equal(format.formatDuration(45), "45 хв");
  assert.equal(format.formatDuration(60), "1 год");
  assert.equal(format.formatDuration(90), "1 год 30 хв");
  assert.equal(format.termsLabel({ sessions: 2, durationMinutes: 60 }), "2 заняття × 1 год");
  assert.equal(format.termsLabel({ sessions: 5, durationMinutes: 45 }), "5 занять × 45 хв");
  assert.equal(format.totalTime({ sessions: 3, durationMinutes: 50 }), "2 год 30 хв");
  const now = new Date("2026-10-18T12:00:00Z");
  assert.equal(format.formatRelative("2026-10-18T11:59:40Z", now), "щойно");
  assert.equal(format.formatRelative("2026-10-18T11:55:00Z", now), "5 хвилин тому");
  assert.equal(format.formatRelative("2026-10-18T10:00:00Z", now), "2 години тому");
  assert.equal(format.formatRelative("2026-10-17T12:00:00Z", now), "учора");
  assert.match(format.formatRelative("2026-10-01T12:00:00Z", now), /1 жовтня 2026/);
  assert.equal(format.formatRelative("not a date", now), "");
});

test("request and exchange pages are allowed as return paths", () => {
  assert.equal(
    routes.safeAppPath("/requests?direction=outgoing&status=PENDING", "/main"),
    "/requests?direction=outgoing&status=PENDING",
  );
  assert.equal(routes.safeAppPath(`/requests/${requestId}`, "/main"), `/requests/${requestId}`);
  assert.equal(routes.safeAppPath(`/exchanges/${exchangeId}`, "/main"), `/exchanges/${exchangeId}`);
  assert.equal(routes.safeAppPath("/exchanges", "/main"), "/exchanges");
  for (const unsafe of ["/requests/not-a-uuid", `/exchanges/${exchangeId}/x`])
    assert.equal(routes.safeAppPath(unsafe, "/main"), "/main", unsafe);
});
