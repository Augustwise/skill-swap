import { expect, test as base, IDS } from "./exchange-requirements-fixture.mjs";

export { expect, IDS, DEMO_PASSWORD, mutations } from "./exchange-requirements-fixture.mjs";

export const test = base.extend({
  trust: async ({ scenario, browser }, runTrust) => {
    const { world, prepare, olha, andrii } = scenario;
    const admin = world.seedUser({
      email: "admin@students.example.test",
      firstName: "Ірина",
      lastName: "Модераторка",
      role: "ADMIN",
      city: "Київ",
      formats: ["ONLINE"],
      teachingSkills: [{ skillId: IDS.skills.english, level: "ADVANCED" }],
      learningSkills: [{ skillId: IDS.skills.go, level: "BEGINNER" }],
    });
    const contexts = [];
    async function secondSession(user = olha) {
      const context = await browser.newContext();
      contexts.push(context);
      await world.attach(context);
      await world.signIn(context, user.email);
      return { context, page: await context.newPage() };
    }
    async function finishedPlan(overrides = {}) {
      const active = await scenario.exchange(overrides);
      for (const [index, lesson] of active.lessons.entries()) {
        await scenario.schedule(lesson, {
          ...scenario.slot(120 + index * 120),
          title: `Навчальна зустріч ${index + 1}`,
        });
      }
      world.advanceMinutes(240 + active.lessons.length * 120);
      for (const lesson of active.lessons) {
        await prepare(olha, `sessions/${lesson.id}/confirm`);
        await prepare(andrii, `sessions/${lesson.id}/confirm`);
      }
      return active;
    }
    async function completed(overrides = {}) {
      const active = await finishedPlan(overrides);
      await prepare(olha, `exchanges/${active.id}/complete`);
      await prepare(andrii, `exchanges/${active.id}/complete`);
      return active;
    }
    function conversation(requestId) {
      const item = [...world.conversations.values()].find((entry) => entry.requestId === requestId);
      expect(item, "Prepared request must have a conversation").toBeTruthy();
      return item;
    }
    try {
      await runTrust({ ...scenario, admin, secondSession, finishedPlan, completed, conversation });
    } finally {
      for (const context of contexts) await context.close();
    }
  },
});

export async function submitReview(page, exchangeId, rating, comment = "") {
  await page.goto(`/exchanges/${exchangeId}`);
  await page.getByRole("button", { name: "Залишити відгук", exact: true }).click();
  const dialog = page.getByRole("dialog");
  await dialog.getByRole("radio", { name: new RegExp(`^${rating} з 5 —`) }).check();
  await dialog.getByLabel("Текст відгуку", { exact: false }).fill(comment);
  await dialog.getByRole("button", { name: "Надіслати відгук", exact: true }).click();
  await expect(dialog).toHaveCount(0);
  await expect(
    page.getByRole("article", { name: "Твій відгук про партнера", exact: true }),
  ).toBeVisible();
}

export async function blockStudent(page, id) {
  await page.goto(`/students/${id}`);
  await page.getByRole("button", { name: "Заблокувати", exact: true }).click();
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Підтвердити блокування", exact: true })
    .click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect(page).toHaveURL(/\/settings\?tab=blocks$/);
}

export async function openDeletion(page, password) {
  await page.goto("/settings?tab=security");
  await page.getByRole("button", { name: "Видалити акаунт", exact: true }).click();
  const dialog = page.getByRole("dialog");
  await dialog.getByRole("checkbox").check();
  await dialog.getByLabel("Поточний пароль", { exact: true }).fill(password);
  return dialog;
}
