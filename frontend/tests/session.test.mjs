import assert from "node:assert/strict";
import { test } from "node:test";
import { loadSource } from "./load-source.mjs";

const time = loadSource("app/components/SessionTime.ts");
const api = loadSource("app/components/SessionApi.ts");
const exchanges = loadSource("app/components/ExchangeApi.ts");
const routes = loadSource("app/components/AppRoutes.ts");
const schedule = loadSource("app/components/ScheduleApi.ts");
const { AuthError } = loadSource("app/components/AuthApi.ts");
const id = "80000000-0000-0000-0000-000000000001";
const draft = () => ({
  title: " Гітара ",
  description: " Акорди ",
  teacherId: id,
  startsAt: "2099-05-12T12:00",
  endsAt: "2099-05-12T13:00",
  format: "ONLINE",
  meetingUrl: " https://meet.example.com/lesson ",
  meetingPlace: "старе місце",
});

test("decisions refresh the proposal and never submit an already replaced time", async () => {
  const original = globalThis.fetch;
  const calls = [];
  const lesson = { id, activeProposal: { id: "old" }, canRespondToProposal: true };
  globalThis.fetch = async (url, options) => {
    calls.push({ url, options });
    return Response.json({ session: { ...lesson, activeProposal: { id: "new" } } });
  };
  try {
    await assert.rejects(api.decideCurrentSession(lesson, "accept", "csrf"), {
      code: "session_not_proposed",
    });
    assert.equal(calls.length, 1);
    assert.equal(calls[0].options.method, undefined);
    globalThis.fetch = async (url, options) => {
      calls.push({ url, options });
      return Response.json({ session: lesson });
    };
    await api.decideCurrentSession(lesson, "decline", "csrf");
    assert.equal(calls.at(-1).options.method, "POST");
    assert.match(calls.at(-1).url, /\/decline$/);
  } finally {
    globalThis.fetch = original;
  }
});

test("lesson times round trip in the browser zone and serialize with UTC", () => {
  const local = "2099-05-12T12:00";
  assert.equal(time.localDateTime(time.timestamp(local)), local);
  assert.match(time.timestamp(local), /Z$/);
  for (const value of ["", "invalid", "2099-02-30T12:00", "2099-05-12T25:00"])
    assert.equal(time.timestamp(value), null);
  assert.equal(time.localDateTime("invalid"), "");
});

test("lesson validation enforces future dates, order and duration boundaries", () => {
  const data = draft();
  assert.deepEqual(time.sessionDraftErrors(data, [id]), {});
  for (const [end, valid] of [
    ["12:14", false],
    ["12:15", true],
    ["16:00", true],
    ["16:01", false],
    ["11:59", false],
  ]) {
    const errors = time.sessionDraftErrors({ ...data, endsAt: `2099-05-12T${end}` }, [id]);
    assert.equal(!errors.endsAt, valid, end);
  }
  assert.ok(
    time.sessionDraftErrors(data, [id], Date.parse(time.timestamp(data.startsAt))).startsAt,
  );
  assert.ok(time.sessionDraftErrors(data, []).teacherId);
});

test("lesson payload trims fields and sends only the selected meeting format", () => {
  const payload = time.sessionPayload(draft());
  assert.equal(payload.title, "Гітара");
  assert.equal(payload.meetingPlace, "");
  assert.equal(payload.meetingUrl, "https://meet.example.com/lesson");
  const offline = { ...draft(), format: "OFFLINE", meetingPlace: " Бібліотека " };
  assert.deepEqual(time.sessionDraftErrors(offline, [id]), {});
  assert.equal(time.sessionPayload(offline).meetingUrl, "");
  assert.ok(time.sessionDraftErrors({ ...offline, meetingPlace: " " }, [id]).meetingPlace);
  assert.ok(time.sessionDraftErrors({ ...draft(), title: "😀".repeat(256) }, [id]).title);
  assert.ok(
    time.sessionDraftErrors({ ...draft(), description: "а".repeat(1001) }, [id]).description,
  );
});

test("meeting links reject executable schemes, credentials and malformed URLs", () => {
  for (const value of [
    null,
    "",
    "javascript:alert(1)",
    "data:text/html,x",
    "//example.com",
    "https://a b.com",
    "https://user:pass@example.com",
  ])
    assert.equal(time.meetingLink(value), null);
  assert.equal(
    time.meetingLink("https://meet.example.com/lesson"),
    "https://meet.example.com/lesson",
  );
});

test("lesson API guards identifiers and sends decisions with CSRF", async () => {
  await assert.rejects(api.sessionDetails("nope"), { status: 404 });
  await assert.rejects(api.exchangeSessions("nope"), { status: 404 });
  const original = globalThis.fetch;
  const calls = [];
  globalThis.fetch = async (url, options) => {
    calls.push({ url, options });
    return Response.json({ session: { id } });
  };
  try {
    await api.proposeSession(id, time.sessionPayload(draft()), "csrf");
    await api.decideSession(id, "accept", "csrf");
    await api.decideSession(id, "decline", "csrf");
    await api.cancelSession(id, " Змінилися плани ", "csrf");
    assert.deepEqual(
      calls.map((call) => call.url),
      [
        `/api/v1/sessions/${id}/proposals`,
        `/api/v1/sessions/${id}/accept`,
        `/api/v1/sessions/${id}/decline`,
        `/api/v1/sessions/${id}/cancel`,
      ],
    );
    assert.ok(
      calls.every(
        ({ options }) =>
          options.method === "POST" && options.headers.get("X-CSRF-Token") === "csrf",
      ),
    );
    assert.equal(JSON.parse(calls[3].options.body).reason, "Змінилися плани");
  } finally {
    globalThis.fetch = original;
  }
  assert.match(api.sessionErrorMessage(new AuthError(409, "session_conflict")), /інший час/);
});

test("exchange pages paginate and lesson reads use the existing private API", async () => {
  const original = globalThis.fetch;
  const urls = [];
  globalThis.fetch = async (url) => {
    urls.push(url);
    return Response.json({ items: [], session: { id } });
  };
  try {
    await exchanges.listActiveExchanges(2);
    await api.exchangeSessions(id);
    await api.sessionDetails(id);
    assert.deepEqual(urls, [
      "/api/v1/me/exchanges?status=ACTIVE&page=2",
      `/api/v1/exchanges/${id}/sessions`,
      `/api/v1/sessions/${id}`,
    ]);
  } finally {
    globalThis.fetch = original;
  }
  for (const path of ["/exchanges?page=2", `/sessions/${id}`])
    assert.equal(routes.safeAppPath(path, "/main"), path);
  assert.equal(routes.safeAppPath("/sessions/nope", "/main"), "/main");
});

test("schedule filters preserve status, dates and pagination in shareable URLs", () => {
  for (const status of schedule.scheduleStatuses) {
    const filters = { status, upcoming: false, page: 3 };
    const href = schedule.scheduleHref(filters);
    assert.deepEqual(
      schedule.readScheduleFilters(new URL(href, "http://localhost").searchParams),
      filters,
    );
    assert.equal(routes.safeAppPath(href, "/main"), href);
  }
  assert.deepEqual(schedule.readScheduleFilters(new URLSearchParams("status=DRAFT&page=-1")), {
    status: "SCHEDULED",
    upcoming: true,
    page: 1,
  });
  assert.equal(
    schedule.readScheduleFilters(new URLSearchParams("status=COMPLETED")).upcoming,
    false,
  );
});

test("schedule uses server pagination and filters without client-side exclusions", async () => {
  const original = globalThis.fetch;
  let request;
  globalThis.fetch = async (url, options) => {
    request = { url, options };
    return Response.json({
      items: [],
      total: 45,
      page: 3,
      pageSize: 20,
      timezone: "Europe/Kyiv",
      pendingProposalsCount: 2,
    });
  };
  try {
    const result = await schedule.listSchedule({ status: "PROPOSED", upcoming: false, page: 3 });
    assert.equal(request.url, "/api/v1/me/schedule?status=PROPOSED&upcoming=false&page=3");
    assert.equal(request.options.credentials, "same-origin");
    assert.equal(result.total, 45);
    assert.equal(result.pendingProposalsCount, 2);
    assert.equal(result.timezone, "Europe/Kyiv");
  } finally {
    globalThis.fetch = original;
  }
});
