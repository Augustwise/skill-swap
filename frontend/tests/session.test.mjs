import assert from "node:assert/strict";
import { test } from "node:test";
import { loadSource } from "./load-source.mjs";

const time = loadSource("app/components/SessionTime.ts");
const api = loadSource("app/components/SessionApi.ts");
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
