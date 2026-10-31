import { expect, test } from "@playwright/test";
import { IDS, createMvpWorld } from "./mvp-fixture.mjs";

test("chat preserves request-to-exchange history, validates limits, linkifies safely, paginates, and recovers from outage", async ({
  browser,
}) => {
  const world = createMvpWorld({ pollIntervalSeconds: 1 });
  const olha = world.seedUser({
    email: "olha@students.example.test",
    firstName: "Ольга",
    lastName: "Коваленко",
    city: "Київ",
    formats: ["ONLINE"],
    teachingSkills: [{ skillId: IDS.skills.guitar, level: "ADVANCED" }],
    learningSkills: [{ skillId: IDS.skills.photoshop, level: "BEGINNER" }],
  });
  const andrii = world.seedUser({
    email: "andrii@students.example.test",
    firstName: "Андрій",
    lastName: "Шевченко",
    city: "Київ",
    formats: ["ONLINE"],
    teachingSkills: [{ skillId: IDS.skills.photoshop, level: "ADVANCED" }],
    learningSkills: [{ skillId: IDS.skills.guitar, level: "BEGINNER" }],
  });
  const stranger = world.seedUser({
    email: "stranger@students.example.test",
    firstName: "Ігор",
    lastName: "Сторонній",
    city: "Київ",
    formats: ["ONLINE"],
    teachingSkills: [{ skillId: IDS.skills.go, level: "INTERMEDIATE" }],
    learningSkills: [{ skillId: IDS.skills.english, level: "BEGINNER" }],
  });

  const olhaCtx = await browser.newContext();
  const andriiCtx = await browser.newContext();
  const strangerCtx = await browser.newContext();
  await world.attach(olhaCtx);
  await world.attach(andriiCtx);
  await world.attach(strangerCtx);
  await world.signIn(olhaCtx, olha.email);
  await world.signIn(andriiCtx, andrii.email);
  await world.signIn(strangerCtx, stranger.email);

  const olhaPage = await olhaCtx.newPage();
  const andriiPage = await andriiCtx.newPage();
  const strangerPage = await strangerCtx.newPage();

  // Olha creates an exchange request and opens its chat.
  await olhaPage.goto(`/students/${andrii.id}`);
  await olhaPage.getByRole("button", { name: "Запропонувати обмін" }).click();
  await olhaPage.getByRole("dialog").getByRole("button", { name: "Надіслати запит" }).click();
  await olhaPage.getByRole("dialog").getByRole("link", { name: "Переглянути запит" }).click();
  await expect(olhaPage).toHaveURL(/\/requests\/[0-9a-f-]+$/);
  const requestId = [...world.requests.keys()][0];

  await olhaPage.getByRole("link", { name: "Написати повідомлення" }).click();
  await expect(olhaPage).toHaveURL(/\/messages\/[0-9a-f-]+$/);
  const conversationId = [...world.conversations.keys()][0];

  // Stranger cannot access the conversation.
  await strangerPage.goto(`/messages/${conversationId}`);
  await expect(strangerPage.getByText("Розмову не знайдено.")).toBeVisible();

  const composer = olhaPage.getByLabel("Нове повідомлення, отримувач: Андрій");
  const sendBtn = olhaPage.getByRole("button", { name: "Надіслати" });

  // Whitespace-only message keeps Send disabled; > 2000 Unicode chars shows counter and disables Send.
  await composer.fill("   \n  ");
  await expect(sendBtn).toBeDisabled();

  await composer.fill("Ї".repeat(2005));
  await expect(olhaPage.getByText("2005 / 2000")).toBeVisible();
  await expect(olhaPage.getByText("Повідомлення — до 2000 символів.")).toBeVisible();
  await expect(sendBtn).toBeDisabled();

  // Send greeting first, then Shift+Enter multiline message with links.
  await composer.fill("Привіт щодо обміну!");
  await composer.press("Enter");
  await expect(
    olhaPage.locator(".chat-message__body").getByText("Привіт щодо обміну!"),
  ).toBeVisible();

  await composer.fill("Матеріали на https://example.com/notes");
  await composer.press("Shift+Enter");
  await composer.type("та www.figma.com/file");
  await composer.press("Enter");

  const sentLink = olhaPage.locator('.chat-message__body a[href="https://example.com/notes"]');
  await expect(sentLink).toHaveAttribute("target", "_blank");
  await expect(sentLink).toHaveAttribute("rel", "noopener noreferrer nofollow");
  await expect(
    olhaPage.locator('.chat-message__body a[href="https://www.figma.com/file"]'),
  ).toBeVisible();
  await expect(olhaPage.locator('.chat-message__status[title="Надіслано"]').last()).toBeVisible();

  // Andrii accepts the request -> conversation moves to the active exchange with history intact.
  await andriiPage.goto(`/requests/${requestId}`);
  await andriiPage.getByRole("button", { name: "Прийняти", exact: true }).click();
  await expect(andriiPage).toHaveURL(/\/exchanges\/[0-9a-f-]+\?started=1$/);

  // Andrii opens the chat, sees Olha's message, and marks it read -> Olha's status updates to Прочитано.
  await andriiPage.goto(`/messages/${conversationId}`);
  await expect(andriiPage.locator(".chat-message__body").getByText(/Матеріали на/)).toBeVisible();
  await expect(andriiPage.getByRole("link", { name: "Запланувати заняття" })).toBeVisible();
  await expect(olhaPage.locator('.chat-message__status[title="Прочитано"]')).toBeVisible({
    timeout: 6000,
  });

  // Simulate transient server outage on Olha's polling, send message from Andrii, then verify auto-recovery.
  world.addFault({
    method: "GET",
    path: `conversations/${conversationId}/messages`,
    times: 2,
    status: 503,
    code: "unavailable",
  });
  await expect(olhaPage.locator(".chat-connection")).toContainText("Немає з’єднання із сервером", {
    timeout: 6000,
  });

  const andriiComposer = andriiPage.getByLabel("Нове повідомлення, отримувач: Ольга");
  await andriiComposer.fill("Бачу твої матеріали, дякую!");
  await andriiComposer.press("Enter");

  await expect(
    olhaPage.locator(".chat-message__body").getByText("Бачу твої матеріали, дякую!"),
  ).toBeVisible({ timeout: 8000 });
  await expect(olhaPage.locator(".chat-connection")).toHaveCount(0);

  // Seed 28 historical messages in chronological order to verify pagination and reload persistence.
  const list = world.messages.get(conversationId);
  const archived = Array.from({ length: 28 }, (_, index) => {
    const i = index + 1;
    return {
      id: `46990000-0000-4000-8000-${String(i).padStart(12, "0")}`,
      conversationId,
      authorId: andrii.id,
      type: "TEXT",
      body: `Архівне повідомлення №${i}`,
      createdAt: new Date(Date.now() - (40 - i) * 60_000).toISOString(),
      editedAt: null,
      readAt: new Date().toISOString(),
    };
  });
  list.unshift(...archived);

  await olhaPage.reload();
  await expect(
    olhaPage.locator(".chat-message__body").getByText("Бачу твої матеріали, дякую!"),
  ).toBeVisible();
  await expect(
    olhaPage.locator(".chat-message__body").getByText("Архівне повідомлення №1", { exact: true }),
  ).toHaveCount(0);
  const earlierBtn = olhaPage.getByRole("button", { name: "Показати попередні повідомлення" });
  await expect(earlierBtn).toBeVisible();
  await earlierBtn.click();
  await expect(
    olhaPage.locator(".chat-message__body").getByText("Архівне повідомлення №1", { exact: true }),
  ).toBeVisible();

  await olhaCtx.close();
  await andriiCtx.close();
  await strangerCtx.close();
});

test("notifications target only the intended recipient, avoid duplicates, and update unread counters", async ({
  browser,
}) => {
  const world = createMvpWorld();
  const olha = world.seedUser({
    email: "olha@students.example.test",
    firstName: "Ольга",
    lastName: "Коваленко",
    city: "Київ",
    formats: ["ONLINE"],
    teachingSkills: [{ skillId: IDS.skills.guitar, level: "ADVANCED" }],
    learningSkills: [{ skillId: IDS.skills.photoshop, level: "BEGINNER" }],
  });
  const andrii = world.seedUser({
    email: "andrii@students.example.test",
    firstName: "Андрій",
    lastName: "Шевченко",
    city: "Київ",
    formats: ["ONLINE"],
    teachingSkills: [{ skillId: IDS.skills.photoshop, level: "ADVANCED" }],
    learningSkills: [{ skillId: IDS.skills.guitar, level: "BEGINNER" }],
  });
  const bystander = world.seedUser({
    email: "bystander@students.example.test",
    firstName: "Марія",
    lastName: "Осторонь",
    city: "Київ",
    formats: ["ONLINE"],
    teachingSkills: [{ skillId: IDS.skills.go, level: "INTERMEDIATE" }],
    learningSkills: [{ skillId: IDS.skills.english, level: "BEGINNER" }],
  });

  const olhaCtx = await browser.newContext();
  const andriiCtx = await browser.newContext();
  const bystanderCtx = await browser.newContext();
  await world.attach(olhaCtx);
  await world.attach(andriiCtx);
  await world.attach(bystanderCtx);
  await world.signIn(olhaCtx, olha.email);
  await world.signIn(andriiCtx, andrii.email);
  await world.signIn(bystanderCtx, bystander.email);

  const olhaPage = await olhaCtx.newPage();
  const andriiPage = await andriiCtx.newPage();
  const bystanderPage = await bystanderCtx.newPage();

  // Olha sends exchange request -> Andrii receives EXCHANGE_REQUEST notification; bystander receives 0.
  await olhaPage.goto(`/students/${andrii.id}`);
  await olhaPage.getByRole("button", { name: "Запропонувати обмін" }).click();
  await olhaPage.getByRole("dialog").getByRole("button", { name: "Надіслати запит" }).click();
  await olhaPage.getByRole("dialog").getByRole("button", { name: "Закрити", exact: true }).click();

  await bystanderPage.goto("/notifications");
  await expect(bystanderPage.getByText("Сповіщень поки немає.")).toBeVisible();

  await andriiPage.goto("/main");
  const bell = andriiPage.getByRole("button", { name: "Сповіщення: 1 непрочитаних" });
  await expect(bell).toBeVisible();
  await bell.click();
  await expect(
    andriiPage.locator(".notification-menu__panel").getByText(/пропонує обмін навичками/),
  ).toBeVisible();

  // Escape closes the notification panel and restores focus to the bell button.
  await andriiPage.keyboard.press("Escape");
  await expect(andriiPage.locator(".notification-menu__panel")).toBeHidden();
  await expect(bell).toBeFocused();

  // Opening the notification navigates to the request and marks it read.
  await bell.click();
  await andriiPage
    .locator(".notification-menu__panel")
    .getByRole("link", { name: /пропонує обмін навичками/ })
    .click();
  await expect(andriiPage).toHaveURL(/\/requests\/[0-9a-f-]+$/);

  // Andrii sends a chat message and accepts the request -> Olha receives CHAT_MESSAGE + REQUEST_RESPONSE.
  await andriiPage.getByRole("link", { name: "Написати повідомлення" }).click();
  const composer = andriiPage.getByLabel("Нове повідомлення, отримувач: Ольга");
  await composer.fill("Привіт, приймаю обмін!");
  await composer.press("Enter");
  await andriiPage.getByRole("link", { name: "Запит на обмін" }).click();
  await andriiPage.getByRole("button", { name: "Прийняти", exact: true }).click();
  await expect(andriiPage).toHaveURL(/\/exchanges\/[0-9a-f-]+\?started=1$/);

  await olhaPage.goto("/notifications?filter=unread");
  await expect(olhaPage.getByText("2 сповіщення · непрочитаних: 2")).toBeVisible();
  await expect(olhaPage.getByText(/Нове повідомлення від Андрій Шевченко/)).toBeVisible();
  await expect(olhaPage.getByText(/прийняв\(-ла\) запит на обмін/)).toBeVisible();

  await olhaPage.getByRole("button", { name: "Позначити всі прочитаними" }).click();
  await expect(olhaPage.getByText("Непрочитаних сповіщень немає.")).toBeVisible();
  await expect(olhaPage.getByRole("button", { name: "Сповіщення", exact: true })).toBeVisible();

  await olhaCtx.close();
  await andriiCtx.close();
  await bystanderCtx.close();
});

test("lesson planning validates times and URLs, detects overlaps, tracks rescheduling/cancellation, and orders schedule", async ({
  browser,
}) => {
  test.setTimeout(60_000);

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
    firstName: "Денис",
    lastName: "Інший",
    city: "Київ",
    formats: ["ONLINE"],
    teachingSkills: [{ skillId: IDS.skills.go, level: "INTERMEDIATE" }],
    learningSkills: [{ skillId: IDS.skills.english, level: "BEGINNER" }],
  });

  const olhaCtx = await browser.newContext();
  const andriiCtx = await browser.newContext();
  const strangerCtx = await browser.newContext();
  await world.attach(olhaCtx);
  await world.attach(andriiCtx);
  await world.attach(strangerCtx);
  await world.signIn(olhaCtx, olha.email);
  await world.signIn(andriiCtx, andrii.email);
  await world.signIn(strangerCtx, stranger.email);

  const olhaPage = await olhaCtx.newPage();
  const andriiPage = await andriiCtx.newPage();
  const strangerPage = await strangerCtx.newPage();

  // Create and accept an exchange (2 lessons each way = 4 lessons).
  await olhaPage.goto(`/students/${andrii.id}`);
  await olhaPage.getByRole("button", { name: "Запропонувати обмін" }).click();
  await olhaPage.getByRole("dialog").getByRole("radio", { name: "Онлайн" }).check();
  await olhaPage.getByRole("dialog").getByRole("button", { name: "Надіслати запит" }).click();
  await olhaPage.getByRole("dialog").getByRole("link", { name: "Переглянути запит" }).click();
  await expect(olhaPage).toHaveURL(/\/requests\/[0-9a-f-]+$/);
  const reqId = [...world.requests.keys()][0];

  await andriiPage.goto(`/requests/${reqId}`);
  await andriiPage.getByRole("button", { name: "Прийняти", exact: true }).click();
  await expect(andriiPage).toHaveURL(/\/exchanges\/[0-9a-f-]+\?started=1$/);
  const exchangeId = [...world.exchanges.keys()][0];
  const lessonIds = [...world.lessons.values()]
    .sort((a, b) => a.planOrder - b.planOrder)
    .map((l) => l.id);

  // Lesson 1: validate past time, duration bounds, and unsafe javascript: URL, then propose valid slot.
  await olhaPage.goto(`/sessions/${lessonIds[0]}`);
  await olhaPage.getByRole("button", { name: "Запланувати заняття" }).click();
  const form = olhaPage.getByRole("dialog");

  await form.getByLabel("Початок").fill(world.futureLocal(-60));
  await form.getByLabel("Завершення").fill(world.futureLocal(-30));
  await form.getByLabel("Посилання на зустріч").fill("javascript:alert(1)");
  await form.getByRole("button", { name: "Надіслати пропозицію" }).click();
  await expect(form.getByText("Обери коректний майбутній час.")).toBeVisible();
  await expect(form.getByText("Вкажи http(s) посилання на зустріч до 500 символів.")).toBeVisible();

  // End before start / < 15 minutes rejected.
  const slot1Start = world.futureLocal(180);
  const slot1TooShort = world.futureLocal(185);
  const slot1End = world.futureLocal(240);
  await form.getByLabel("Початок").fill(slot1Start);
  await form.getByLabel("Завершення").fill(slot1TooShort);
  await form.getByLabel("Посилання на зустріч").fill("https://meet.example.com/guitar-1");
  await form.getByRole("button", { name: "Надіслати пропозицію" }).click();
  await expect(
    form.getByText("Завершення має бути через 15–240 хвилин після початку."),
  ).toBeVisible();

  await form.getByLabel("Тема заняття").fill("Акорди та ритм");
  await form.getByLabel("Завершення").fill(slot1End);
  await form.getByRole("button", { name: "Надіслати пропозицію" }).click();
  await expect(
    olhaPage.getByText("Пропозицію надіслано. Очікуємо погодження партнера."),
  ).toBeVisible();
  await expect(olhaPage.getByText("Ти: погоджено · Партнер: очікує")).toBeVisible();

  // Andrii declines first, returning Lesson 1 to DRAFT, then Olha re-proposes and Andrii accepts.
  await andriiPage.goto(`/sessions/${lessonIds[0]}`);
  await expect(andriiPage.getByText("Ти: очікує · Партнер: погоджено")).toBeVisible();
  await andriiPage.getByRole("button", { name: "Відхилити", exact: true }).click();
  await expect(andriiPage.getByText("Пропозицію відхилено.")).toBeVisible();
  await expect(andriiPage.getByRole("button", { name: "Запланувати заняття" })).toBeVisible();

  await olhaPage.reload();
  await olhaPage.getByRole("button", { name: "Запланувати заняття" }).click();
  await olhaPage.getByRole("dialog").getByLabel("Тема заняття").fill("Акорди та бій");
  await olhaPage.getByRole("dialog").getByLabel("Початок").fill(slot1Start);
  await olhaPage.getByRole("dialog").getByLabel("Завершення").fill(slot1End);
  await olhaPage
    .getByRole("dialog")
    .getByLabel("Посилання на зустріч")
    .fill("https://meet.example.com/guitar-1");
  await olhaPage.getByRole("dialog").getByRole("button", { name: "Надіслати пропозицію" }).click();
  await expect(olhaPage.getByText(/Пропозицію надіслано/)).toBeVisible();

  await andriiPage.reload();
  await andriiPage.getByRole("button", { name: "Погодити час" }).click();
  await expect(andriiPage.getByText("Час погоджено. Заняття додано до розкладу.")).toBeVisible();

  // Lesson 2: overlapping proposal with Lesson 1 fails with 409 session_conflict, then earlier non-overlapping slot succeeds.
  await olhaPage.goto(`/sessions/${lessonIds[1]}`);
  await olhaPage.getByRole("button", { name: "Запланувати заняття" }).click();
  const form2 = olhaPage.getByRole("dialog");
  await form2.getByLabel("Тема заняття").fill("Перебір на гітарі");
  await form2.getByLabel("Початок").fill(world.futureLocal(210));
  await form2.getByLabel("Завершення").fill(world.futureLocal(270));
  await form2.getByLabel("Посилання на зустріч").fill("https://meet.example.com/guitar-2");
  await form2.getByRole("button", { name: "Надіслати пропозицію" }).click();
  await expect(
    form2.getByText("У цей час хтось із вас уже має погоджене заняття. Обери інший час."),
  ).toBeVisible();

  // Schedule Lesson 2 EARLIER than Lesson 1 (60..120 min vs 180..240 min) to verify chronological sorting in /schedule.
  await form2.getByLabel("Початок").fill(world.futureLocal(60));
  await form2.getByLabel("Завершення").fill(world.futureLocal(120));
  await form2.getByRole("button", { name: "Надіслати пропозицію" }).click();
  await expect(olhaPage.getByText(/Пропозицію надіслано/)).toBeVisible();

  await andriiPage.goto(`/sessions/${lessonIds[1]}`);
  await andriiPage.getByRole("button", { name: "Погодити час" }).click();
  await expect(andriiPage.getByText(/Час погоджено/)).toBeVisible();

  // Verify chronological order, Europe/Kyiv timezone, and safe external link on /schedule.
  await olhaPage.goto("/schedule");
  await expect(olhaPage.getByText(/Часовий пояс: Europe\/Kyiv/)).toBeVisible();
  const titles = await olhaPage.locator(".session-list h2").allInnerTexts();
  expect(titles).toEqual(["Перебір на гітарі", "Акорди та бій"]);

  const meetLink = olhaPage.getByRole("link", { name: "Відкрити зустріч ↗" }).first();
  await expect(meetLink).toHaveAttribute("target", "_blank");
  await expect(meetLink).toHaveAttribute("rel", "noopener noreferrer");

  // Stranger sees empty schedule and 404 on foreign lesson.
  await strangerPage.goto("/schedule");
  await expect(strangerPage.getByText("У розкладі поки немає занять.")).toBeVisible();
  await strangerPage.goto(`/sessions/${lessonIds[0]}`);
  await expect(strangerPage.getByText("Заняття не знайдено.")).toBeVisible();

  // Rescheduling Lesson 2 resets status to PROPOSED; cancelling Lesson 2 keeps progress at 0%.
  await olhaPage.goto(`/sessions/${lessonIds[1]}`);
  await olhaPage.getByRole("button", { name: "Перенести заняття" }).click();
  await olhaPage.getByRole("dialog").getByLabel("Початок").fill(world.futureLocal(300));
  await olhaPage.getByRole("dialog").getByLabel("Завершення").fill(world.futureLocal(360));
  await olhaPage.getByRole("dialog").getByRole("button", { name: "Надіслати пропозицію" }).click();
  await expect(olhaPage.getByText("Очікуємо відповіді партнера на пропозицію.")).toBeVisible();

  await olhaPage.getByRole("button", { name: "Скасувати заняття" }).click();
  await olhaPage
    .getByRole("dialog")
    .getByLabel("Причина скасування")
    .fill("Перенесемо на інший день");
  await olhaPage
    .getByRole("dialog")
    .getByRole("button", { name: "Скасувати заняття", exact: true })
    .click();
  await expect(olhaPage.getByText("Заняття скасовано. Історію збережено.")).toBeVisible();

  await olhaPage.goto(`/exchanges/${exchangeId}`);
  await expect(olhaPage.locator(".exchange-progress__percent")).toHaveText("0%");

  await olhaCtx.close();
  await andriiCtx.close();
  await strangerCtx.close();
});
