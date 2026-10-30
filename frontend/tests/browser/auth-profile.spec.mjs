import { expect, test } from "@playwright/test";
import { BASE_URL, DEMO_PASSWORD, IDS, MAILPIT_URL, createMvpWorld } from "./mvp-fixture.mjs";

test("guest sees public pages, is blocked from private routes, and unsafe next URLs are sanitized", async ({
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
  await world.attach(page);

  await page.goto("/");
  await expect(page).toHaveTitle(/SkillSwap/i);
  await expect(page.getByRole("link", { name: "Спробувати" })).toHaveAttribute("href", "/register");

  await page.goto("/terms");
  await expect(page.getByRole("heading", { level: 1 })).toBeVisible();

  for (const protectedPath of [
    "/main",
    "/profile",
    "/search",
    "/matches",
    "/requests",
    "/exchanges",
    "/schedule",
    "/messages",
    "/notifications",
    "/settings",
    "/admin",
  ]) {
    await page.goto(protectedPath);
    await expect(page.getByRole("heading", { name: "Увійди, щоб продовжити" })).toBeVisible();
    const loginLink = page.getByRole("link", { name: "Увійти", exact: true });
    await expect(loginLink).toHaveAttribute(
      "href",
      new RegExp(`^/login\\?next=${encodeURIComponent(protectedPath)}`),
    );
  }

  await page.goto("/onboarding");
  await expect(page.getByRole("heading", { name: "Увійди, щоб продовжити" })).toBeVisible();

  await page.goto("/login?next=//evil.example/phish");
  await page.getByLabel("Університетська пошта").fill(olha.email);
  await page.getByLabel("Пароль", { exact: true }).fill(DEMO_PASSWORD);
  await page.getByRole("button", { name: "Увійти", exact: true }).click();
  await expect(page).toHaveURL(`${BASE_URL}/main`);

  await page.getByRole("button", { name: "Вийти" }).click();
  await expect(page).toHaveURL(`${BASE_URL}/login`);

  await page.goto("/login?next=/schedule");
  await page.getByLabel("Університетська пошта").fill(olha.email);
  await page.getByLabel("Пароль", { exact: true }).fill(DEMO_PASSWORD);
  await page.getByRole("button", { name: "Увійти", exact: true }).click();
  await expect(page).toHaveURL(`${BASE_URL}/schedule`);
});

test("registration validates fields and UTF-8 password limits, retries catalog, and verifies via Mailpit", async ({
  page,
}) => {
  const world = createMvpWorld();
  world.seedUser({
    email: "existing@students.example.test",
    firstName: "Ірина",
    lastName: "Мельник",
  });
  world.addFault({ method: "GET", path: "universities", status: 503, code: "unavailable" });
  await world.attach(page);

  await page.goto("/register");
  await expect(
    page.getByText("Не вдалося завантажити університети. Спробуй ще раз."),
  ).toBeVisible();
  await page.getByRole("button", { name: "Завантажити університети ще раз" }).click();
  const uniSelect = page.locator('select[name="university"]');
  await expect(uniSelect).toBeEnabled();

  await page.getByRole("button", { name: "Створити акаунт" }).click();
  await expect(page.getByText("Введи ім’я.")).toBeVisible();
  await expect(page.locator('input[name="firstName"]')).toBeFocused();

  await page.locator('input[name="firstName"]').fill("Тарас");
  await page.locator('input[name="lastName"]').fill("Бондар");
  await uniSelect.selectOption(IDS.universities.kpi);
  await page.locator('input[name="email"]').fill("taras@knu.example.test");
  await page.locator("#register-agreement").check();

  // 6 four-byte emojis = 24 UTF-8 bytes, 12 UTF-16 code units, but only 6 Unicode code points.
  await page.locator('input[name="password"]').fill("🎸🎸🎸🎸🎸🎸");
  await page.getByRole("button", { name: "Створити акаунт" }).click();
  await expect(page.getByText("Пошта має належати домену students.example.test.")).toBeVisible();
  await expect(page.getByText("Пароль має містити щонайменше 12 символів.")).toBeVisible();

  // 37 two-byte Cyrillic letters = 74 UTF-8 bytes (> 72 bytes limit).
  await page.locator('input[name="email"]').fill("  EXISTING@students.example.test ");
  await page.locator('input[name="password"]').fill("а".repeat(37));
  await page.getByRole("button", { name: "Створити акаунт" }).click();
  await expect(page.getByText("Пароль не може перевищувати 72 байти.")).toBeVisible();

  // Valid 12-char password with normalized duplicate email triggers 409 email_taken.
  await page.locator('input[name="password"]').fill(DEMO_PASSWORD);
  await page.getByRole("button", { name: "Створити акаунт" }).click();
  await expect(page.getByText("Акаунт із цією поштою вже існує.")).toBeVisible();

  // Register a fresh student and confirm via local Mailpit inbox.
  await page.locator('input[name="email"]').fill("  Taras.Bondar@students.example.test ");
  await page.getByRole("button", { name: "Створити акаунт" }).click();
  await expect(page.getByRole("heading", { name: "Акаунт створено" })).toBeVisible();
  await expect(page.getByText(/taras\.bondar@students\.example\.test/)).toBeVisible();

  await page.goto(`${MAILPIT_URL}/?to=taras.bondar@students.example.test`);
  await expect(page.getByRole("heading", { name: "Локальна пошта Mailpit" })).toBeVisible();
  const mailLink = page.getByRole("link", {
    name: "Відкрити посилання для taras.bondar@students.example.test",
  });
  const verifyHref = await mailLink.getAttribute("href");
  await mailLink.click();

  await expect(page.getByRole("heading", { name: "Пошту підтверджено" })).toBeVisible();
  await expect(page).toHaveURL(`${BASE_URL}/verify-email`);

  // Re-opening the same single-use token must fail with invalid-link message and strip token.
  await page.goto(verifyHref);
  await expect(page.getByRole("heading", { name: "Посилання не працює" })).toBeVisible();
  await expect(page).toHaveURL(`${BASE_URL}/verify-email`);

  // Legacy /onboarding?token=... redirects to /verify-email and strips malformed token.
  await page.goto("/onboarding?token=bad-token");
  await expect(page.getByRole("heading", { name: "Посилання не працює" })).toBeVisible();
  await expect(page).toHaveURL(`${BASE_URL}/verify-email`);
});

test("unverified user is kept on verify-email, can resend confirmation mail, and can log out", async ({
  page,
}) => {
  const world = createMvpWorld();
  world.config.mailAvailable = false;
  await world.attach(page);

  await page.goto("/register");
  await page.locator('input[name="firstName"]').fill("Віктор");
  await page.locator('input[name="lastName"]').fill("Савчук");
  await page.locator('select[name="university"]').selectOption(IDS.universities.kpi);
  await page.locator('input[name="email"]').fill("viktor@students.example.test");
  await page.locator('input[name="password"]').fill(DEMO_PASSWORD);
  await page.locator("#register-agreement").check();
  await page.getByRole("button", { name: "Створити акаунт" }).click();
  await expect(page.getByText(/але лист не вдалося надіслати/i)).toBeVisible();

  world.config.mailAvailable = true;
  await page.getByRole("link", { name: "Перейти до входу" }).click();
  await page.getByLabel("Університетська пошта").fill("viktor@students.example.test");
  await page.getByLabel("Пароль", { exact: true }).fill(DEMO_PASSWORD);
  await page.getByRole("button", { name: "Увійти", exact: true }).click();
  await expect(page).toHaveURL(`${BASE_URL}/verify-email`);
  await expect(page.getByRole("heading", { name: "Підтвердь свою пошту" })).toBeVisible();

  // Direct navigation to private routes redirects unverified user back to /verify-email.
  await page.goto("/search");
  await expect(page).toHaveURL(`${BASE_URL}/verify-email`);

  await page.getByRole("button", { name: "Надіслати лист повторно" }).click();
  await expect(page.getByText(/Лист надіслано повторно/)).toBeVisible();
  await expect(page.getByRole("button", { name: /Повторно через/ })).toBeDisabled();
  expect(world.mailbox[0].to).toBe("viktor@students.example.test");

  await page.getByRole("button", { name: "Вийти з акаунта" }).click();
  await expect(page).toHaveURL(`${BASE_URL}/login`);
});

test("login enforces rate-limit lockout and password recovery invalidates old sessions", async ({
  browser,
}) => {
  const world = createMvpWorld();
  const user = world.seedUser({
    email: "nadia@students.example.test",
    firstName: "Надія",
    lastName: "Лисенко",
    city: "Київ",
    formats: ["ONLINE"],
    teachingSkills: [{ skillId: IDS.skills.english, level: "ADVANCED" }],
    learningSkills: [{ skillId: IDS.skills.go, level: "BEGINNER" }],
  });

  const existingCtx = await browser.newContext();
  await world.attach(existingCtx);
  await world.signIn(existingCtx, user.email);
  const existingPage = await existingCtx.newPage();
  await existingPage.goto("/main");
  await expect(existingPage.getByRole("heading", { name: "Привіт, Надія!" })).toBeVisible();

  const recoveryCtx = await browser.newContext();
  await world.attach(recoveryCtx);
  const page = await recoveryCtx.newPage();
  await page.goto("/login");

  for (let attempt = 1; attempt <= 4; attempt++) {
    await page.getByLabel("Університетська пошта").fill(user.email);
    await page.getByLabel("Пароль", { exact: true }).fill("WrongPassword999");
    await page.getByRole("button", { name: "Увійти", exact: true }).click();
    await expect(page.locator(".register-form__error")).toContainText("Невірна пошта або пароль.");
  }

  await page.getByLabel("Пароль", { exact: true }).fill("WrongPassword999");
  await page.getByRole("button", { name: "Увійти", exact: true }).click();
  await expect(page.locator(".register-form__error")).toContainText("Забагато спроб.");

  await page.getByRole("button", { name: "Забув пароль?" }).click();
  await expect(page.getByRole("heading", { name: "Відновити пароль" })).toBeVisible();
  await page.getByLabel("Університетська пошта").fill(user.email);
  await page.getByRole("button", { name: "Надіслати посилання" }).click();
  await expect(page.getByRole("heading", { name: "Перевір пошту" })).toBeVisible();

  await page.goto(`${MAILPIT_URL}/?to=${user.email}`);
  const resetLink = page.getByRole("link", { name: `Відкрити посилання для ${user.email}` });
  const resetHref = await resetLink.getAttribute("href");
  await resetLink.click();

  await expect(page.getByRole("heading", { name: "Новий пароль" })).toBeVisible();
  await page.locator("#reset-password").fill("NewSecurePass2026");
  await page.locator("#reset-confirmation").fill("MismatchPass2026");
  await page.getByRole("button", { name: "Зберегти пароль" }).click();
  await expect(page.getByText("Паролі не збігаються.")).toBeVisible();

  await page.locator("#reset-confirmation").fill("NewSecurePass2026");
  await page.getByRole("button", { name: "Зберегти пароль" }).click();
  await expect(page.getByRole("heading", { name: "Пароль змінено" })).toBeVisible();
  await expect(page).toHaveURL(`${BASE_URL}/reset-password`);

  // Old session in the other browser context must now be revoked.
  await existingPage.reload();
  await expect(existingPage.getByRole("heading", { name: "Увійди, щоб продовжити" })).toBeVisible();

  // Reusing the reset token or opening a malformed token shows invalid link state.
  await page.goto(resetHref);
  await page.locator("#reset-password").fill("AnotherValidPass2026");
  await page.locator("#reset-confirmation").fill("AnotherValidPass2026");
  await page.getByRole("button", { name: "Зберегти пароль" }).click();
  await expect(page.getByRole("heading", { name: "Посилання недійсне" })).toBeVisible();

  await page.goto("/reset-password?token=malformed-short");
  await expect(page.getByRole("heading", { name: "Посилання недійсне" })).toBeVisible();

  // Sign in with the new password succeeds and clears lockout.
  await page.goto("/login");
  await page.getByLabel("Університетська пошта").fill(user.email);
  await page.getByLabel("Пароль", { exact: true }).fill("NewSecurePass2026");
  await page.getByRole("button", { name: "Увійти", exact: true }).click();
  await expect(page).toHaveURL(`${BASE_URL}/main`);

  await existingCtx.close();
  await recoveryCtx.close();
});

test("4-step onboarding saves profile, teaching/learning skills, and format, then redirects to main", async ({
  page,
}) => {
  const world = createMvpWorld();
  const student = world.seedUser({
    email: "daryna@students.example.test",
    firstName: "Дарина",
    lastName: "Ткаченко",
  });
  await world.attach(page);
  await world.signIn(page, student.email);

  await page.goto("/onboarding");
  await expect(page.getByRole("heading", { name: "Розкажи про себе" })).toBeVisible();

  await page.locator('input[name="firstName"]').fill("");
  await page.getByRole("button", { name: /Далі — додати навички/ }).click();
  await expect(page.getByText("Введи ім’я довжиною до 100 символів.")).toBeVisible();

  await page.locator('input[name="firstName"]').fill("Дарина");
  await page.locator('select[name="facultyId"]').selectOption(IDS.faculties.fiot);
  await page.locator('select[name="course"]').selectOption("3");
  await page
    .locator('textarea[name="bio"]')
    .fill("Граю на гітарі та цікавлюся графічним дизайном.");

  world.addFault({ method: "GET", path: "skills", status: 503, code: "unavailable" });
  await page.getByRole("button", { name: /Далі — додати навички/ }).click();
  await expect(page.getByRole("heading", { name: "Чого ти можеш навчити?" })).toBeVisible();
  await expect(page.getByText("Не вдалося завантажити навички.")).toBeVisible();
  await page.getByRole("button", { name: "Спробувати ще раз" }).click();
  await expect(page.getByRole("button", { name: "Гітара" })).toBeVisible();

  await page.getByRole("button", { name: /Далі — що хочу вивчити/ }).click();
  await expect(page.locator(".onboarding-notice")).toContainText("Обери хоча б одну навичку.");

  await page.getByRole("button", { name: "Гітара" }).click();
  await page.getByLabel("Мій рівень для Гітара").selectOption("ADVANCED");
  await page.getByRole("button", { name: /Далі — що хочу вивчити/ }).click();

  await expect(page.getByRole("heading", { name: "Чого ти хочеш навчитися?" })).toBeVisible();
  await page.getByRole("button", { name: "Photoshop" }).click();
  await page.getByRole("button", { name: "Figma" }).click();
  await page.getByRole("button", { name: "Підняти Figma" }).click();
  await page.getByRole("button", { name: /Далі — обрати формат/ }).click();

  await expect(page.getByRole("heading", { name: "Як тобі зручно навчатися?" })).toBeVisible();
  await page.getByRole("button", { name: "Офлайн" }).click();
  await page.getByRole("button", { name: /Завершити/ }).click();
  await expect(page.getByText(/Для офлайн-занять місто обов’язкове/)).toBeVisible();

  await page.getByRole("button", { name: "Назад" }).click();
  await page.getByRole("button", { name: "Назад" }).click();
  await page.getByRole("button", { name: "Назад" }).click();
  await page.locator('input[name="city"]').fill("Київ");
  await page.getByRole("button", { name: /Далі — додати навички/ }).click();
  await page.getByRole("button", { name: /Далі — що хочу вивчити/ }).click();
  await page.getByRole("button", { name: /Далі — обрати формат/ }).click();
  await page.getByRole("button", { name: "Обидва формати" }).click();
  await page.getByRole("button", { name: /Завершити/ }).click();

  await expect(page).toHaveURL(`${BASE_URL}/main`);
  await expect(page.getByRole("heading", { name: "Привіт, Дарина!" })).toBeVisible();

  // Visiting /onboarding again when already eligible redirects straight to /main.
  await page.goto("/onboarding");
  await expect(page).toHaveURL(`${BASE_URL}/main`);
});

test("profile editor validates fields, guards unsaved changes, persists edits, and manages dual skill lists", async ({
  page,
}) => {
  const world = createMvpWorld();
  const student = world.seedUser({
    email: "maksym@students.example.test",
    firstName: "Максим",
    lastName: "Кравець",
    universityId: IDS.universities.kpi,
    facultyId: IDS.faculties.fiot,
    course: 2,
    city: "Київ",
    bio: "Початковий опис",
    formats: ["ONLINE"],
    teachingSkills: [{ skillId: IDS.skills.go, level: "INTERMEDIATE" }],
    learningSkills: [{ skillId: IDS.skills.english, level: "BEGINNER" }],
  });
  await world.attach(page);
  await world.signIn(page, student.email);

  await page.goto("/profile");
  await expect(page.locator('input[name="firstName"]')).toHaveValue("Максим");

  // Unsaved-changes guard cancels navigation when dismissed, and Cancel button resets draft.
  await page.locator('input[name="firstName"]').fill("Максимко");
  await expect(page.getByText("Є незбережені зміни")).toBeVisible();

  let dialogMessage = "";
  page.once("dialog", async (dialog) => {
    dialogMessage = dialog.message();
    await dialog.dismiss();
  });
  await page.getByRole("link", { name: "Пошук" }).click();
  expect(dialogMessage).toContain("Є незбережені зміни");
  await expect(page).toHaveURL(`${BASE_URL}/profile`);

  await page.getByRole("button", { name: "Скасувати" }).click();
  await expect(page.locator('input[name="firstName"]')).toHaveValue("Максим");

  // Format and offline-city validation + 600-character bio truncation.
  await page.getByRole("checkbox", { name: "Онлайн" }).uncheck();
  await page.getByRole("button", { name: "Зберегти зміни" }).click();
  await expect(page.getByText("Обери онлайн- та/або офлайн-формат.")).toBeVisible();

  await page.getByRole("checkbox", { name: "Офлайн" }).check();
  await page.locator('input[name="city"]').fill("");
  await page.getByRole("button", { name: "Зберегти зміни" }).click();
  await expect(page.getByText(/Для офлайн-занять місто обов’язкове/)).toBeVisible();

  // University switch resets faculty and loads faculties of the selected university.
  await page.locator('select[name="universityId"]').selectOption(IDS.universities.knu);
  await expect(page.locator('select[name="facultyId"]')).toHaveValue("");
  await expect(page.locator('select[name="facultyId"] option', { hasText: "ФКНК" })).toHaveCount(1);
  await page.locator('select[name="facultyId"]').selectOption(IDS.faculties.fcsc);
  await page.locator('select[name="course"]').selectOption("4");
  await page.locator('input[name="city"]').fill("Львів");
  await page.getByRole("checkbox", { name: "Онлайн" }).check();
  await page.locator('textarea[name="bio"]').fill("О".repeat(620));
  await expect(page.getByText("600 / 600 символів")).toBeVisible();

  await page.getByRole("button", { name: "Зберегти зміни" }).click();
  await expect(page.getByText("Зміни збережено.")).toBeVisible();

  // Reload and verify all saved profile fields persist.
  await page.reload();
  await expect(page.locator('select[name="universityId"]')).toHaveValue(IDS.universities.knu);
  await expect(page.locator('select[name="facultyId"]')).toHaveValue(IDS.faculties.fcsc);
  await expect(page.locator('select[name="course"]')).toHaveValue("4");
  await expect(page.locator('input[name="city"]')).toHaveValue("Львів");

  // Manage teaching & learning skills on /profile?tab=skills.
  await page.getByRole("link", { name: /Мої навички/ }).click();
  await expect(page).toHaveURL(`${BASE_URL}/profile?tab=skills`);

  // Already added skill (Go) is filtered out from the teaching catalog picker (no duplicates).
  await page.getByRole("button", { name: "+ Додати навичку, якої я можу навчити" }).click();
  const addForm = page.getByRole("form", { name: "Додати навичку" });
  const catalogSelect = addForm.getByLabel("Навичка зі спільного каталогу");
  await expect(catalogSelect.locator("option", { hasText: "Python" })).toHaveCount(1);
  await expect(catalogSelect.locator("option", { hasText: "Go" })).toHaveCount(0);

  await catalogSelect.selectOption(IDS.skills.python);
  await addForm.getByLabel("Мій поточний рівень").selectOption("ADVANCED");
  await addForm.getByRole("button", { name: "Додати", exact: true }).click();
  await expect(page.getByText("Навичку збережено.")).toBeVisible();

  await page.getByLabel("Рівень Go: навчаю").selectOption("ADVANCED");
  await expect(page.getByText("Навичку збережено.")).toBeVisible();

  await page.getByRole("button", { name: "Видалити Python зі списку навчаю" }).click();
  await expect(page.getByText("Навичку видалено.")).toBeVisible();
});
