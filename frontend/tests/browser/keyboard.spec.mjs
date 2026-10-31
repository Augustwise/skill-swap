import { expect, test } from "../helpers/browser-version.mjs";
import { BASE_URL, DEMO_PASSWORD, IDS, MAILPIT_URL, createMvpWorld } from "./mvp-fixture.mjs";

import { activate, assertLayout, tabTo, type } from "../helpers/keyboard.mjs";

for (const width of [360, 768, 1440]) {
  test(`registration, onboarding, search and an exchange request work using only the keyboard at ${width}px`, async ({
    page,
  }) => {
    test.setTimeout(90_000);
    await page.setViewportSize({ width, height: 900 });
    const world = createMvpWorld();
    world.seedUser({
      email: "keyboard.partner@students.example.test",
      firstName: "Андрій",
      lastName: "Партнер",
      city: "Київ",
      formats: ["ONLINE"],
      teachingSkills: [{ skillId: IDS.skills.photoshop, level: "ADVANCED" }],
      learningSkills: [{ skillId: IDS.skills.guitar, level: "BEGINNER" }],
    });
    await world.attach(page);
    await page.goto("/register");
    await activate(page, page.getByRole("button", { name: "Створити акаунт" }));
    await expect(page.locator('input[name="firstName"]')).toBeFocused();
    await expect(page.locator('input[name="firstName"]')).toHaveAttribute("aria-invalid", "true");
    await expect(page.getByText("Введи ім’я.")).toBeVisible();
    await type(page, page.locator('input[name="firstName"]'), "Ольга");
    await type(page, page.locator('input[name="lastName"]'), "Клавіатурна");
    const email = `keyboard.${width}@students.example.test`;
    await type(page, page.getByLabel("Університетська пошта"), email);
    await tabTo(page, page.locator('select[name="university"]'));
    await page.keyboard.press("Home");
    await page.keyboard.press("Enter");
    await expect(page.locator('select[name="university"]')).toHaveValue(IDS.universities.kpi);
    await type(page, page.getByLabel("Пароль", { exact: true }), DEMO_PASSWORD);
    await tabTo(page, page.locator("#register-agreement"));
    await page.keyboard.press("Space");
    await assertLayout(page);
    await activate(page, page.getByRole("button", { name: "Створити акаунт" }));
    await expect(page.getByRole("heading", { name: "Акаунт створено" })).toBeVisible();

    await page.goto(`${MAILPIT_URL}/?to=${email}`);
    await activate(page, page.getByRole("link", { name: `Відкрити посилання для ${email}` }));
    await expect(page.getByRole("heading", { name: "Пошту підтверджено" })).toBeVisible();
    await activate(page, page.getByRole("link", { name: "Перейти до входу" }));
    await type(page, page.getByLabel("Університетська пошта"), email);
    await type(page, page.getByLabel("Пароль", { exact: true }), DEMO_PASSWORD);
    await activate(page, page.getByRole("button", { name: "Увійти", exact: true }));
    await expect(page).toHaveURL(`${BASE_URL}/onboarding`);
    await type(page, page.locator('input[name="city"]'), "Київ");
    await assertLayout(page);
    await activate(page, page.getByRole("button", { name: "Далі — додати навички" }));
    await activate(page, page.getByRole("button", { name: "Гітара", exact: true }));
    await activate(page, page.getByRole("button", { name: "Далі — що хочу вивчити" }));
    await activate(page, page.getByRole("button", { name: "Photoshop", exact: true }));
    await activate(page, page.getByRole("button", { name: "Далі — обрати формат" }));
    await activate(page, page.getByRole("button", { name: "Онлайн", exact: true }));
    await activate(page, page.getByRole("button", { name: "Завершити", exact: true }));
    await expect(page.getByRole("heading", { name: "Привіт, Ольга!" })).toBeVisible();
    await assertLayout(page);
    await activate(page, page.getByRole("link", { name: "До вмісту", exact: true }));
    await expect(page.locator("main")).toBeFocused();
    await activate(page, page.getByRole("link", { name: "Знайти партнера", exact: true }));
    await type(page, page.getByLabel("Навичка або ім’я", { exact: true }), "Photoshop");
    await page.keyboard.press("Enter");
    await expect(page.getByRole("heading", { name: "Андрій Партнер" })).toBeVisible();
    await assertLayout(page);
    await activate(page, page.getByRole("link", { name: "Переглянути профіль: Андрій Партнер" }));
    await activate(page, page.getByRole("button", { name: "Запропонувати обмін" }));
    const dialog = page.getByRole("dialog");
    await expect(dialog).toBeVisible();
    expect(await dialog.evaluate((element) => element.contains(document.activeElement))).toBe(true);
    await assertLayout(page);
    await page.keyboard.press("Escape");
    await expect(dialog).toHaveCount(0);
    await expect(page.getByRole("button", { name: "Запропонувати обмін" })).toBeFocused();
    await page.keyboard.press("Enter");
    await type(page, dialog.getByLabel(/Повідомлення/), "Обмін, створений лише клавіатурою");
    await activate(page, dialog.getByRole("button", { name: "Надіслати запит" }));
    await expect(dialog.getByRole("heading", { name: "Запит надіслано" })).toBeVisible();
    await activate(page, dialog.getByRole("link", { name: "Переглянути запит" }));
    await expect(page).toHaveURL(/\/requests\/[0-9a-f-]+$/);
    await expect(
      page.getByText("Обмін, створений лише клавіатурою", { exact: true }),
    ).toBeVisible();
    await assertLayout(page);
    expect(world.requests.size).toBe(1);
  });
}
