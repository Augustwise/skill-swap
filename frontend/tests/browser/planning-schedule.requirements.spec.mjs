import { expect, fillLesson, mutations, test } from "./exchange-requirements-fixture.mjs";

test.describe("FR-09 — lesson planning acceptance gaps", () => {
  test("offline planning requires a place and sends the chosen teacher and explicit UTC timestamps", async ({
    scenario,
  }) => {
    const { world, exchange, actor, andrii } = scenario;
    const active = await exchange({ format: "OFFLINE" });
    const lesson = active.lessons[0];
    const winterYear = new Date(world.nowMs()).getUTCFullYear() + 1;
    const { page } = await actor();
    await page.goto(`/sessions/${lesson.id}`);
    await page.getByRole("button", { name: "Запланувати заняття", exact: true }).click();
    const dialog = page.getByRole("dialog");
    await dialog.getByLabel("Тема заняття").fill("  Робота з шарами  ");
    await dialog.getByLabel("Хто навчає").selectOption(andrii.id);
    await dialog.getByLabel("Початок", { exact: true }).fill(`${winterYear}-01-15T10:00`);
    await dialog.getByLabel("Завершення", { exact: true }).fill(`${winterYear}-01-15T11:00`);
    await dialog.getByLabel("Формат").selectOption("OFFLINE");
    await dialog.getByLabel("Місце зустрічі").fill("   ");
    await dialog.getByRole("button", { name: "Надіслати пропозицію" }).click();
    await expect(dialog.getByText("Вкажи місце зустрічі до 180 символів.")).toBeVisible();
    expect(mutations(world, `sessions/${lesson.id}/proposals`)).toHaveLength(0);
    await dialog.getByLabel("Місце зустрічі").fill("  Бібліотека, кімната 12  ");
    await dialog.getByRole("button", { name: "Надіслати пропозицію" }).click();
    await expect(
      page.getByText("Пропозицію надіслано. Очікуємо погодження партнера."),
    ).toBeVisible();
    const payload = mutations(world, `sessions/${lesson.id}/proposals`)[0].body;
    expect(payload).toMatchObject({
      title: "Робота з шарами",
      teacherId: andrii.id,
      format: "OFFLINE",
      startsAt: `${winterYear}-01-15T08:00:00.000Z`,
      endsAt: `${winterYear}-01-15T09:00:00.000Z`,
      meetingPlace: "Бібліотека, кімната 12",
      meetingUrl: "",
    });
    await expect(page.locator(".session-detail")).toContainText(
      "Викладач: Андрій Шевченко · Учень: Ольга Коваленко",
    );
    await expect(page.locator(".session-detail")).toContainText("Бібліотека, кімната 12");
    await expect(page.getByRole("button", { name: "Погодити час" })).toHaveCount(0);
  });

  for (const [label, end] of [
    ["equal to the start", 120],
    ["before the start", 119],
    ["more than four hours later", 361],
  ]) {
    test(`a lesson ending ${label} is rejected before sending a proposal`, async ({ scenario }) => {
      const { world, exchange, actor } = scenario;
      const active = await exchange();
      const lesson = active.lessons[0];
      const { page } = await actor();
      await page.goto(`/sessions/${lesson.id}`);
      await page.getByRole("button", { name: "Запланувати заняття", exact: true }).click();
      const dialog = page.getByRole("dialog");
      await fillLesson(dialog, world, { start: 120, end });
      await dialog.getByRole("button", { name: "Надіслати пропозицію" }).click();
      await expect(
        dialog.getByText("Завершення має бути через 15–240 хвилин після початку."),
      ).toBeVisible();
      await expect(dialog.getByLabel("Завершення", { exact: true })).toBeFocused();
      expect(mutations(world, `sessions/${lesson.id}/proposals`)).toHaveLength(0);
    });
  }

  test("a server rejection of an expired proposal leaves it unconfirmed and explains the time error", async ({
    scenario,
  }) => {
    const { world, exchange, proposal, actor, andrii } = scenario;
    const active = await exchange();
    const lesson = await proposal(active.lessons[0]);
    const { page } = await actor(andrii);
    await page.goto(`/sessions/${lesson.id}`);
    await expect(page.getByRole("button", { name: "Погодити час" })).toBeVisible();
    world.addFault({
      method: "POST",
      path: `sessions/${lesson.id}/accept`,
      status: 409,
      code: "session_in_past",
    });
    await page.getByRole("button", { name: "Погодити час" }).click();
    await expect(page.getByRole("main").getByRole("alert")).toContainText(
      "Запропонований час уже минув. Обери майбутню дату.",
    );
    await expect(page.locator(".session-detail .session-status")).toHaveText("Очікує погодження");
    expect(world.lessons.get(lesson.id).scheduleConfirmedBy.size).toBe(1);
    expect(world.lessons.get(lesson.id).status).toBe("PROPOSED");
  });

  test("rescheduling requires a new agreement and a stale recipient cannot accidentally accept a replacement", async ({
    scenario,
  }) => {
    const { world, exchange, schedule, proposal, slot, actor, andrii } = scenario;
    const active = await exchange();
    const lesson = await schedule(active.lessons[0]);
    const sender = await actor();
    await sender.page.goto(`/sessions/${lesson.id}`);
    await sender.page.getByRole("button", { name: "Перенести заняття" }).click();
    const dialog = sender.page.getByRole("dialog");
    await fillLesson(dialog, world, { start: 240, end: 300, title: "Новий час заняття" });
    await dialog.getByRole("button", { name: "Надіслати пропозицію" }).click();
    await expect(sender.page.getByText("Ти: погоджено · Партнер: очікує")).toBeVisible();
    await sender.page.goto("/schedule");
    await expect(sender.page.getByText("У розкладі поки немає занять.")).toBeVisible();

    const recipient = await actor(andrii);
    await recipient.page.goto(`/sessions/${lesson.id}`);
    await expect(recipient.page.getByRole("button", { name: "Погодити час" })).toBeVisible();
    await proposal(lesson, { ...slot(360), title: "Остаточний час заняття" });
    const acceptsBefore = mutations(world, `sessions/${lesson.id}/accept`).length;
    await recipient.page.getByRole("button", { name: "Погодити час" }).click();
    await expect(recipient.page.getByRole("main").getByRole("alert")).toContainText(
      "Пропозиція вже змінилася",
    );
    await expect(
      recipient.page.getByRole("heading", { name: "Остаточний час заняття", exact: true }),
    ).toBeVisible();
    expect(mutations(world, `sessions/${lesson.id}/accept`)).toHaveLength(acceptsBefore);
    await recipient.page.getByRole("button", { name: "Погодити час" }).click();
    await expect(
      recipient.page.getByText("Час погоджено. Заняття додано до розкладу."),
    ).toBeVisible();
    await expect(recipient.page.locator(".session-history > li")).toHaveCount(3);
    await sender.page.goto("/schedule");
    await expect(
      sender.page.getByRole("heading", { name: "Остаточний час заняття", exact: true }),
    ).toBeVisible();
  });
});

test.describe("FR-10 — exchanges and schedule acceptance gaps", () => {
  test("scheduled lessons are chronological, the nearest one is shown and the meeting link opens its external destination", async ({
    scenario,
  }) => {
    const { exchange, schedule, slot, actor } = scenario;
    const active = await exchange({ teachSessions: 2, teachDurationMinutes: 30 });
    await schedule(active.lessons[0], { ...slot(300), title: "Пізніше заняття" });
    await schedule(active.lessons[1], { ...slot(60), title: "Найближче заняття з гітари" });
    await schedule(active.lessons[2], { ...slot(180), title: "Середнє заняття" });
    const { page, context } = await actor();
    await page.goto("/exchanges");
    await expect(
      page.getByRole("list", { name: "Активні обміни", exact: true }).locator(":scope > li"),
    ).toHaveCount(1);
    await page.getByRole("link", { name: "Умови та план занять →", exact: true }).click();
    await expect(page.locator(".exchange-next")).toContainText("Найближче заняття з гітари");
    await page.goto("/schedule");
    await expect(page.getByRole("list", { name: "Заняття за часом" }).locator("h2")).toHaveText([
      "Найближче заняття з гітари",
      "Середнє заняття",
      "Пізніше заняття",
    ]);
    await context.route("https://meet.example.test/**", (route) =>
      route.fulfill({
        contentType: "text/html; charset=utf-8",
        body: "<h1>Тестова зовнішня зустріч</h1>",
      }),
    );
    const popupPromise = page.waitForEvent("popup");
    await page.getByRole("link", { name: "Відкрити зустріч ↗", exact: true }).first().click();
    const popup = await popupPromise;
    await expect(popup).toHaveURL("https://meet.example.test/lesson");
    await expect(popup.getByRole("heading", { name: "Тестова зовнішня зустріч" })).toBeVisible();
    await popup.close();
  });

  test("cancelled lessons remain in the plan denominator and do not add to completed progress", async ({
    scenario,
  }) => {
    const { world, exchange, schedule, prepare, olha, andrii, actor } = scenario;
    const active = await exchange({ teachSessions: 2, learnSessions: 2 });
    const cancelled = await schedule(active.lessons[0]);
    await prepare(andrii, `sessions/${cancelled.id}/cancel`, {
      reason: "Зустріч скасовано партнером",
    });
    for (const lesson of [active.lessons[1], active.lessons[2]]) {
      await schedule(lesson, {
        ...scenario.slot(300 + lesson.planOrder * 120),
        title: `Проведене заняття ${lesson.planOrder}`,
      });
      world.advanceMinutes(1000);
      await prepare(olha, `sessions/${lesson.id}/confirm`);
      await prepare(andrii, `sessions/${lesson.id}/confirm`);
    }
    const { page } = await actor();
    await page.goto(`/exchanges/${active.id}`);
    await expect(page.locator(".exchange-progress__percent")).toHaveText("50%");
    await expect(page.getByRole("progressbar")).toHaveAttribute("value", "2");
    await expect(page.getByRole("progressbar")).toHaveAttribute("max", "4");
    const plan = page.getByRole("region", { name: "План занять", exact: true });
    await expect(plan.locator(".session-list > li")).toHaveCount(4);
    await expect(plan.locator(".session-status--cancelled")).toHaveCount(1);
    await expect(plan.locator(".session-status--completed")).toHaveCount(2);
    await expect(
      page.getByRole("button", { name: "Підтвердити завершення", exact: true }),
    ).toHaveCount(0);
  });

  test("a cancelled lesson can be found in schedule history while a stranger sees no meetings", async ({
    scenario,
  }) => {
    const { exchange, schedule, prepare, andrii, stranger, actor } = scenario;
    const active = await exchange();
    const lesson = await schedule(active.lessons[0], { title: "Зустріч для скасування" });
    await prepare(andrii, `sessions/${lesson.id}/cancel`, { reason: "Не можу приєднатися" });
    const { page } = await actor();
    await page.goto("/schedule");
    await expect(page.getByText("У розкладі поки немає занять.")).toBeVisible();
    await page
      .getByRole("navigation", { name: "Статус занять" })
      .getByRole("link", { name: "Скасовані", exact: true })
      .click();
    await expect(page).toHaveURL(/\/schedule\?status=CANCELLED&upcoming=false$/);
    await expect(
      page.getByRole("heading", { name: "Зустріч для скасування", exact: true }),
    ).toBeVisible();
    await page.getByRole("link", { name: "Зустріч для скасування", exact: true }).click();
    await expect(page.getByText("Причина скасування: Не можу приєднатися")).toBeVisible();
    await page.getByRole("link", { name: "До розкладу" }).click();
    await expect(page).toHaveURL(/\/schedule\?status=CANCELLED&upcoming=false$/);
    const outsider = await actor(stranger);
    await outsider.page.goto("/schedule?status=ALL&upcoming=false");
    await expect(outsider.page.getByText("У розкладі поки немає занять.")).toBeVisible();
    await expect(
      outsider.page.getByRole("heading", { name: "Зустріч для скасування", exact: true }),
    ).toHaveCount(0);
  });

  test("a failed schedule refresh keeps its filters and recovers the corresponding list", async ({
    scenario,
  }) => {
    const { world, exchange, proposal, actor } = scenario;
    const active = await exchange();
    await proposal(active.lessons[0], { title: "Пропозиція для розкладу" });
    const { page } = await actor();
    await page.goto("/schedule?status=PROPOSED&upcoming=false");
    await expect(
      page.getByRole("heading", { name: "Пропозиція для розкладу", exact: true }),
    ).toBeVisible();
    world.addFault({ method: "GET", path: "me/schedule", status: 503 });
    await page.getByRole("button", { name: "Оновити розклад" }).click();
    await expect(page.locator(".discovery-alert")).toBeVisible();
    await expect(page.getByText("У розкладі поки немає занять.")).toHaveCount(0);
    await page.getByRole("button", { name: "Спробувати ще раз" }).click();
    await expect(
      page.getByRole("heading", { name: "Пропозиція для розкладу", exact: true }),
    ).toBeVisible();
    await expect(page).toHaveURL(/\/schedule\?status=PROPOSED&upcoming=false$/);
    await expect(
      page
        .getByRole("navigation", { name: "Статус занять" })
        .getByRole("link", { name: "Очікують погодження", exact: true }),
    ).toHaveAttribute("aria-current", "page");
  });
});
