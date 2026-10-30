import { expect, test } from "@playwright/test";
import { BASE_URL, IDS, createMvpWorld } from "./mvp-fixture.mjs";

test("search filters by offered skill, syncs URL and pagination, and shows clear empty states", async ({
  page,
}) => {
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
  world.seedUser({
    email: "andrii@students.example.test",
    firstName: "Андрій",
    lastName: "Шевченко",
    city: "Київ",
    formats: ["ONLINE"],
    teachingSkills: [{ skillId: IDS.skills.photoshop, level: "ADVANCED" }],
    learningSkills: [{ skillId: IDS.skills.guitar, level: "BEGINNER" }],
  });
  world.seedUser({
    email: "marko@students.example.test",
    firstName: "Марко",
    lastName: "Сидоренко",
    city: "Київ",
    formats: ["OFFLINE"],
    teachingSkills: [{ skillId: IDS.skills.photoshop, level: "INTERMEDIATE" }],
    learningSkills: [{ skillId: IDS.skills.go, level: "BEGINNER" }],
  });
  // Only wants Photoshop, does NOT teach it -> must NOT appear when searching Photoshop.
  world.seedUser({
    email: "learner@students.example.test",
    firstName: "Софія",
    lastName: "ВивчаєФотошоп",
    city: "Київ",
    formats: ["ONLINE"],
    teachingSkills: [{ skillId: IDS.skills.english, level: "ADVANCED" }],
    learningSkills: [{ skillId: IDS.skills.photoshop, level: "BEGINNER" }],
  });
  for (let i = 1; i <= 21; i++) {
    world.seedUser({
      email: `extra${i}@students.example.test`,
      firstName: `Студент${i}`,
      lastName: "Додатковий",
      city: "Київ",
      formats: ["ONLINE"],
      teachingSkills: [{ skillId: IDS.skills.go, level: "BEGINNER" }],
      learningSkills: [{ skillId: IDS.skills.english, level: "BEGINNER" }],
    });
  }

  await world.attach(page);
  await world.signIn(page, olha.email);

  // Global search from topbar navigates to /search?q=Photoshop.
  await page.goto("/main");
  const topSearch = page.getByLabel("Пошук студентів за навичкою або ім’ям");
  await topSearch.fill("Photoshop");
  await topSearch.press("Enter");
  await expect(page).toHaveURL(`${BASE_URL}/search?q=Photoshop`);
  await expect(page.getByText("Знайдено 2 студенти за запитом «Photoshop»")).toBeVisible();
  await expect(page.getByText("Андрій Шевченко")).toBeVisible();
  await expect(page.getByText("Марко Сидоренко")).toBeVisible();
  await expect(page.getByText("Софія ВивчаєФотошоп")).toHaveCount(0);

  // Combined filters update URL, persist on reload, and support Back navigation.
  await page.getByLabel("Категорія").selectOption(IDS.categories.design);
  await page.getByLabel("Рівень навички").selectOption("ADVANCED");
  await page.getByLabel("Формат занять").selectOption("ONLINE");
  await page.getByRole("checkbox", { name: "Лише взаємні збіги" }).check();
  await expect(page).toHaveURL(
    new RegExp(
      `q=Photoshop.*categoryId=${IDS.categories.design}.*level=ADVANCED.*format=ONLINE.*mutual=true`,
    ),
  );
  await expect(page.getByText("Знайдено 1 студент за запитом «Photoshop»")).toBeVisible();
  await expect(page.getByText("Андрій Шевченко")).toBeVisible();
  await expect(page.getByText("Марко Сидоренко")).toHaveCount(0);

  await page.reload();
  await expect(page.getByText("Андрій Шевченко")).toBeVisible();

  // Reset filters shows paginated list of 24 discoverable students (20 on page 1, 4 on page 2).
  await page.getByRole("button", { name: "Скинути фільтри" }).click();
  await expect(page).toHaveURL(`${BASE_URL}/search`);
  await expect(page.getByText("Знайдено 24 студенти")).toBeVisible();
  await expect(page.locator(".discovery-grid > li")).toHaveCount(20);

  await page.getByRole("link", { name: "Наступна" }).click();
  await expect(page).toHaveURL(`${BASE_URL}/search?page=2`);
  await expect(page.locator(".discovery-grid > li")).toHaveCount(4);

  // Out-of-range page offers return link to page 1.
  await page.goto("/search?page=9");
  await expect(page.getByText("На цій сторінці результатів немає.")).toBeVisible();
  await page.getByRole("link", { name: "До першої сторінки" }).click();
  await expect(page).toHaveURL(`${BASE_URL}/search`);

  // Empty search state with reset link.
  await page.getByLabel("Навичка або ім’я").fill("Квантова фізика");
  await page.getByRole("button", { name: "Знайти" }).click();
  await expect(page.getByText("За запитом «Квантова фізика» нікого не знайдено.")).toBeVisible();
  await page.getByRole("link", { name: "Скинути фільтри" }).click();
  await expect(page).toHaveURL(`${BASE_URL}/search`);
});

test("mutual matches explain reciprocity without percentages, respect city/one-sided rules, and update dynamically", async ({
  page,
}) => {
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
    formats: ["ONLINE"],
    teachingSkills: [{ skillId: IDS.skills.photoshop, level: "ADVANCED" }],
    learningSkills: [{ skillId: IDS.skills.guitar, level: "BEGINNER" }],
  });
  const marko = world.seedUser({
    email: "marko@students.example.test",
    firstName: "Марко",
    lastName: "Сидоренко",
    city: "Київ",
    formats: ["ONLINE"],
    teachingSkills: [{ skillId: IDS.skills.photoshop, level: "INTERMEDIATE" }],
    learningSkills: [{ skillId: IDS.skills.go, level: "BEGINNER" }],
  });
  // Offline-only student in Lviv cannot match Olha in Kyiv.
  world.seedUser({
    email: "lviv@students.example.test",
    firstName: "Оксана",
    lastName: "Львівська",
    city: "Львів",
    formats: ["OFFLINE"],
    teachingSkills: [{ skillId: IDS.skills.photoshop, level: "ADVANCED" }],
    learningSkills: [{ skillId: IDS.skills.guitar, level: "BEGINNER" }],
  });
  const hidden = world.seedUser({
    email: "hidden@students.example.test",
    firstName: "Прихований",
    lastName: "Профіль",
    city: "Київ",
    formats: ["ONLINE"],
    teachingSkills: [{ skillId: IDS.skills.photoshop, level: "ADVANCED" }],
    learningSkills: [{ skillId: IDS.skills.guitar, level: "BEGINNER" }],
    privacy: { isDiscoverable: false },
  });

  await world.attach(page);
  await world.signIn(page, olha.email);

  await page.goto("/matches");
  await expect(page.getByText("Знайдено 1 студент")).toBeVisible();
  await expect(page.getByText("Андрій Шевченко")).toBeVisible();
  await expect(page.getByText("Марко Сидоренко")).toHaveCount(0);
  await expect(page.getByText("Оксана Львівська")).toHaveCount(0);

  // Verify no artificial percentage score is shown on mutual match cards.
  const mainText = await page.locator("#matches-content").innerText();
  expect(mainText).not.toMatch(/\d+\s*%/);

  // Candidate profile for mutual match shows explanation and "Запропонувати обмін", without edit inputs.
  await page.goto(`/students/${andrii.id}`);
  await expect(page.getByRole("heading", { name: "Чому це взаємний збіг" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Запропонувати обмін" })).toBeVisible();
  await expect(page.locator('form[aria-label="Редагування профілю"]')).toHaveCount(0);

  // Candidate profile for one-sided student explains why it is not mutual and hides proposal button.
  await page.goto(`/students/${marko.id}`);
  await expect(page.getByRole("heading", { name: "Чому це не взаємний збіг" })).toBeVisible();
  await expect(
    page.getByText("у списку «Хоче вивчити» немає навичок, яких навчаєш ти"),
  ).toBeVisible();
  await expect(page.getByRole("button", { name: "Запропонувати обмін" })).toHaveCount(0);

  // Hidden profile and own ID return unavailable state.
  await page.goto(`/students/${hidden.id}`);
  await expect(page.getByText("Профіль недоступний.")).toBeVisible();
  await page.goto(`/students/${olha.id}`);
  await expect(page.getByText("Профіль недоступний.")).toBeVisible();

  // Adding Go to Olha's teaching skills immediately turns Marko into a mutual match.
  await page.goto("/profile?tab=skills");
  await page.getByRole("button", { name: "+ Додати навичку, якої я можу навчити" }).click();
  const addForm = page.getByRole("form", { name: "Додати навичку" });
  await addForm.getByLabel("Навичка зі спільного каталогу").selectOption(IDS.skills.go);
  await addForm.getByRole("button", { name: "Додати", exact: true }).click();
  await expect(page.getByText("Навичку збережено.")).toBeVisible();

  await page.goto("/matches");
  await expect(page.getByText("Знайдено 2 студенти")).toBeVisible();
  await expect(page.getByText("Марко Сидоренко")).toBeVisible();
});

test("exchange proposal enforces equal teaching time, privacy restrictions, and duplicate guard", async ({
  page,
}) => {
  const world = createMvpWorld();
  const olha = world.seedUser({
    email: "olha@students.example.test",
    firstName: "Ольга",
    lastName: "Коваленко",
    universityId: IDS.universities.kpi,
    city: "Київ",
    formats: ["ONLINE"],
    teachingSkills: [{ skillId: IDS.skills.guitar, level: "ADVANCED" }],
    learningSkills: [{ skillId: IDS.skills.photoshop, level: "BEGINNER" }],
  });
  const andrii = world.seedUser({
    email: "andrii@knu.example.test",
    firstName: "Андрій",
    lastName: "Шевченко",
    universityId: IDS.universities.knu,
    city: "Київ",
    formats: ["ONLINE"],
    teachingSkills: [{ skillId: IDS.skills.photoshop, level: "ADVANCED" }],
    learningSkills: [{ skillId: IDS.skills.guitar, level: "BEGINNER" }],
    privacy: { acceptsRequests: false, requestScope: "SAME_UNIVERSITY" },
  });

  await world.attach(page);
  await world.signIn(page, olha.email);

  await page.goto(`/students/${andrii.id}`);
  await page.getByRole("button", { name: "Запропонувати обмін" }).click();
  const dialog = page.getByRole("dialog");

  // Unequal total teaching time is rejected before submission.
  await dialog.getByRole("checkbox", { name: "Однакові умови в обидва боки" }).uncheck();
  await dialog.getByLabel("Андрій проводить: Занять").selectOption("3");
  await expect(
    dialog.getByText("Ти навчаєш 2 год, а Андрій — 3 год. Сумарний час має збігатися."),
  ).toBeVisible();
  await dialog.getByRole("button", { name: "Надіслати запит" }).click();
  await expect(dialog.locator(".exchange-plan__summary--error")).toContainText(
    "Сумарний час має збігатися.",
  );
  expect(world.requests.size).toBe(0);

  // Equal total time with different lesson counts/durations (2 × 60 min = 4 × 30 min = 2 hours).
  await dialog.getByLabel("Андрій проводить: Занять").selectOption("4");
  await dialog.getByLabel("Андрій проводить: Тривалість").selectOption("30");
  await expect(
    dialog.getByText("Разом 6 занять: по 2 год навчання в кожному напрямку."),
  ).toBeVisible();

  // Message capped at 500 Unicode characters.
  await dialog.getByLabel(/Повідомлення/).fill("П".repeat(520));
  await expect(dialog.getByText("500 / 500 символів")).toBeVisible();

  // Closed requests check.
  await dialog.getByRole("button", { name: "Надіслати запит" }).click();
  await expect(dialog.getByText("Студент зараз не приймає нових запитів на обмін.")).toBeVisible();

  // Same university scope check.
  andrii.privacy.acceptsRequests = true;
  await dialog.getByRole("button", { name: "Надіслати запит" }).click();
  await expect(
    dialog.getByText("Студент приймає запити лише від студентів свого університету."),
  ).toBeVisible();

  // Open scope succeeds and creates the 6-lesson request.
  andrii.privacy.requestScope = "ANYONE";
  await dialog.getByRole("button", { name: "Надіслати запит" }).click();
  await expect(dialog.getByRole("heading", { name: "Запит надіслано" })).toBeVisible();
  await expect(dialog.getByText(/6 занять/)).toBeVisible();
  await dialog.getByRole("button", { name: "Закрити", exact: true }).click();

  // Attempting a second pending request for the same skill pair returns duplicate_request.
  await page.getByRole("button", { name: "Запропонувати обмін" }).click();
  await page.getByRole("dialog").getByRole("button", { name: "Надіслати запит" }).click();
  await expect(
    page.getByRole("dialog").getByText("Запит на цю пару навичок уже очікує відповіді."),
  ).toBeVisible();
});

test("requests support withdraw, decline, idempotent accept, agreed snapshot preservation, and stranger 404", async ({
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
  const stranger = world.seedUser({
    email: "stranger@students.example.test",
    firstName: "Олег",
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

  const sendRequest = async (messageText) => {
    await olhaPage.goto(`/students/${andrii.id}`);
    await olhaPage.getByRole("button", { name: "Запропонувати обмін" }).click();
    const dlg = olhaPage.getByRole("dialog");
    await dlg.getByLabel(/Повідомлення/).fill(messageText);
    await dlg.getByRole("button", { name: "Надіслати запит" }).click();
    await expect(dlg.getByRole("heading", { name: "Запит надіслано" })).toBeVisible();
    await dlg.getByRole("link", { name: "Переглянути запит" }).click();
    await expect(olhaPage).toHaveURL(/\/requests\/[0-9a-f-]+$/);
    return olhaPage.url().split("/").pop();
  };

  // 1) Withdraw flow by author.
  await sendRequest("Перша чернетка запиту");
  await olhaPage.getByRole("button", { name: "Відкликати", exact: true }).click();
  await olhaPage.getByRole("dialog").getByRole("button", { name: "Відкликати запит" }).click();
  await expect(olhaPage.getByText("Запит відкликано.")).toBeVisible();
  await expect(olhaPage.getByRole("button", { name: "Відкликати" })).toHaveCount(0);

  // 2) Decline flow by recipient + history & stranger 404 check.
  const declinedId = await sendRequest("Друга спроба запиту");
  await strangerPage.goto(`/requests/${declinedId}`);
  await expect(strangerPage.getByText("Запит не знайдено.")).toBeVisible();

  await andriiPage.goto(`/requests/${declinedId}`);
  await andriiPage.getByRole("button", { name: "Відхилити", exact: true }).click();
  await andriiPage.getByRole("dialog").getByRole("button", { name: "Відхилити запит" }).click();
  await expect(andriiPage.getByText("Запит відхилено.")).toBeVisible();
  await expect(andriiPage.getByRole("button", { name: "Прийняти" })).toHaveCount(0);
  await expect(andriiPage.locator(".exchange-history")).toContainText("Запит відхилено");

  // 3) Accept flow creates active exchange and preserves agreed snapshot skills even after profile skill deletion.
  const acceptedId = await sendRequest("Готова до обміну навичками!");
  await andriiPage.goto(`/requests/${acceptedId}`);
  await andriiPage.getByRole("button", { name: "Прийняти", exact: true }).click();
  await expect(andriiPage).toHaveURL(/\/exchanges\/[0-9a-f-]+\?started=1$/);
  expect(world.exchanges.size).toBe(1);
  const exchangeId = [...world.exchanges.keys()][0];

  // Stranger gets not-found on the active exchange as well.
  await strangerPage.goto(`/exchanges/${exchangeId}`);
  await expect(strangerPage.getByText("Обмін не знайдено.")).toBeVisible();

  // Olha deletes Guitar from her profile skills, yet the active exchange still shows Guitar.
  await olhaPage.goto("/profile?tab=skills");
  await olhaPage.getByRole("button", { name: "Видалити Гітара зі списку навчаю" }).click();
  await expect(olhaPage.getByText("Навичку видалено.")).toBeVisible();

  await olhaPage.goto(`/exchanges/${exchangeId}`);
  await expect(olhaPage.locator("#exchange-content")).toContainText("Гітара");
  await expect(olhaPage.locator("#exchange-content")).toContainText("Photoshop");

  await olhaCtx.close();
  await andriiCtx.close();
  await strangerCtx.close();
});
