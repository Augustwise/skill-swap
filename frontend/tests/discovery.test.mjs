import assert from "node:assert/strict";
import { test } from "node:test";
import { loadSource } from "./load-source.mjs";

const discovery = loadSource("app/components/DiscoveryApi.ts");
const format = loadSource("app/components/DiscoveryFormat.ts");
const routes = loadSource("app/components/AppRoutes.ts");
const { loginDestination } = loadSource("app/components/AuthValidation.ts");
const { AuthError } = loadSource("app/components/AuthApi.ts");

const category = "20000000-0000-0000-0000-000000000002";
const student = "60000000-0000-0000-0000-000000000002";

function jsonResponse(body) {
  return new Response(JSON.stringify(body), { headers: { "Content-Type": "application/json" } });
}

test("search filters round-trip through the URL and drop invalid values", () => {
  const filters = discovery.readSearchFilters(
    new URLSearchParams(
      `q=%20Photoshop%20&categoryId=${category.toUpperCase()}&level=ADVANCED&format=ONLINE&mutual=true&page=3`,
    ),
  );
  assert.deepEqual(filters, {
    q: "Photoshop",
    categoryId: category,
    level: "ADVANCED",
    format: "ONLINE",
    mutual: true,
    page: 3,
  });
  assert.equal(
    discovery.searchQueryString(filters),
    `q=Photoshop&categoryId=${category}&level=ADVANCED&format=ONLINE&mutual=true&page=3`,
  );
  assert.deepEqual(
    discovery.readSearchFilters(
      new URLSearchParams("categoryId=x&level=EXPERT&format=VIDEO&mutual=yes&page=501"),
    ),
    discovery.emptyFilters,
  );
  assert.equal(discovery.searchQueryString(discovery.emptyFilters), "");
  assert.equal(discovery.readPage("2.5"), 1);
});

test("search text is trimmed, stripped of control characters and limited to 100 characters", () => {
  assert.equal(discovery.normalizeQuery("  Олена\nКоваль "), "Олена Коваль");
  assert.equal(Array.from(discovery.normalizeQuery("🎸".repeat(120))).length, 100);
  assert.equal(discovery.hasActiveFilters(discovery.emptyFilters), false);
  assert.equal(discovery.hasActiveFilters({ ...discovery.emptyFilters, mutual: true }), true);
});

test("discovery requests call the sprint 4 endpoints with the session cookie", async (t) => {
  const calls = [];
  t.mock.method(globalThis, "fetch", async (url, options) => {
    calls.push({ url, ...options });
    if (url.endsWith("skill-categories")) return jsonResponse({ items: [{ id: category }] });
    if (url.includes("/students/")) return jsonResponse({ student: { id: student } });
    return jsonResponse({ items: [], page: 1, pageSize: 20, total: 0 });
  });
  await discovery.searchStudents({ ...discovery.emptyFilters, q: "Photoshop", mutual: true });
  await discovery.searchStudents(discovery.emptyFilters);
  await discovery.mutualMatches(1);
  await discovery.mutualMatches(2);
  assert.equal((await discovery.studentProfile(student)).id, student);
  assert.equal((await discovery.skillCategories())[0].id, category);
  assert.deepEqual(
    calls.map((call) => call.url),
    [
      "/api/v1/students?q=Photoshop&mutual=true",
      "/api/v1/students",
      "/api/v1/me/matches",
      "/api/v1/me/matches?page=2",
      `/api/v1/students/${student}`,
      "/api/v1/skill-categories",
    ],
  );
  for (const call of calls) {
    assert.equal(call.credentials, "same-origin");
    assert.equal(call.cache, "no-store");
    assert.equal(call.method, undefined);
  }
});

test("API errors map to the states the pages show", () => {
  assert.equal(discovery.discoveryProblem(new AuthError(401, "unauthenticated")), "login");
  assert.equal(discovery.discoveryProblem(new AuthError(403, "email_not_verified")), "unverified");
  assert.equal(discovery.discoveryProblem(new AuthError(404, "student_not_found")), "notFound");
  assert.equal(discovery.discoveryProblem(new AuthError(422, "validation_failed")), "invalid");
  assert.equal(discovery.discoveryProblem(new AuthError(400, "invalid_page")), "invalid");
  assert.equal(
    discovery.discoveryProblem(new AuthError(503, "database_unavailable")),
    "unavailable",
  );
  assert.equal(discovery.discoveryProblem(new TypeError("offline")), "unavailable");
});

test("matching eligibility follows the server rule", () => {
  const profile = {
    teachingSkills: [{ skillId: "a" }],
    learningSkills: [{ skillId: "b" }],
    formats: ["ONLINE"],
    city: "",
  };
  assert.equal(discovery.eligibleForMatching(profile), true);
  assert.equal(discovery.eligibleForMatching({ ...profile, learningSkills: [] }), false);
  assert.equal(discovery.eligibleForMatching({ ...profile, formats: [] }), false);
  assert.equal(discovery.eligibleForMatching({ ...profile, formats: ["OFFLINE"] }), false);
  assert.equal(
    discovery.eligibleForMatching({ ...profile, formats: ["OFFLINE"], city: "Київ" }),
    true,
  );
  assert.equal(discovery.pageCount(0), 1);
  assert.equal(discovery.pageCount(41), 3);
});

test("labels use Ukrainian plural forms, one decimal for ratings and safe initials", () => {
  assert.equal(format.countLabel(1, format.studentForms), "1 студент");
  assert.equal(format.countLabel(3, format.reviewForms), "3 відгуки");
  assert.equal(format.countLabel(11, format.reviewForms), "11 відгуків");
  assert.equal(format.countLabel(21, format.skillForms), "21 навичка");
  assert.equal(format.formatRating(4.666), "4,7");
  assert.equal(format.formatRating(5), "5,0");
  assert.equal(format.initials({ firstName: "ольга", lastName: "гнатюк" }), "ОГ");
  assert.equal(format.initials({ firstName: "", lastName: "" }), "СС");
  assert.equal(format.avatarColor(student), format.avatarColor(student));
  assert.equal(format.formatList(["ONLINE", "OFFLINE"], "Київ"), "Онлайн, Офлайн · Київ");
  assert.equal(
    format.studyDetails({ university: { name: "КПІ" }, faculty: { name: "ФІОТ" }, course: 3 }),
    "КПІ · ФІОТ · 3 курс",
  );
  assert.equal(format.formatDate("not a date"), "");
});

test("match overview explains one-sided interest with catalog IDs and the offline city rule", () => {
  const skill = (skillId, name) => ({ skillId, categoryId: category, name, level: "BEGINNER" });
  const viewer = {
    teachingSkills: [skill("guitar", "Гітара")],
    learningSkills: [skill("photoshop", "Photoshop")],
    formats: ["ONLINE", "OFFLINE"],
    city: " Київ ",
  };
  const candidate = {
    teachingSkills: [skill("photoshop", "Photoshop"), skill("figma", "Figma")],
    learningSkills: [skill("english", "Англійська")],
    formats: ["OFFLINE"],
    city: "київ",
  };
  const overview = discovery.matchOverview(viewer, candidate);
  assert.deepEqual(
    overview.canTeachYou.map((item) => item.name),
    ["Photoshop"],
  );
  assert.deepEqual(overview.wantsToLearn, []);
  assert.deepEqual(overview.commonFormats, ["OFFLINE"]);
  assert.deepEqual(
    discovery.matchOverview(viewer, { ...candidate, city: "Львів" }).commonFormats,
    [],
  );
});

test("return paths stay inside the known app pages", () => {
  assert.equal(
    routes.safeAppPath("/search?q=Photoshop&page=2", "/main"),
    "/search?q=Photoshop&page=2",
  );
  assert.equal(routes.safeAppPath(`/students/${student}`, "/main"), `/students/${student}`);
  for (const unsafe of [
    "https://evil.example/main",
    "//evil.example/main",
    "/\\evil.example",
    "javascript:alert(1)",
    "/students/not-a-uuid",
    "/admin/unknown",
    null,
  ])
    assert.equal(routes.safeAppPath(unsafe, "/main"), "/main", String(unsafe));
  assert.equal(routes.loginHref("/matches?page=2"), "/login?next=%2Fmatches%3Fpage%3D2");
  assert.equal(
    routes.studentHref(student, "/search?q=Go"),
    `/students/${student}?from=%2Fsearch%3Fq%3DGo`,
  );
  assert.equal(loginDestination("?next=%2Fsearch%3Fq%3DGo"), "/search?q=Go");
  assert.equal(loginDestination("?next=https://evil.example"), "/onboarding");
  assert.equal(loginDestination("?next=/profile"), "/profile");
});
