import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { test } from "node:test";
import ts from "typescript";

// Compile the small API modules in memory; no browser or test framework is needed.
function loadSource(filename) {
  const source = ts.transpileModule(fs.readFileSync(filename, "utf8"), {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
  });
  const compiled = { exports: {} };
  new Function("require", "module", "exports", source.outputText)(
    (name) => loadSource(path.resolve(path.dirname(filename), `${name}.ts`)),
    compiled,
    compiled.exports,
  );
  return compiled.exports;
}

const { validateProfile, saveProfile, changeProfileSkill, profileErrors } = loadSource(
  path.resolve("app/components/ProfileApi.ts"),
);
const draft = {
  firstName: "Олена",
  lastName: "Коваль",
  facultyId: "",
  course: "3",
  city: "",
  bio: "",
  formats: ["ONLINE"],
};

test("login returns to the profile and rejects untrusted destinations", () => {
  const { loginDestination } = loadSource(path.resolve("app/components/AuthValidation.ts"));
  assert.equal(loginDestination("?next=/profile"), "/profile");
  assert.equal(loginDestination("?next=/main"), "/main");
  assert.equal(loginDestination("?next=https://example.com"), "/onboarding");
  assert.equal(loginDestination("?next=//example.com"), "/onboarding");
});

test("offline lessons require a city; online lessons do not", () => {
  assert.deepEqual(validateProfile(draft), {});
  assert.ok(validateProfile({ ...draft, formats: ["OFFLINE"] }).city);
  assert.deepEqual(validateProfile({ ...draft, city: "Київ", formats: ["OFFLINE"] }), {});
});

test("CSRF rejection refreshes the session once and keeps the submitted draft", async (t) => {
  let writes = 0;
  t.mock.method(globalThis, "fetch", async (url, options) => {
    if (url.endsWith("auth/me")) return Response.json({ csrfToken: "renewed" });
    writes++;
    if (writes === 1) return Response.json({ error: { code: "csrf_invalid" } }, { status: 403 });
    assert.equal(options.headers.get("X-CSRF-Token"), "renewed");
    assert.equal(JSON.parse(options.body).firstName, draft.firstName);
    return Response.json({ profile: { id: "saved" } });
  });
  assert.equal((await saveProfile(draft, "stale")).id, "saved");
  assert.equal(writes, 2);
});

test("validation and expired-session failures never become successful saves", async (t) => {
  t.mock.method(globalThis, "fetch", async () =>
    Response.json(
      { error: { code: "validation_failed", fields: { city: "English API message" } } },
      { status: 422 },
    ),
  );
  await assert.rejects(saveProfile(draft, "csrf"), (error) => {
    assert.match(profileErrors(error).city, /місто/);
    return true;
  });
  t.mock.method(globalThis, "fetch", async () =>
    Response.json({ error: { code: "unauthorized" } }, { status: 401 }),
  );
  await assert.rejects(saveProfile(draft, "csrf"), (error) => error.status === 401);
});

test("profile limits count Unicode characters and reject invalid input", () => {
  assert.deepEqual(validateProfile({ ...draft, bio: "🎸".repeat(600) }), {});
  for (const [field, value] of Object.entries({
    firstName: " ",
    lastName: "x".repeat(101),
    course: "7",
    bio: "x".repeat(601),
    city: "Київ\nЛьвів",
    formats: [],
  }))
    assert.ok(validateProfile({ ...draft, [field]: value })[field], field);
});

test("writes use session CSRF, normalized values and the correct skill endpoints", async (t) => {
  const calls = [];
  t.mock.method(globalThis, "fetch", async (url, options) => {
    calls.push({ url, ...options });
    return new Response(JSON.stringify({ profile: { id: "saved" } }));
  });
  assert.equal((await saveProfile({ ...draft, firstName: " Олена " }, "csrf")).id, "saved");
  const body = JSON.parse(calls[0].body);
  assert.equal(body.firstName, "Олена");
  assert.equal(body.facultyId, null);
  assert.equal(body.course, 3);
  await changeProfileSkill("teaching", "POST", "guitar", "csrf", "BEGINNER");
  await changeProfileSkill("learning", "PATCH", "design", "csrf", "ADVANCED");
  await changeProfileSkill("learning", "DELETE", "design", "csrf");
  assert.deepEqual(
    calls.map(({ url, method }) => [url, method]),
    [
      ["/api/v1/me/profile", "PATCH"],
      ["/api/v1/me/teaching-skills", "POST"],
      ["/api/v1/me/learning-skills/design", "PATCH"],
      ["/api/v1/me/learning-skills/design", "DELETE"],
    ],
  );
  for (const call of calls) {
    assert.equal(call.headers.get("X-CSRF-Token"), "csrf");
    assert.equal(call.credentials, "same-origin");
    assert.equal(call.cache, "no-store");
  }
});
