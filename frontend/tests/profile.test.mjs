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

const { validateProfile, saveProfile, changeProfileSkill } = loadSource(
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

test("offline lessons require a city; online lessons do not", () => {
  assert.deepEqual(validateProfile(draft), {});
  assert.ok(validateProfile({ ...draft, formats: ["OFFLINE"] }).city);
  assert.deepEqual(validateProfile({ ...draft, city: "Київ", formats: ["OFFLINE"] }), {});
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
