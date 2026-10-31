import { expect, test } from "../helpers/browser-version.mjs";
import { assertPageBudget, measurePageLoads } from "../helpers/performance.mjs";
import { BASE_URL, DEMO_PASSWORD, IDS, MAILPIT_URL, createMvpWorld } from "./mvp-fixture.mjs";

test("frontend render latency includes loaded page data across 30 local loads with a mocked API", async ({
  page,
}, testInfo) => {
  test.setTimeout(90_000);

  const world = createMvpWorld();
  const admin = world.seedUser({
    email: "perf.admin@students.example.test",
    firstName: "Олена",
    lastName: "Швидкість",
    role: "ADMIN",
    city: "Київ",
    formats: ["ONLINE"],
    teachingSkills: [{ skillId: IDS.skills.guitar, level: "ADVANCED" }],
    learningSkills: [{ skillId: IDS.skills.photoshop, level: "BEGINNER" }],
  });
  world.seedUser({
    email: "perf.partner@students.example.test",
    firstName: "Максим",
    lastName: "Партнер",
    city: "Київ",
    formats: ["ONLINE"],
    teachingSkills: [{ skillId: IDS.skills.photoshop, level: "ADVANCED" }],
    learningSkills: [{ skillId: IDS.skills.guitar, level: "BEGINNER" }],
  });

  await world.attach(page);

  await world.signIn(page, admin.email);
  const samples = await measurePageLoads(page, [
    {
      path: "/main",
      ready: async (page) => {
        await expect(page.getByRole("heading", { name: "Привіт, Олена!" })).toBeVisible();
        await expect(page.locator("#dashboard-matches")).toContainText("Максим Партнер");
        await expect(page.locator('main [aria-busy="true"]')).toHaveCount(0);
      },
    },
    {
      path: "/search?q=Photoshop",
      ready: async (page) => {
        await expect(page.getByRole("heading", { name: "Максим Партнер" })).toBeVisible();
        await expect(page.locator(".discovery-results")).toHaveAttribute("aria-busy", "false");
        await expect(page.getByRole("combobox", { name: /^Категорія/ })).toBeEnabled();
      },
    },
    {
      path: "/profile",
      ready: async (page) => {
        await expect(page.getByLabel("Ім’я", { exact: true })).toHaveValue("Олена");
        await expect(page.getByText(admin.email, { exact: true })).toBeVisible();
      },
    },
  ]);
  await assertPageBudget(samples, testInfo, { api: "mock" });
});

test("open chat receives partner message through polling well within 10 seconds", async ({
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

  const olhaCtx = await browser.newContext();
  const andriiCtx = await browser.newContext();
  await world.attach(olhaCtx);
  await world.attach(andriiCtx);
  await world.signIn(olhaCtx, olha.email);
  await world.signIn(andriiCtx, andrii.email);

  const olhaPage = await olhaCtx.newPage();
  const andriiPage = await andriiCtx.newPage();

  await olhaPage.goto(`/students/${andrii.id}`);
  await olhaPage.getByRole("button", { name: "Запропонувати обмін" }).click();
  await olhaPage.getByRole("dialog").getByRole("button", { name: "Надіслати запит" }).click();
  await olhaPage.getByRole("dialog").getByRole("link", { name: "Переглянути запит" }).click();
  await expect(olhaPage).toHaveURL(/\/requests\/[0-9a-f-]+$/);
  await olhaPage.getByRole("link", { name: "Написати повідомлення" }).click();
  await expect(olhaPage).toHaveURL(/\/messages\/[0-9a-f-]+$/);
  const conversationId = [...world.conversations.keys()][0];

  await andriiPage.goto(`/messages/${conversationId}`);
  const composer = andriiPage.getByLabel("Нове повідомлення, отримувач: Ольга");
  await composer.fill("Повідомлення реального часу через опитування");

  const sentAt = performance.now();
  await composer.press("Enter");
  await expect(
    olhaPage
      .locator(".chat-message__body")
      .getByText("Повідомлення реального часу через опитування"),
  ).toBeVisible({ timeout: 10_000 });
  const deliveryMs = performance.now() - sentAt;

  expect(deliveryMs).toBeLessThanOrEqual(10_000);

  await olhaCtx.close();
  await andriiCtx.close();
});

test("transient 500 and network failures produce zero uncaught exceptions, preserve drafts, and recover via retry", async ({
  page,
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

  await world.attach(page);
  await world.signIn(page, olha.email);

  const uncaughtErrors = [];
  page.on("pageerror", (err) => uncaughtErrors.push(err.message));

  // 1. Read failure (500 internal_error) on /search -> clear Ukrainian retry -> clean load.
  world.addFault({ method: "GET", path: "students", status: 500, code: "internal_error" });
  await page.goto("/search");
  await expect(page.getByRole("button", { name: "Спробувати ще раз" })).toBeVisible();
  await page.getByRole("button", { name: "Спробувати ще раз" }).click();
  await expect(page.getByText("Андрій Шевченко")).toBeVisible();

  // 2. Network abort on privacy write -> form state preserved -> retry succeeds.
  await page.goto("/settings");
  const discoverableSwitch = page.getByRole("switch", {
    name: "Показувати мій профіль",
    exact: false,
  });
  await discoverableSwitch.uncheck();
  world.addFault({ method: "PATCH", path: "me/privacy", kind: "abort" });
  await page.getByRole("button", { name: "Зберегти налаштування" }).click();
  await expect(page.locator(".safety-feedback")).toBeVisible();
  await expect(discoverableSwitch).not.toBeChecked();
  await page.getByRole("button", { name: "Зберегти налаштування" }).click();
  await expect(page.getByRole("status")).toContainText("збережено");

  // 3. Server 503 on exchange request creation -> typed message preserved -> retry succeeds.
  await page.goto(`/students/${andrii.id}`);
  await page.getByRole("button", { name: "Запропонувати обмін" }).click();
  const dialog = page.getByRole("dialog");
  await dialog.getByLabel(/Повідомлення/).fill("Чернетка запиту не губиться при збої");
  world.addFault({ method: "POST", path: "exchange-requests", status: 503, code: "unavailable" });
  await dialog.getByRole("button", { name: "Надіслати запит" }).click();
  await expect(dialog.getByRole("alert")).toBeVisible();
  await expect(dialog.getByLabel(/Повідомлення/)).toHaveValue(
    "Чернетка запиту не губиться при збої",
  );
  await dialog.getByRole("button", { name: "Надіслати запит" }).click();
  await expect(dialog.getByRole("heading", { name: "Запит надіслано" })).toBeVisible();

  expect(uncaughtErrors).toEqual([]);
});

test("pending mutations disable repeat clicks and server endpoints remain idempotent", async ({
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

  const olhaCtx = await browser.newContext();
  const andriiCtx = await browser.newContext();
  await world.attach(olhaCtx);
  await world.attach(andriiCtx);
  await world.signIn(olhaCtx, olha.email);
  const andriiSession = await world.signIn(andriiCtx, andrii.email);

  const olhaPage = await olhaCtx.newPage();
  const andriiPage = await andriiCtx.newPage();

  await olhaPage.goto(`/students/${andrii.id}`);
  await olhaPage.getByRole("button", { name: "Запропонувати обмін" }).click();
  await olhaPage.getByRole("dialog").getByLabel("Ти проводиш: Занять").selectOption("1");
  await olhaPage.getByRole("dialog").getByRole("button", { name: "Надіслати запит" }).click();
  await olhaPage.getByRole("dialog").getByRole("link", { name: "Переглянути запит" }).click();
  await expect(olhaPage).toHaveURL(/\/requests\/[0-9a-f-]+$/);
  const requestId = [...world.requests.keys()][0];

  // Hold accept request in flight and verify button disables immediately against double-clicks.
  let releaseAccept;
  const holdAccept = new Promise((resolve) => {
    releaseAccept = resolve;
  });
  world.addFault({
    method: "POST",
    path: `exchange-requests/${requestId}/accept`,
    kind: "hold",
    hold: holdAccept,
  });

  await andriiPage.goto(`/requests/${requestId}`);
  await andriiPage.getByRole("button", { name: "Прийняти", exact: true }).click();
  await expect(andriiPage.getByRole("button", { name: "Приймаємо…" })).toBeDisabled();
  releaseAccept();

  await expect(andriiPage).toHaveURL(/\/exchanges\/[0-9a-f-]+\?started=1$/);
  expect(world.exchanges.size).toBe(1);
  const exchangeId = [...world.exchanges.keys()][0];

  // Repeat POST to accept is idempotent: returns same exchange and creates no extra exchange or notification.
  const notificationsBefore = (world.notifications.get(olha.id) ?? []).length;
  const repeatAccept = await andriiPage.evaluate(
    async ({ reqId, csrf }) => {
      const res = await fetch(`/api/v1/exchange-requests/${reqId}/accept`, {
        method: "POST",
        headers: { "X-CSRF-Token": csrf },
      });
      return { status: res.status, data: await res.json() };
    },
    { reqId: requestId, csrf: andriiSession.csrfToken },
  );
  expect(repeatAccept.status).toBe(200);
  expect(repeatAccept.data.exchange.id).toBe(exchangeId);
  expect(world.exchanges.size).toBe(1);
  expect((world.notifications.get(olha.id) ?? []).length).toBe(notificationsBefore);

  await olhaCtx.close();
  await andriiCtx.close();
});

test("session cookie is HttpOnly SameSite=Lax, browser storage holds no secrets, token pages strip tokens with no-referrer, and CSRF rotation recovers", async ({
  page,
}) => {
  const world = createMvpWorld();
  const user = world.seedUser({
    email: "sec@students.example.test",
    firstName: "Назар",
    lastName: "Безпека",
    city: "Київ",
    formats: ["ONLINE"],
    teachingSkills: [{ skillId: IDS.skills.guitar, level: "ADVANCED" }],
    learningSkills: [{ skillId: IDS.skills.photoshop, level: "BEGINNER" }],
  });
  await world.attach(page);

  // Sign in via real login form so Set-Cookie headers are applied by the browser.
  await page.goto("/login");
  await page.getByLabel("Університетська пошта").fill(user.email);
  await page.getByLabel("Пароль", { exact: true }).fill(DEMO_PASSWORD);
  await page.getByRole("button", { name: "Увійти", exact: true }).click();
  await expect(page).toHaveURL(`${BASE_URL}/main`);

  const cookies = await page.context().cookies();
  const sessionCookie = cookies.find((c) => c.name === "skillswap_session");
  expect(sessionCookie).toBeDefined();
  expect(sessionCookie.httpOnly).toBe(true);
  expect(sessionCookie.sameSite).toBe("Lax");
  expect(sessionCookie.path).toBe("/");

  // HttpOnly session cookie is invisible to document.cookie; web storage holds zero tokens or passwords.
  const storageSnapshot = await page.evaluate(() => ({
    cookie: document.cookie,
    local: Object.entries(localStorage),
    session: Object.entries(sessionStorage),
  }));
  expect(storageSnapshot.cookie).toBe("");
  const serializedStorage = JSON.stringify([storageSnapshot.local, storageSnapshot.session]);
  expect(serializedStorage).not.toContain(sessionCookie.value);
  expect(serializedStorage).not.toContain(DEMO_PASSWORD);
  expect(serializedStorage).not.toContain("csrf_");

  // Direct write without X-CSRF-Token is rejected with 403 csrf_invalid.
  const withoutCsrf = await page.evaluate(async () => {
    const res = await fetch("/api/v1/me/privacy", {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        isDiscoverable: false,
        acceptsRequests: true,
        requestScope: "ANYONE",
      }),
    });
    return { status: res.status, body: await res.json() };
  });
  expect(withoutCsrf.status).toBe(403);
  expect(withoutCsrf.body.error.code).toBe("csrf_invalid");

  // Rotating server CSRF token mid-session triggers automatic /auth/me token refresh and transparent retry.
  await page.goto("/settings");
  await page.getByRole("switch", { name: "Показувати мій профіль", exact: false }).uncheck();
  world.rotateCsrf(sessionCookie.value);
  await page.getByRole("button", { name: "Зберегти налаштування" }).click();
  await expect(page.getByRole("status")).toContainText("збережено");

  // /verify-email and /reset-password enforce referrer=no-referrer and strip query tokens.
  await page.context().clearCookies();
  await page.goto("/verify-email?token=bad-token");
  await expect(page.locator('meta[name="referrer"]')).toHaveAttribute("content", "no-referrer");
  await expect(page).toHaveURL(`${BASE_URL}/verify-email`);

  await page.context().clearCookies();
  await page.goto("/login");
  await page.getByRole("button", { name: "Забув пароль?" }).click();
  await page.getByLabel("Університетська пошта").fill(user.email);
  await page.getByRole("button", { name: "Надіслати посилання" }).click();
  await expect(page.getByRole("heading", { name: "Перевір пошту" })).toBeVisible();

  await page.goto(`${MAILPIT_URL}/?to=${user.email}`);
  await page.getByRole("link", { name: `Відкрити посилання для ${user.email}` }).click();
  await expect(page.locator('meta[name="referrer"]')).toHaveAttribute("content", "no-referrer");
  await page.locator("#reset-password").fill("UpdatedPassword2026");
  await page.locator("#reset-confirmation").fill("UpdatedPassword2026");
  await page.getByRole("button", { name: "Зберегти пароль" }).click();
  await expect(page.getByRole("heading", { name: "Пароль змінено" })).toBeVisible();
  await expect(page).toHaveURL(`${BASE_URL}/reset-password`);
});

test("untrusted HTML and script payloads render as inert text everywhere and private fields stay hidden on public profiles", async ({
  browser,
}) => {
  test.setTimeout(90_000);
  const xss = '<script>window.__xss=1</script><img src=x onerror="window.__xss=1">';
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
    email: "andrii.secret@students.example.test",
    firstName: "Андрій",
    lastName: "Шевченко",
    city: "Київ",
    bio: `Про мене: ${xss}`,
    formats: ["ONLINE", "OFFLINE"],
    teachingSkills: [{ skillId: IDS.skills.photoshop, level: "ADVANCED" }],
    learningSkills: [{ skillId: IDS.skills.guitar, level: "BEGINNER" }],
  });

  const olhaCtx = await browser.newContext();
  const andriiCtx = await browser.newContext();
  await world.attach(olhaCtx);
  await world.attach(andriiCtx);
  await world.signIn(olhaCtx, olha.email);
  await world.signIn(andriiCtx, andrii.email);

  const olhaPage = await olhaCtx.newPage();
  const andriiPage = await andriiCtx.newPage();

  // Search query with XSS payload.
  await olhaPage.goto(`/search?q=${encodeURIComponent(xss)}`);
  await expect(olhaPage.locator("[onerror]")).toHaveCount(0);
  expect(await olhaPage.evaluate(() => window.__xss)).toBeUndefined();

  // Candidate profile renders bio payload as plain text and hides email & private meeting details.
  const [dtoRes] = await Promise.all([
    olhaPage.waitForResponse((res) => res.url().endsWith(`/api/v1/students/${andrii.id}`)),
    olhaPage.goto(`/students/${andrii.id}`),
  ]);
  const studentDtoText = await dtoRes.text();
  await expect(olhaPage.getByText(xss)).toBeVisible();
  expect(studentDtoText).not.toContain("andrii.secret@students.example.test");
  await expect(olhaPage.locator("main")).not.toContainText("andrii.secret@students.example.test");

  // Exchange request message, chat message, lesson title/place, review comment, and report reason with XSS payload.
  await olhaPage.getByRole("button", { name: "Запропонувати обмін" }).click();
  const propDialog = olhaPage.getByRole("dialog");
  await propDialog.getByRole("radio", { name: /Офлайн/ }).check();
  await propDialog.getByLabel("Ти проводиш: Занять").selectOption("1");
  await propDialog.getByLabel(/Повідомлення/).fill(xss);
  await propDialog.getByRole("button", { name: "Надіслати запит" }).click();
  await propDialog.getByRole("link", { name: "Переглянути запит" }).click();
  await expect(olhaPage).toHaveURL(/\/requests\/[0-9a-f-]+$/);
  await expect(olhaPage.getByText(xss)).toBeVisible();
  const requestId = [...world.requests.keys()][0];

  await olhaPage.getByRole("link", { name: "Написати повідомлення" }).click();
  await expect(olhaPage).toHaveURL(/\/messages\/[0-9a-f-]+$/);
  const convId = [...world.conversations.keys()][0];
  await olhaPage.getByLabel("Нове повідомлення, отримувач: Андрій").fill(xss);
  await olhaPage.getByLabel("Нове повідомлення, отримувач: Андрій").press("Enter");
  await expect(olhaPage.locator(".chat-message__body").getByText(xss)).toBeVisible();

  await andriiPage.goto(`/requests/${requestId}`);
  await andriiPage.getByRole("button", { name: "Прийняти", exact: true }).click();
  await expect(andriiPage).toHaveURL(/\/exchanges\/[0-9a-f-]+\?started=1$/);
  const exchangeId = [...world.exchanges.keys()][0];
  const lessonIds = [...world.lessons.values()]
    .sort((a, b) => a.planOrder - b.planOrder)
    .map((l) => l.id);

  for (let i = 0; i < 2; i++) {
    await olhaPage.goto(`/sessions/${lessonIds[i]}`);
    await olhaPage.getByRole("button", { name: "Запланувати заняття" }).click();
    const d = olhaPage.getByRole("dialog");
    await d.getByLabel("Тема заняття").fill(`Тема ${i + 1} ${xss}`);
    await d.getByLabel("Початок").fill(world.futureLocal(60 + i * 120));
    await d.getByLabel("Завершення").fill(world.futureLocal(120 + i * 120));
    await d.getByLabel("Місце зустрічі").fill(`Приватна кімната 404 ${xss}`);
    await d.getByRole("button", { name: "Надіслати пропозицію" }).click();
    await expect(olhaPage.getByText(/Пропозицію надіслано/)).toBeVisible();
    await andriiPage.goto(`/sessions/${lessonIds[i]}`);
    await andriiPage.getByRole("button", { name: "Погодити час" }).click();
    await expect(andriiPage.getByText(/Час погоджено/)).toBeVisible();
  }

  // Private meetingPlace must NOT leak onto public candidate profile.
  await olhaPage.goto(`/students/${andrii.id}`);
  await expect(olhaPage.locator("main")).not.toContainText("Приватна кімната 404");

  // Complete exchange and leave review + report containing XSS payload.
  world.advanceMinutes(360);
  for (const id of lessonIds) {
    await olhaPage.goto(`/sessions/${id}`);
    await olhaPage.getByRole("button", { name: "Заняття відбулося" }).click();
    await expect(olhaPage.locator(".completion-confirmations")).toContainText("Підтверджено ✓");
    await andriiPage.goto(`/sessions/${id}`);
    await andriiPage.getByRole("button", { name: "Заняття відбулося" }).click();
    await expect(andriiPage.locator(".completion-confirmations")).toContainText("Підтверджено ✓");
  }
  await olhaPage.goto(`/exchanges/${exchangeId}`);
  await olhaPage.getByRole("button", { name: "Підтвердити завершення" }).click();
  await expect(
    olhaPage.getByText("Ти підтвердив. Очікуємо підтвердження партнера.").first(),
  ).toBeVisible();
  await andriiPage.goto(`/exchanges/${exchangeId}`);
  await andriiPage.getByRole("button", { name: "Підтвердити завершення" }).click();
  await expect(andriiPage.getByText("Обмін успішно завершено!").first()).toBeVisible();

  await olhaPage.goto(`/exchanges/${exchangeId}`);
  await olhaPage.getByRole("button", { name: "Залишити відгук", exact: true }).click();
  await olhaPage
    .getByRole("dialog")
    .getByRole("radio", { name: "5 з 5 — Чудово", exact: true })
    .check();
  await olhaPage.getByRole("dialog").getByLabel("Текст відгуку", { exact: false }).fill(xss);
  await olhaPage.getByRole("dialog").getByRole("button", { name: "Надіслати відгук" }).click();
  await expect(olhaPage.getByRole("article", { name: "Твій відгук про партнера" })).toContainText(
    xss,
  );

  await olhaPage.getByRole("button", { name: "Поскаржитися" }).click();
  await olhaPage.getByRole("dialog").getByLabel("Причина скарги").fill(xss);
  await olhaPage.getByRole("dialog").getByRole("button", { name: "Надіслати скаргу" }).click();
  await olhaPage.goto("/settings?tab=reports");
  await expect(olhaPage.locator(".safety-text")).toHaveText(xss);

  for (const checkPath of [
    `/students/${andrii.id}`,
    `/messages/${convId}`,
    `/sessions/${lessonIds[0]}`,
    `/exchanges/${exchangeId}`,
    "/settings?tab=reports",
  ]) {
    await olhaPage.goto(checkPath);
    expect(await olhaPage.evaluate(() => window.__xss)).toBeUndefined();
    await expect(olhaPage.locator("main script")).toHaveCount(0);
    await expect(olhaPage.locator("[onerror]")).toHaveCount(0);
  }

  await olhaCtx.close();
  await andriiCtx.close();
});

test("key interfaces fit 360, 768, 1440, and 1920 px viewports without horizontal overflow and satisfy keyboard and form accessibility", async ({
  page,
}) => {
  test.setTimeout(60_000);

  const world = createMvpWorld();
  const user = world.seedUser({
    email: "a11y@students.example.test",
    firstName: "Соломія",
    lastName: "Доступність",
    city: "Київ",
    formats: ["ONLINE"],
    teachingSkills: [{ skillId: IDS.skills.guitar, level: "ADVANCED" }],
    learningSkills: [{ skillId: IDS.skills.photoshop, level: "BEGINNER" }],
  });
  await world.attach(page);

  const widths = [360, 768, 1440, 1920];
  for (const width of widths) {
    await page.setViewportSize({ width, height: 900 });
    await page.context().clearCookies();

    for (const publicRoute of ["/", "/login", "/register"]) {
      await page.goto(publicRoute);
      await page.locator("h1").first().waitFor({ state: "visible" });
      const metrics = await page.evaluate(() => ({
        lang: document.documentElement.lang,
        scrollWidth: document.documentElement.scrollWidth,
        innerWidth: window.innerWidth,
      }));
      expect(metrics.lang).toBe("uk");
      expect(metrics.scrollWidth).toBeLessThanOrEqual(metrics.innerWidth);
    }

    await world.signIn(page, user.email);
    for (const privateRoute of ["/main", "/search", "/profile", "/schedule"]) {
      await page.goto(privateRoute);
      if (privateRoute === "/main")
        await expect(page.getByRole("heading", { name: "Привіт, Соломія!" })).toBeVisible();
      if (privateRoute === "/search")
        await expect(page.locator(".discovery-results")).toHaveAttribute("aria-busy", "false");
      if (privateRoute === "/profile")
        await expect(page.getByLabel("Ім’я", { exact: true })).toHaveValue("Соломія");
      if (privateRoute === "/schedule")
        await expect(page.getByText("У розкладі поки немає занять.")).toBeVisible();
      await expect(page.locator('main [aria-busy="true"]')).toHaveCount(0);
      const metrics = await page.evaluate(() => ({
        lang: document.documentElement.lang,
        scrollWidth: document.documentElement.scrollWidth,
        innerWidth: window.innerWidth,
      }));
      expect(metrics.lang).toBe("uk");
      expect(metrics.scrollWidth).toBeLessThanOrEqual(metrics.innerWidth);
    }
  }

  // Keyboard Tab reaches skip link with visible focus indicator and moves focus to main content.
  await page.setViewportSize({ width: 1280, height: 800 });
  await page.goto("/main");
  const skipLink = page.getByRole("link", { name: "До вмісту" });
  await skipLink.waitFor({ state: "attached" });
  await page.evaluate(() => {
    if (document.activeElement instanceof HTMLElement) document.activeElement.blur();
  });
  await page.keyboard.press("Tab");
  await expect(skipLink).toBeFocused();
  const focusVisual = await skipLink.evaluate((el) => {
    const s = getComputedStyle(el);
    return { outlineStyle: s.outlineStyle, outlineWidth: s.outlineWidth, boxShadow: s.boxShadow };
  });
  expect(
    (focusVisual.outlineStyle !== "none" && focusVisual.outlineWidth !== "0px") ||
      focusVisual.boxShadow !== "none",
  ).toBe(true);
  await page.keyboard.press("Enter");
  await expect(page.locator("main")).toBeFocused();

  // Every visible form control on /register, /profile, and /settings has an accessible label; errors set aria-invalid + text.
  async function assertControlsLabeled() {
    const unlabeled = await page.evaluate(() => {
      const controls = [
        ...document.querySelectorAll('input:not([type="hidden"]), select, textarea'),
      ].filter((el) => {
        const rect = el.getBoundingClientRect();
        return rect.width > 0 && rect.height > 0 && getComputedStyle(el).visibility !== "hidden";
      });
      return controls
        .filter((el) => {
          const ariaLabel = el.getAttribute("aria-label")?.trim();
          const labelledBy = el.getAttribute("aria-labelledby");
          const labels = "labels" in el && el.labels ? [...el.labels] : [];
          return !ariaLabel && !labelledBy && labels.length === 0;
        })
        .map((el) => el.outerHTML.slice(0, 120));
    });
    expect(unlabeled).toEqual([]);
  }

  await page.goto("/profile");
  await page.getByRole("link", { name: "До вмісту" }).waitFor({ state: "attached" });
  await assertControlsLabeled();

  await page.goto("/settings");
  await page.getByRole("link", { name: "До вмісту" }).waitFor({ state: "attached" });
  await assertControlsLabeled();

  await page.context().clearCookies();
  await page.goto("/register");
  await page.locator("h1").first().waitFor({ state: "visible" });
  await assertControlsLabeled();

  await page.getByRole("button", { name: "Створити акаунт" }).click();
  await expect(page.locator('input[name="firstName"]')).toHaveAttribute("aria-invalid", "true");
  await expect(page.getByText("Введи ім’я.")).toBeVisible();
});
