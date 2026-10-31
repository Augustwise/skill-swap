import { expect, test as base } from "@playwright/test";
import { DEMO_PASSWORD, IDS, createMvpWorld } from "./mvp-fixture.mjs";

export { expect, DEMO_PASSWORD, IDS };

export const test = base.extend({
  scenario: async ({ browser }, runScenario) => {
    const world = createMvpWorld();
    const olha = world.seedUser({
      email: "olha@students.example.test",
      firstName: "Ольга",
      lastName: "Коваленко",
      city: "Київ",
      formats: ["ONLINE", "OFFLINE"],
      teachingSkills: [{ skillId: IDS.skills.guitar, level: "ADVANCED" }],
      learningSkills: [{ skillId: IDS.skills.photoshop, level: "BEGINNER" }],
    });
    const andrii = world.seedUser({
      email: "andrii@students.example.test",
      firstName: "Андрій",
      lastName: "Шевченко",
      city: "Київ",
      formats: ["ONLINE", "OFFLINE"],
      teachingSkills: [{ skillId: IDS.skills.photoshop, level: "ADVANCED" }],
      learningSkills: [{ skillId: IDS.skills.guitar, level: "BEGINNER" }],
    });
    const stranger = world.seedUser({
      email: "stranger@students.example.test",
      firstName: "Марко",
      lastName: "Сторонній",
      city: "Київ",
      formats: ["ONLINE"],
      teachingSkills: [{ skillId: IDS.skills.go, level: "ADVANCED" }],
      learningSkills: [{ skillId: IDS.skills.english, level: "BEGINNER" }],
    });
    const actors = new Map();
    async function actor(user = olha) {
      if (actors.has(user.id)) return actors.get(user.id);
      const context = await browser.newContext();
      const value = { user, context };
      actors.set(user.id, value);
      await world.attach(context);
      value.session = await world.signIn(context, user.email);
      value.page = await context.newPage();
      return value;
    }

    async function prepare(user, path, body) {
      const current = await actor(user);
      if (current.page.url() === "about:blank") await current.page.goto("/terms");
      const result = await current.page.evaluate(
        async ({ path, body, csrfToken }) => {
          const response = await fetch(`/api/v1/${path}`, {
            method: "POST",
            headers: { "Content-Type": "application/json", "X-CSRF-Token": csrfToken },
            body: JSON.stringify(body ?? {}),
          });
          return { status: response.status, data: await response.json() };
        },
        { path, body, csrfToken: current.session.csrfToken },
      );
      expect(result.status, `Fixture preparation: POST ${path}`).toBeGreaterThanOrEqual(200);
      expect(result.status, `Fixture preparation: POST ${path}`).toBeLessThan(300);
      return result.data;
    }

    async function request(overrides = {}) {
      const data = await prepare(olha, "exchange-requests", {
        recipientId: andrii.id,
        teachSkillId: IDS.skills.guitar,
        learnSkillId: IDS.skills.photoshop,
        format: "ONLINE",
        teachSessions: 1,
        teachDurationMinutes: 60,
        learnSessions: 1,
        learnDurationMinutes: 60,
        message: "Домовимося про навчання",
        ...overrides,
      });
      return data.request;
    }

    async function exchange(overrides = {}) {
      const pending = await request(overrides);
      const data = await prepare(andrii, `exchange-requests/${pending.id}/accept`);
      return {
        ...data.exchange,
        lessons: [...world.lessons.values()].filter(
          (lesson) => lesson.exchangeId === data.exchange.id,
        ),
      };
    }

    function slot(minutes = 120, duration = 60) {
      const start = Math.floor(world.nowMs() / 60_000) * 60_000 + minutes * 60_000;
      return {
        startsAt: new Date(start).toISOString(),
        endsAt: new Date(start + duration * 60_000).toISOString(),
      };
    }

    async function proposal(lesson, overrides = {}) {
      await prepare(olha, `sessions/${lesson.id}/proposals`, {
        title: "Основи акордів",
        description: "Практична вправа",
        teacherId: lesson.teacherId,
        ...slot(),
        format: "ONLINE",
        meetingUrl: "https://meet.example.test/lesson",
        meetingPlace: "",
        ...overrides,
      });
      return world.lessons.get(lesson.id);
    }

    async function schedule(lesson, overrides = {}) {
      await proposal(lesson, overrides);
      await prepare(andrii, `sessions/${lesson.id}/accept`);
      return world.lessons.get(lesson.id);
    }

    try {
      await runScenario({
        world,
        olha,
        andrii,
        stranger,
        actor,
        prepare,
        request,
        exchange,
        slot,
        proposal,
        schedule,
      });
    } finally {
      for (const value of actors.values()) await value.context.close();
    }
  },
});

export function mutations(world, path) {
  return world.requestLog.filter((entry) => entry.method !== "GET" && entry.path === path);
}

export async function openProposal(page, partnerId) {
  await page.goto(`/students/${partnerId}`);
  await page.getByRole("button", { name: "Запропонувати обмін" }).click();
  const dialog = page.getByRole("dialog");
  await dialog.getByRole("radio", { name: "Онлайн", exact: true }).check();
  return dialog;
}

export async function fillLesson(
  dialog,
  world,
  { start = 120, end = 180, title = "Основи акордів" } = {},
) {
  await dialog.getByLabel("Тема заняття").fill(title);
  await dialog.getByLabel("Початок", { exact: true }).fill(world.futureLocal(start));
  await dialog.getByLabel("Завершення", { exact: true }).fill(world.futureLocal(end));
  await dialog.getByLabel("Посилання на зустріч").fill("https://meet.example.test/lesson");
}
