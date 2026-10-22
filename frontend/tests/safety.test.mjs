import assert from "node:assert/strict";
import { test } from "node:test";
import { loadSource } from "./load-source.mjs";

const safety = loadSource("app/components/SafetyApi.ts");
const admin = loadSource("app/components/AdminApi.ts");
const { AuthError } = loadSource("app/components/AuthApi.ts");
const { safeAppPath, loginHref } = loadSource("app/components/AppRoutes.ts");
const id = "80000000-0000-0000-0000-000000000010";
const token = "csrf-test";
const draft = {
  categoryId: id,
  name: " Вебдизайн ",
  slug: " web-design ",
  description: " Опис ",
  isActive: false,
};

test("safety reasons count trimmed Unicode, reject controls and preserve multiline plain text", () => {
  for (const value of ["🙂".repeat(1000), "  <script>bad()</script>\nДруга\tлінія  "])
    assert.equal(safety.textError(value, 1000), "");
  for (const value of [" \n ", "🙂".repeat(1001), "bad\0text", "bad\u0085text"])
    assert.ok(safety.textError(value, 1000));
  assert.equal(safety.textError("", 500, false), "");
});

test("password changes preserve whitespace and enforce characters, UTF-8 bytes and confirmation", () => {
  assert.match(
    safety.passwordFields("same password", "same password", "same password").newPassword,
    /відрізнятися/,
  );
  assert.deepEqual(safety.passwordFields(" old ", "🙂".repeat(18), "🙂".repeat(18)), {});
  assert.ok(safety.passwordFields("", "12345678901", "different").currentPassword);
  assert.ok(safety.passwordFields("old", "🙂".repeat(19), "🙂".repeat(19)).newPassword);
  assert.ok(safety.passwordFields("old", "123456789012", "different").confirmation);
});

test("settings and moderation URLs normalize pages and retain filters", () => {
  assert.equal(safety.settingsTab("__proto__"), "privacy");
  assert.equal(admin.adminTab("__proto__"), "reports");
  assert.equal(safety.settingsHref("blocks", 2), "/settings?tab=blocks&page=2");
  assert.equal(safety.settingsHref("reports", 501), "/settings?tab=reports");
  assert.equal(admin.adminHref("reports", { status: "bogus", page: 2 }), "/admin?status=&page=2");
  assert.equal(
    admin.adminHref("reports", { status: "OPEN", page: 2 }),
    "/admin?status=OPEN&page=2",
  );
  assert.equal(
    new URL(
      admin.adminHref("skills", { q: " C++ & React ", page: 3 }),
      "https://local",
    ).searchParams.get("q"),
    "C++ & React",
  );
  assert.equal(
    safeAppPath(`/admin/reports/${id}?from=%2Fadmin`, "/main"),
    `/admin/reports/${id}?from=%2Fadmin`,
  );
  assert.equal(safeAppPath("/admin/reports/not-an-id", "/main"), "/main");
  assert.equal(safeAppPath("//evil.test/admin", "/main"), "/main");
  assert.equal(
    loginHref("/settings?tab=reports&page=2"),
    "/login?next=%2Fsettings%3Ftab%3Dreports%26page%3D2",
  );
});

test("catalog validation mirrors category, line, slug and Unicode limits", () => {
  assert.deepEqual(admin.skillFields(draft), {});
  for (const [key, value] of [
    ["categoryId", "-".repeat(36)],
    ["name", ""],
    ["name", "bad\nname"],
    ["name", "🙂".repeat(151)],
    ["slug", "bad--slug"],
    ["slug", "Веб"],
    ["slug", "x".repeat(151)],
    ["description", "🙂".repeat(501)],
  ]) {
    assert.ok(admin.skillFields({ ...draft, [key]: value })[key], key);
  }
  assert.deepEqual(
    admin.skillFields({ ...draft, name: "🙂".repeat(150), description: "🙂".repeat(500) }),
    {},
  );
});

test("safety reads use private endpoints with server pagination", async () => {
  const original = globalThis.fetch;
  const calls = [];
  globalThis.fetch = async (url, options) => {
    calls.push({ url, options });
    return Response.json({ privacy: { isDiscoverable: true }, items: [] });
  };
  try {
    await safety.privacySettings();
    await safety.blockedStudents(2);
    await safety.myReports(-1);
    await admin.adminReports("OPEN", 3);
    await admin.adminReport(id);
    await admin.catalogSkills("C++ & React", 2);
    await admin.moderationLog(2, id);
    assert.deepEqual(
      calls.map((call) => call.url),
      [
        "/api/v1/me/privacy",
        "/api/v1/me/blocks?page=2",
        "/api/v1/me/reports?page=1",
        "/api/v1/admin/reports?status=OPEN&page=3",
        `/api/v1/admin/reports/${id}`,
        "/api/v1/admin/skills?q=C%2B%2B+%26+React&page=2",
        `/api/v1/admin/moderation-actions?page=2&userId=${id}`,
      ],
    );
    for (const { options } of calls) {
      assert.equal(options.credentials, "same-origin");
      assert.equal(options.cache, "no-store");
    }
  } finally {
    globalThis.fetch = original;
  }
});

test("safety mutations send exact bodies, HTTP methods and CSRF without exposing passwords in URLs", async () => {
  const original = globalThis.fetch;
  const calls = [];
  globalThis.fetch = async (url, options) => {
    calls.push({ url, options });
    return options.method === "DELETE"
      ? new Response(null, { status: 204 })
      : Response.json({ privacy: {}, skill: {} });
  };
  try {
    await safety.updatePrivacy(
      { isDiscoverable: false, acceptsRequests: true, requestScope: "ANYONE" },
      token,
    );
    await safety.changePassword(" old ", " new password ", token);
    await safety.deleteAccount(" password ", token);
    await safety.blockStudent(id, token);
    await safety.unblockStudent(id, token);
    await safety.reportStudent(id, "  Причина\nскарги ", token);
    await admin.resolveReport(id, "SUSPEND", " причина ", token);
    await admin.moderateAccount(id, "restore", " причина ", token);
    await admin.saveSkill(null, draft, token);
    await admin.saveSkill(id, draft, token);
    assert.deepEqual(
      calls.map((call) => call.options.method),
      ["PATCH", "POST", "DELETE", "POST", "DELETE", "POST", "POST", "POST", "POST", "PATCH"],
    );
    assert.deepEqual(JSON.parse(calls[1].options.body), {
      currentPassword: " old ",
      newPassword: " new password ",
    });
    assert.deepEqual(JSON.parse(calls[2].options.body), { password: " password " });
    assert.deepEqual(JSON.parse(calls[5].options.body), {
      reportedUserId: id,
      reason: "Причина\nскарги",
    });
    assert.deepEqual(JSON.parse(calls[6].options.body), { decision: "SUSPEND", reason: "причина" });
    assert.deepEqual(JSON.parse(calls[9].options.body), {
      ...draft,
      name: "Вебдизайн",
      slug: "web-design",
      description: "Опис",
    });
    for (const { url, options } of calls) {
      assert.equal(options.headers.get("X-CSRF-Token"), token);
      assert.ok(!url.includes("password") || url.endsWith("change-password"));
    }
  } finally {
    globalThis.fetch = original;
  }
});

test("invalid identifiers and invalid forms never reach the server", async () => {
  const original = globalThis.fetch;
  globalThis.fetch = () => {
    throw new Error("unexpected fetch");
  };
  try {
    for (const action of [
      () => safety.blockStudent("bad", token),
      () => safety.unblockStudent("../me", token),
      () => safety.reportStudent(id, " ", token),
      () => safety.deleteAccount("", token),
      () => safety.changePassword("old", "short", token),
      () => admin.adminReport("bad"),
      () => admin.resolveReport(id, "UNKNOWN", "reason", token),
      () => admin.moderateAccount("bad", "suspend", "reason", token),
      () => admin.saveSkill(null, { ...draft, name: "" }, token),
      () => safety.updatePrivacy({ requestScope: "ALL" }, token),
    ]) {
      await assert.rejects(action, (error) => error instanceof AuthError);
    }
    assert.throws(() => admin.moderationLog(1, "bad"), AuthError);
  } finally {
    globalThis.fetch = original;
  }
});

test("a stale CSRF token retries DELETE once with the refreshed token", async () => {
  const original = globalThis.fetch;
  const calls = [];
  globalThis.fetch = async (url, options) => {
    calls.push({ url, options });
    if (calls.length === 1)
      return Response.json({ error: { code: "csrf_invalid" } }, { status: 403 });
    if (url.endsWith("auth/me")) return Response.json({ csrfToken: "fresh" });
    return new Response(null, { status: 204 });
  };
  try {
    await safety.unblockStudent(id, token);
    assert.equal(calls.length, 3);
    assert.equal(calls[2].options.method, "DELETE");
    assert.equal(calls[2].options.headers.get("X-CSRF-Token"), "fresh");
  } finally {
    globalThis.fetch = original;
  }
});

test("moderation errors and password fields are translated without leaking server text", () => {
  for (const code of [
    "admin_required",
    "student_not_found",
    "user_not_found",
    "report_not_found",
    "skill_not_found",
    "report_already_open",
    "report_already_resolved",
    "self_moderation",
    "skill_already_exists",
    "login_locked",
  ]) {
    assert.match(safety.safetyError(new AuthError(409, code)), /[А-Яа-яІіЇїЄє]/);
  }
  assert.deepEqual(
    safety.safetyFields(
      new AuthError(422, "validation_failed", {
        currentPassword: "secret English",
        reason: "English",
      }),
    ),
    {
      currentPassword: "Поточний пароль неправильний.",
      reason: "Введи причину від 1 до 1000 символів без керівних символів.",
    },
  );
  assert.match(safety.safetyError(new AuthError(429, "login_locked", {}, 120)), /2 хв/);
});
