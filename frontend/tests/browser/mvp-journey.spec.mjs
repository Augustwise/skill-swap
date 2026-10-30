import { expect, test } from "@playwright/test";
import { BASE_URL, DEMO_PASSWORD, IDS, MAILPIT_URL, createMvpWorld } from "./mvp-fixture.mjs";

async function registerVerifyAndOnboard(
  page,
  { email, firstName, lastName, teachName, learnName },
) {
  await page.goto("/register");
  await page.locator('input[name="firstName"]').fill(firstName);
  await page.locator('input[name="lastName"]').fill(lastName);
  await page.locator('select[name="university"]').selectOption(IDS.universities.kpi);
  await page.locator('input[name="email"]').fill(email);
  await page.locator('input[name="password"]').fill(DEMO_PASSWORD);
  await page.locator("#register-agreement").check();
  await page.getByRole("button", { name: "Створити акаунт" }).click();
  await expect(page.getByRole("heading", { name: "Акаунт створено" })).toBeVisible();

  await page.goto(`${MAILPIT_URL}/?to=${email}`);
  await page.getByRole("link", { name: `Відкрити посилання для ${email}` }).click();
  await expect(page.getByRole("heading", { name: "Пошту підтверджено" })).toBeVisible();
  await page.getByRole("link", { name: "Перейти до входу" }).click();

  await page.getByLabel("Університетська пошта").fill(email);
  await page.getByLabel("Пароль", { exact: true }).fill(DEMO_PASSWORD);
  await page.getByRole("button", { name: "Увійти", exact: true }).click();
  await expect(page).toHaveURL(`${BASE_URL}/onboarding`);

  await page.locator('input[name="city"]').fill("Київ");
  await page.locator('select[name="facultyId"]').selectOption(IDS.faculties.fiot);
  await page.locator('select[name="course"]').selectOption("3");
  await page
    .locator('textarea[name="bio"]')
    .fill(`${firstName} обмінюється навичками на платформі.`);
  await page.getByRole("button", { name: /Далі — додати навички/ }).click();

  await page.getByRole("button", { name: teachName }).click();
  await page.getByLabel(`Мій рівень для ${teachName}`).selectOption("ADVANCED");
  await page.getByRole("button", { name: /Далі — що хочу вивчити/ }).click();

  await page.getByRole("button", { name: learnName }).click();
  await page.getByRole("button", { name: /Далі — обрати формат/ }).click();

  await page.getByRole("button", { name: "Онлайн" }).click();
  await page.getByRole("button", { name: /Завершити/ }).click();
  await expect(page).toHaveURL(`${BASE_URL}/main`);
}

test("end-to-end MVP journey from registration through completed exchange, mutual reviews, moderation, privacy, and account deletion", async ({
  browser,
}) => {
  test.setTimeout(120_000);

  const world = createMvpWorld({ pollIntervalSeconds: 1 });
  const marko = world.seedUser({
    email: "marko@students.example.test",
    firstName: "Марко",
    lastName: "Сидоренко",
    city: "Київ",
    formats: ["ONLINE"],
    teachingSkills: [{ skillId: IDS.skills.photoshop, level: "INTERMEDIATE" }],
    learningSkills: [{ skillId: IDS.skills.english, level: "BEGINNER" }],
  });
  const admin = world.seedUser({
    email: "admin@students.example.test",
    firstName: "Ірина",
    lastName: "Модераторка",
    role: "ADMIN",
    city: "Київ",
    formats: ["ONLINE"],
    teachingSkills: [{ skillId: IDS.skills.go, level: "ADVANCED" }],
    learningSkills: [{ skillId: IDS.skills.english, level: "INTERMEDIATE" }],
  });

  const olhaCtx = await browser.newContext();
  const andriiCtx = await browser.newContext();
  const viktorCtx = await browser.newContext();
  const markoCtx = await browser.newContext();
  const adminCtx = await browser.newContext();

  for (const ctx of [olhaCtx, andriiCtx, viktorCtx, markoCtx, adminCtx]) {
    await world.attach(ctx);
  }
  await world.signIn(markoCtx, marko.email);
  await world.signIn(adminCtx, admin.email);

  const olhaPage = await olhaCtx.newPage();
  const andriiPage = await andriiCtx.newPage();
  const viktorPage = await viktorCtx.newPage();
  const markoPage = await markoCtx.newPage();
  const adminPage = await adminCtx.newPage();

  // 1. Olha and Andrii register, confirm email via local Mailpit, sign in, and complete onboarding.
  await registerVerifyAndOnboard(olhaPage, {
    email: "olha@students.example.test",
    firstName: "Ольга",
    lastName: "Коваленко",
    teachName: "Гітара",
    learnName: "Photoshop",
  });
  await registerVerifyAndOnboard(andriiPage, {
    email: "andrii@students.example.test",
    firstName: "Андрій",
    lastName: "Шевченко",
    teachName: "Photoshop",
    learnName: "Гітара",
  });

  const andrii = [...world.users.values()].find((u) => u.email === "andrii@students.example.test");

  // 2. Viktor registers without confirming email -> blocked from private routes on /verify-email.
  await viktorPage.goto("/register");
  await viktorPage.locator('input[name="firstName"]').fill("Віктор");
  await viktorPage.locator('input[name="lastName"]').fill("Непідтверджений");
  await viktorPage.locator('select[name="university"]').selectOption(IDS.universities.kpi);
  await viktorPage.locator('input[name="email"]').fill("viktor@students.example.test");
  await viktorPage.locator('input[name="password"]').fill(DEMO_PASSWORD);
  await viktorPage.locator("#register-agreement").check();
  await viktorPage.getByRole("button", { name: "Створити акаунт" }).click();
  await expect(viktorPage.getByRole("heading", { name: "Акаунт створено" })).toBeVisible();

  await viktorPage.goto("/login");
  await viktorPage.getByLabel("Університетська пошта").fill("viktor@students.example.test");
  await viktorPage.getByLabel("Пароль", { exact: true }).fill(DEMO_PASSWORD);
  await viktorPage.getByRole("button", { name: "Увійти", exact: true }).click();
  await expect(viktorPage).toHaveURL(`${BASE_URL}/verify-email`);
  await viktorPage.goto("/search");
  await expect(viktorPage).toHaveURL(`${BASE_URL}/verify-email`);

  // 3. Olha searches Photoshop, checks mutual matches, inspects Marko's one-sided profile, and sends 1x60 request to Andrii.
  const topSearch = olhaPage.getByLabel("Пошук студентів за навичкою або ім’ям");
  await topSearch.fill("Photoshop");
  await topSearch.press("Enter");
  await expect(olhaPage.getByText("Андрій Шевченко")).toBeVisible();
  await expect(olhaPage.getByText("Марко Сидоренко")).toBeVisible();

  await olhaPage.goto(`/students/${marko.id}`);
  await expect(olhaPage.getByRole("heading", { name: "Марко Сидоренко" })).toBeVisible();

  await olhaPage.goto("/matches");
  await expect(olhaPage.getByText("Андрій Шевченко")).toBeVisible();
  await expect(olhaPage.getByText("Марко Сидоренко")).toHaveCount(0);

  await olhaPage.goto(`/students/${andrii.id}`);
  await olhaPage.getByRole("button", { name: "Запропонувати обмін" }).click();
  const proposalDialog = olhaPage.getByRole("dialog");
  await proposalDialog.getByLabel("Ти проводиш: Занять").selectOption("1");
  await proposalDialog
    .getByLabel(/Повідомлення/)
    .fill("Привіт! Обміняймося по одному заняттю 60 хв.");
  await proposalDialog.getByRole("button", { name: "Надіслати запит" }).click();
  await proposalDialog.getByRole("link", { name: "Переглянути запит" }).click();
  await expect(olhaPage).toHaveURL(/\/requests\/[0-9a-f-]+$/);
  const requestId = [...world.requests.keys()][0];

  // 4. Andrii opens notification in bell, chats with Olha, and accepts the request -> active exchange.
  await andriiPage.reload();
  const bell = andriiPage.getByRole("button", { name: "Сповіщення: 1 непрочитаних" });
  await bell.click();
  await andriiPage
    .locator(".notification-menu__panel")
    .getByRole("link", { name: /пропонує обмін навичками/ })
    .click();
  await expect(andriiPage).toHaveURL(`${BASE_URL}/requests/${requestId}`);

  await andriiPage.getByRole("link", { name: "Написати повідомлення" }).click();
  await expect(andriiPage).toHaveURL(/\/messages\/[0-9a-f-]+$/);
  const conversationId = [...world.conversations.keys()][0];

  const andriiComposer = andriiPage.getByLabel("Нове повідомлення, отримувач: Ольга");
  await andriiComposer.fill("Привіт! Домовилися, приймаю запит.");
  await andriiComposer.press("Enter");

  await olhaPage.goto(`/messages/${conversationId}`);
  await expect(
    olhaPage.locator(".chat-message__body").getByText("Привіт! Домовилися, приймаю запит."),
  ).toBeVisible();
  await olhaPage.getByLabel("Нове повідомлення, отримувач: Андрій").fill("Чудово, чекаю на старт!");
  await olhaPage.getByLabel("Нове повідомлення, отримувач: Андрій").press("Enter");

  await andriiPage.goto(`/requests/${requestId}`);
  await andriiPage.getByRole("button", { name: "Прийняти", exact: true }).click();
  await expect(andriiPage).toHaveURL(/\/exchanges\/[0-9a-f-]+\?started=1$/);
  const exchangeId = [...world.exchanges.keys()][0];
  const lessonIds = [...world.lessons.values()]
    .sort((a, b) => a.planOrder - b.planOrder)
    .map((l) => l.id);
  expect(lessonIds).toHaveLength(2);

  // 5. Foreign user Marko cannot open private request, exchange, or chat by direct URL; Olha cannot access /admin.
  await markoPage.goto(`/requests/${requestId}`);
  await expect(markoPage.getByText("Запит не знайдено.")).toBeVisible();
  await markoPage.goto(`/exchanges/${exchangeId}`);
  await expect(markoPage.getByText("Обмін не знайдено.")).toBeVisible();
  await markoPage.goto(`/messages/${conversationId}`);
  await expect(markoPage.getByText("Розмову не знайдено.")).toBeVisible();

  await olhaPage.goto("/admin");
  await expect(
    olhaPage.getByRole("heading", { name: "Доступ лише для адміністраторів" }),
  ).toBeVisible();

  // 6. Schedule and accept both lessons (one in each direction) and verify personal /schedule.
  const slots = [
    [60, 120, "Основи акордів", "https://meet.example.com/guitar"],
    [180, 240, "Шари та маски в Photoshop", "https://meet.example.com/ps"],
  ];
  for (let idx = 0; idx < 2; idx++) {
    const [startMin, endMin, title, link] = slots[idx];
    await olhaPage.goto(`/sessions/${lessonIds[idx]}`);
    await olhaPage.getByRole("button", { name: "Запланувати заняття" }).click();
    const dialog = olhaPage.getByRole("dialog");
    await dialog.getByLabel("Тема заняття").fill(title);
    await dialog.getByLabel("Початок").fill(world.futureLocal(startMin));
    await dialog.getByLabel("Завершення").fill(world.futureLocal(endMin));
    await dialog.getByLabel("Посилання на зустріч").fill(link);
    await dialog.getByRole("button", { name: "Надіслати пропозицію" }).click();
    await expect(olhaPage.getByText(/Пропозицію надіслано/)).toBeVisible();

    await andriiPage.goto(`/sessions/${lessonIds[idx]}`);
    await andriiPage.getByRole("button", { name: "Погодити час" }).click();
    await expect(andriiPage.getByText(/Час погоджено/)).toBeVisible();
  }

  await olhaPage.goto("/schedule");
  await expect(olhaPage.getByText("Основи акордів")).toBeVisible();
  await expect(olhaPage.getByText("Шари та маски в Photoshop")).toBeVisible();
  await markoPage.goto("/schedule");
  await expect(markoPage.getByText("У розкладі поки немає занять.")).toBeVisible();

  // 7. Advance time past both lessons; verify two-sided confirmation for each lesson and final exchange completion.
  world.advanceMinutes(360);

  await olhaPage.goto(`/sessions/${lessonIds[0]}`);
  await olhaPage.getByRole("button", { name: "Заняття відбулося" }).click();
  await expect(olhaPage.locator(".completion-confirmations")).toContainText("Підтверджено ✓");

  await olhaPage.goto(`/exchanges/${exchangeId}`);
  await expect(olhaPage.locator(".exchange-progress__percent")).toHaveText("0%");
  await expect(olhaPage.getByRole("button", { name: "Підтвердити завершення" })).toHaveCount(0);

  await andriiPage.goto(`/sessions/${lessonIds[0]}`);
  await andriiPage.getByRole("button", { name: "Заняття відбулося" }).click();
  await expect(andriiPage.locator(".completion-confirmations")).toContainText("Підтверджено ✓");

  await olhaPage.goto(`/sessions/${lessonIds[1]}`);
  await olhaPage.getByRole("button", { name: "Заняття відбулося" }).click();
  await andriiPage.goto(`/sessions/${lessonIds[1]}`);
  await andriiPage.getByRole("button", { name: "Заняття відбулося" }).click();

  await olhaPage.goto(`/exchanges/${exchangeId}`);
  await expect(olhaPage.locator(".exchange-progress__percent")).toHaveText("100%");
  await olhaPage.getByRole("button", { name: "Підтвердити завершення" }).click();
  await expect(
    olhaPage.getByText("Ти підтвердив. Очікуємо підтвердження партнера.").first(),
  ).toBeVisible();

  await andriiPage.goto(`/exchanges/${exchangeId}`);
  await andriiPage.getByRole("button", { name: "Підтвердити завершення" }).click();
  await expect(andriiPage.getByText("Обмін успішно завершено!").first()).toBeVisible();

  // 8. Both participants leave mutual reviews and see updated aggregate ratings.
  await olhaPage.reload();
  await olhaPage.getByRole("button", { name: "Залишити відгук", exact: true }).click();
  await olhaPage
    .getByRole("dialog")
    .getByRole("radio", { name: "5 з 5 — Чудово", exact: true })
    .check();
  await olhaPage
    .getByRole("dialog")
    .getByLabel("Текст відгуку", { exact: false })
    .fill("Чудове пояснення інструментів Photoshop!");
  await olhaPage.getByRole("dialog").getByRole("button", { name: "Надіслати відгук" }).click();
  await expect(olhaPage.getByRole("article", { name: "Твій відгук про партнера" })).toBeVisible();

  await andriiPage.reload();
  await andriiPage.getByRole("button", { name: "Залишити відгук", exact: true }).click();
  await andriiPage
    .getByRole("dialog")
    .getByRole("radio", { name: "4 з 5 — Дуже добре", exact: true })
    .check();
  await andriiPage
    .getByRole("dialog")
    .getByLabel("Текст відгуку", { exact: false })
    .fill("Гарна структура заняття з гітари.");
  await andriiPage.getByRole("dialog").getByRole("button", { name: "Надіслати відгук" }).click();
  await expect(andriiPage.getByRole("article", { name: "Твій відгук про партнера" })).toBeVisible();

  await olhaPage.goto(`/students/${andrii.id}`);
  await expect(olhaPage.getByRole("region", { name: "Відгуки", exact: true })).toContainText("5,0");
  await olhaPage.goto("/profile?tab=reviews");
  await expect(
    olhaPage.getByRole("region", { name: "Відгуки про мене", exact: true }),
  ).toContainText("4,0");

  // 9. Andrii reports & blocks Marko; Admin resolves report with SUSPEND -> Marko's session is revoked.
  await andriiPage.goto(`/students/${marko.id}`);
  await andriiPage.getByRole("button", { name: "Поскаржитися" }).click();
  await andriiPage
    .getByRole("dialog")
    .getByLabel("Причина скарги")
    .fill("Нав’язливий спам у профілі");
  await andriiPage.getByRole("dialog").getByRole("button", { name: "Надіслати скаргу" }).click();
  await expect(andriiPage.getByRole("dialog")).toHaveCount(0);

  await andriiPage.getByRole("button", { name: "Заблокувати", exact: true }).click();
  await andriiPage
    .getByRole("dialog")
    .getByRole("button", { name: "Підтвердити блокування" })
    .click();
  await expect(andriiPage).toHaveURL(/\/settings\?tab=blocks/);

  const reportId = [...world.reports.keys()][0];
  await adminPage.goto(`/admin/reports/${reportId}`);
  await adminPage.getByRole("button", { name: "Ухвалити рішення" }).click();
  await adminPage
    .getByRole("dialog")
    .getByRole("combobox", { name: "Рішення", exact: true })
    .selectOption("SUSPEND");
  await adminPage
    .getByRole("dialog")
    .getByLabel("Причина рішення")
    .fill("Підтверджено порушення правил");
  await adminPage.getByRole("dialog").getByRole("button", { name: "Зберегти рішення" }).click();
  await expect(adminPage.getByRole("heading", { name: "Рішення за скаргою" })).toBeVisible();

  await markoPage.goto("/main");
  await expect(markoPage.getByRole("heading", { name: "Увійди, щоб продовжити" })).toBeVisible();
  await markoPage.getByRole("link", { name: "Увійти", exact: true }).click();
  await markoPage.getByLabel("Університетська пошта").fill(marko.email);
  await markoPage.getByLabel("Пароль", { exact: true }).fill(DEMO_PASSWORD);
  await markoPage.getByRole("button", { name: "Увійти", exact: true }).click();
  await expect(markoPage.locator(".register-form__error")).toContainText("Цей акаунт призупинено.");

  // 10. Olha hides her profile from discovery while keeping completed exchange; Andrii deletes account and is anonymized.
  await olhaPage.goto("/settings");
  await olhaPage.getByRole("switch", { name: "Показувати мій профіль", exact: false }).uncheck();
  await olhaPage.getByRole("button", { name: "Зберегти налаштування" }).click();
  await expect(olhaPage.getByRole("status")).toContainText("збережено");

  await andriiPage.goto("/search?q=Гітара");
  await expect(andriiPage.getByText("Ольга Коваленко")).toHaveCount(0);

  await andriiPage.goto("/settings?tab=security");
  await andriiPage.getByRole("button", { name: "Видалити акаунт", exact: true }).click();
  await andriiPage.getByRole("dialog").getByRole("checkbox").check();
  await andriiPage.getByRole("dialog").getByLabel("Поточний пароль").fill(DEMO_PASSWORD);
  await andriiPage
    .getByRole("dialog")
    .getByRole("button", { name: "Підтвердити видалення" })
    .click();
  await expect(andriiPage).toHaveURL(/\/login\?deleted=1$/);
  await expect(
    andriiPage.getByText("Акаунт видалено, профіль знеособлено. Усі сесії завершено."),
  ).toBeVisible();

  await olhaPage.goto(`/exchanges/${exchangeId}`);
  await expect(olhaPage.getByText("Видалений користувач").first()).toBeVisible();
  await olhaPage.goto(`/messages/${conversationId}`);
  await expect(
    olhaPage.locator(".chat-message__body").getByText("Повідомлення видалено").first(),
  ).toBeVisible();

  for (const ctx of [olhaCtx, andriiCtx, viktorCtx, markoCtx, adminCtx]) {
    await ctx.close();
  }
});
