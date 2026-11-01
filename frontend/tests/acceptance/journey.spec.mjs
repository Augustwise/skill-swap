import { execFileSync } from "node:child_process";
import path from "node:path";
import { expect, test } from "../helpers/browser-version.mjs";

const password = "SkillSwapDemo2026";
const university = "10000000-0000-0000-0000-000000000001";

async function capture(page, testInfo, name) {
  await testInfo.attach(name, {
    body: await page.screenshot({ fullPage: true }),
    contentType: "image/png",
  });
}

async function register(page, testInfo, name, teach, learn) {
  const email = `lab5.${name}.${Date.now()}@students.example.test`;
  await page.goto("/register");
  await page.locator('[name="firstName"]').fill(name === "olha" ? "Ольга" : "Андрій");
  await page.locator('[name="lastName"]').fill("Тестенко");
  await page.locator('[name="university"]').selectOption(university);
  await page.locator('[name="email"]').fill(email);
  await page.locator('[name="password"]').fill(password);
  await page.locator("#register-agreement").check();
  await page.getByRole("button", { name: "Створити акаунт" }).click();
  await expect(page.getByRole("heading", { name: "Акаунт створено" })).toBeVisible();
  const mailpit = process.env.E2E_MAILPIT_URL;
  let message;
  await expect
    .poll(async () => {
      const response = await page.request.get(`${mailpit}/api/v1/messages`);
      message = (await response.json()).messages.find((item) =>
        item.To.some((to) => to.Address === email),
      );
      return Boolean(message);
    })
    .toBe(true);
  const detail = await (await page.request.get(`${mailpit}/api/v1/message/${message.ID}`)).json();
  const link = detail.Text.match(/http:\/\/localhost:3000\/[^\s]+/)[0];
  const inbox = await page.context().newPage();
  await inbox.goto(mailpit);
  await expect(inbox.getByText(email, { exact: false }).first()).toBeVisible();
  await capture(inbox, testInfo, `mailpit-${name}`);
  await inbox.close();
  await page.goto(link);
  await expect(page.getByRole("heading", { name: "Пошту підтверджено" })).toBeVisible();
  await page.getByRole("link", { name: "Перейти до входу" }).click();
  await page.getByLabel("Університетська пошта").fill(email);
  await page.getByLabel("Пароль", { exact: true }).fill(password);
  await page.getByRole("button", { name: "Увійти", exact: true }).click();
  await expect(page).toHaveURL(/\/onboarding$/);
  await page.locator('[name="city"]').fill("Київ");
  await page.locator('[name="bio"]').fill("Практична перевірка взаємного навчання.");
  await page.getByRole("button", { name: /Далі — додати навички/ }).click();
  await page.getByRole("button", { name: teach, exact: true }).click();
  await page.getByLabel(`Мій рівень для ${teach}`).selectOption("ADVANCED");
  await page.getByRole("button", { name: /Далі — що хочу вивчити/ }).click();
  await page.getByRole("button", { name: learn, exact: true }).click();
  await page.getByRole("button", { name: /Далі — обрати формат/ }).click();
  await page.getByRole("button", { name: "Онлайн", exact: true }).click();
  await page.getByRole("button", { name: /Завершити/ }).click();
  await expect(page).toHaveURL(/\/main$/);
  return (await (await page.request.get("/api/v1/auth/me")).json()).user.id;
}

test("real registration, Mailpit, mutual exchange, lessons and reviews", async ({
  browser,
}, testInfo) => {
  test.skip(process.env.E2E_MAILPIT !== "1", "Enable E2E_MAILPIT=1 for real SMTP acceptance");
  test.setTimeout(150_000);
  const contexts = await Promise.all([
    browser.newContext({
      baseURL: "http://localhost:3000",
      timezoneId: "Europe/Kyiv",
      viewport: { width: 1440, height: 1000 },
    }),
    browser.newContext({
      baseURL: "http://localhost:3000",
      timezoneId: "Europe/Kyiv",
      viewport: { width: 1440, height: 1000 },
    }),
  ]);
  try {
    const [a, b] = await Promise.all(contexts.map((context) => context.newPage()));
    await register(a, testInfo, "olha", "Гітара", "Photoshop");
    const partnerId = await register(b, testInfo, "andrii", "Photoshop", "Гітара");
    await a.goto("/matches");
    await expect(a.getByText("Андрій Тестенко").first()).toBeVisible();
    await capture(a, testInfo, "mutual-matches");
    await a.goto(`/students/${partnerId}`);
    await a.getByRole("button", { name: "Запропонувати обмін" }).click();
    const proposal = a.getByRole("dialog");
    await proposal.getByLabel("Ти проводиш: Занять").selectOption("1");
    await proposal.getByRole("radio", { name: "Онлайн", exact: true }).check();
    await proposal.getByLabel(/Повідомлення/).fill("Обміняймося по одному заняттю по 60 хвилин.");
    await capture(a, testInfo, "exchange-request");
    await proposal.getByRole("button", { name: "Надіслати запит" }).click();
    await proposal.getByRole("link", { name: "Переглянути запит" }).click();
    await expect(a).toHaveURL(/\/requests\/[0-9a-f-]+$/);
    const requestPath = new URL(a.url()).pathname;
    await a.getByRole("link", { name: "Написати повідомлення" }).click();
    await expect(a).toHaveURL(/\/messages\/[0-9a-f-]+$/);
    const chatPath = new URL(a.url()).pathname;
    await b.goto(chatPath);
    await a
      .getByLabel("Нове повідомлення, отримувач: Андрій")
      .fill("Домовилися! Почнемо з акордів і роботи з шарами.");
    await a.getByLabel("Нове повідомлення, отримувач: Андрій").press("Enter");
    await expect(b.locator(".chat-message__body").getByText(/Домовилися!/)).toBeVisible({
      timeout: 10_000,
    });
    await capture(b, testInfo, "private-chat");
    await b.goto(requestPath);
    await b.getByRole("button", { name: "Прийняти", exact: true }).click();
    await expect(b).toHaveURL(/\/exchanges\/[0-9a-f-]+\?started=1$/);
    const exchangeId = new URL(b.url()).pathname.split("/").pop();
    const sessions = (
      await (await b.request.get(`/api/v1/exchanges/${exchangeId}/sessions`)).json()
    ).items;
    expect(sessions).toHaveLength(2);
    for (const [i, session] of sessions.entries()) {
      await a.goto(`/sessions/${session.id}`);
      await a.getByRole("button", { name: "Запланувати заняття" }).click();
      const dialog = a.getByRole("dialog");
      await dialog
        .getByLabel("Тема заняття")
        .fill(i === 0 ? "Основи акордів" : "Шари та маски Photoshop");
      const local = (offset) =>
        new Intl.DateTimeFormat("sv-SE", {
          timeZone: "Europe/Kyiv",
          year: "numeric",
          month: "2-digit",
          day: "2-digit",
          hour: "2-digit",
          minute: "2-digit",
          hour12: false,
        })
          .format(new Date(Date.now() + offset * 60_000))
          .replace(" ", "T");
      await dialog.getByLabel("Початок").fill(local(60 + i * 120));
      await dialog.getByLabel("Завершення").fill(local(120 + i * 120));
      await dialog.getByLabel("Посилання на зустріч").fill("https://meet.example.com/lab5");
      await dialog.getByRole("button", { name: "Надіслати пропозицію" }).click();
      await expect(a.getByText(/Пропозицію надіслано/)).toBeVisible();
      await b.goto(`/sessions/${session.id}`);
      await b.getByRole("button", { name: "Погодити час" }).click();
      await expect(b.getByText(/Час погоджено/)).toBeVisible();
    }
    await a.goto("/schedule");
    await expect(a.getByText("Основи акордів")).toBeVisible();
    await capture(a, testInfo, "scheduled-lessons");
    const database = new URL(process.env.E2E_STACK_DATABASE_URL);
    if (database.hostname !== "127.0.0.1" || database.pathname !== "/skillswap_browser_acceptance")
      throw new Error("Time fixture requires the owned acceptance database");
    if (!/^[0-9a-f-]{36}$/.test(exchangeId)) throw new Error("Invalid fixture exchange ID");
    const sql = `UPDATE sessions SET scheduled_start=now()-interval '2 hours', scheduled_end=now()-interval '1 hour' WHERE exchange_id='${exchangeId}'::uuid;`;
    execFileSync(
      path.join(process.env.E2E_STACK_PG_BIN, process.platform === "win32" ? "psql.exe" : "psql"),
      [database.toString(), "-v", "ON_ERROR_STOP=1", "-c", sql],
      { windowsHide: true },
    );
    await testInfo.attach("time-fixture", {
      body: "Only the two owned lesson timestamps were moved to the past after real scheduling. All confirmations, completion and reviews use UI and the real API.",
      contentType: "text/plain",
    });
    for (const session of sessions) {
      for (const page of [a, b]) {
        await page.goto(`/sessions/${session.id}`);
        await page.getByRole("button", { name: "Заняття відбулося" }).click();
        await expect(page.locator(".completion-confirmations")).toContainText("Підтверджено ✓");
      }
    }
    for (const page of [a, b]) {
      await page.goto(`/exchanges/${exchangeId}`);
      await expect(page.locator(".exchange-progress__percent")).toHaveText("100%");
      await page.getByRole("button", { name: "Підтвердити завершення" }).click();
    }
    await expect(b.getByText("Обмін успішно завершено!").first()).toBeVisible();
    for (const page of [a, b]) {
      await page.reload();
      await page.getByRole("button", { name: "Залишити відгук", exact: true }).click();
      const dialog = page.getByRole("dialog");
      await dialog.getByRole("radio", { name: "5 з 5 — Чудово", exact: true }).check();
      await dialog
        .getByLabel("Текст відгуку", { exact: false })
        .fill("Корисне заняття з практичними вправами.");
      await dialog.getByRole("button", { name: "Надіслати відгук" }).click();
      await expect(page.getByRole("article", { name: "Твій відгук про партнера" })).toBeVisible();
    }
    await capture(b, testInfo, "completed-exchange-reviews");
    await a.goto(`/students/${partnerId}`);
    await expect(a.getByRole("region", { name: "Відгуки", exact: true })).toContainText("5,0");
    await capture(a, testInfo, "partner-rating");
    for (const width of [360, 768, 1440, 1920]) {
      await a.setViewportSize({ width, height: 1000 });
      await a.goto("/main");
      await expect(a.getByRole("heading", { name: "Привіт, Ольга!" })).toBeVisible();
      expect(await a.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      await capture(a, testInfo, `responsive-${width}`);
    }
  } finally {
    await Promise.all(contexts.map((context) => context.close()));
  }
});
