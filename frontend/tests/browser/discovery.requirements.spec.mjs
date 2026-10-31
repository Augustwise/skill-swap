import { expect, test } from "@playwright/test";
import { BASE_URL, IDS, createMvpWorld } from "./mvp-fixture.mjs";

function seedViewer(world) {
  return world.seedUser({
    email: "viewer@students.example.test",
    firstName: "Ольга",
    lastName: "Коваленко",
    city: "Київ",
    formats: ["ONLINE", "OFFLINE"],
    teachingSkills: [{ skillId: IDS.skills.guitar, level: "ADVANCED" }],
    learningSkills: [{ skillId: IDS.skills.photoshop, level: "BEGINNER" }],
  });
}

function seedPartner(world, overrides = {}) {
  return world.seedUser({
    email: "partner@students.example.test",
    firstName: "Андрій",
    lastName: "Шевченко",
    city: "Київ",
    formats: ["ONLINE"],
    bio: "Пояснюю Photoshop на практичних прикладах.",
    teachingSkills: [{ skillId: IDS.skills.photoshop, level: "ADVANCED" }],
    learningSkills: [{ skillId: IDS.skills.guitar, level: "BEGINNER" }],
    ...overrides,
  });
}

function searchRequests(world) {
  return world.requestLog.filter(
    (request) => request.path === "students" && request.method === "GET",
  );
}

test.describe("FR-04 — search acceptance gaps", () => {
  test("name search trims the query and does not confuse desired skills with offered skills", async ({
    page,
  }) => {
    const world = createMvpWorld();
    const viewer = seedViewer(world);
    seedPartner(world);
    seedPartner(world, {
      email: "learner@students.example.test",
      firstName: "Софія",
      lastName: "Мельник",
      teachingSkills: [{ skillId: IDS.skills.figma, level: "ADVANCED" }],
      learningSkills: [{ skillId: IDS.skills.photoshop, level: "BEGINNER" }],
    });
    await world.attach(page);
    await world.signIn(page, viewer.email);
    await page.goto("/search");
    await page.getByLabel("Навичка або ім’я").fill("   шЕвЧеНкО   ");
    await page.getByRole("button", { name: "Знайти", exact: true }).click();
    await expect(page.getByRole("heading", { name: "Андрій Шевченко", exact: true })).toBeVisible();
    await expect(page.locator(".discovery-grid > li")).toHaveCount(1);
    expect(new URL(page.url()).searchParams.get("q")).toBe("шЕвЧеНкО");
    expect(new URLSearchParams(searchRequests(world).at(-1).search).get("q")).toBe("шЕвЧеНкО");

    await page.getByLabel("Навичка або ім’я").fill("Photoshop");
    await page.getByRole("button", { name: "Знайти", exact: true }).click();
    await expect(page.getByRole("heading", { name: "Андрій Шевченко", exact: true })).toBeVisible();
    await expect(page.getByRole("heading", { name: "Софія Мельник", exact: true })).toHaveCount(0);
    await expect(page.locator(".discovery-grid > li")).toHaveCount(1);
  });

  test("filtered pagination preserves every filter and returns from a candidate to the same page", async ({
    page,
  }) => {
    const world = createMvpWorld();
    const viewer = seedViewer(world);
    for (let i = 1; i <= 21; i++) {
      seedPartner(world, { email: `partner${i}@students.example.test`, firstName: `Студент${i}` });
    }
    seedPartner(world, {
      email: "wrong-level@students.example.test",
      firstName: "ІншийРівень",
      teachingSkills: [{ skillId: IDS.skills.photoshop, level: "BEGINNER" }],
    });
    const query = `q=Photoshop&categoryId=${IDS.categories.design}&level=ADVANCED&format=ONLINE&mutual=true`;
    await world.attach(page);
    await world.signIn(page, viewer.email);
    await page.goto(`/search?${query}`);
    const cards = page.locator(".discovery-grid > li");
    await expect(cards).toHaveCount(20);
    const firstPageNames = await cards.locator("h3").allTextContents();
    await expect(page.getByRole("heading", { name: "ІншийРівень Шевченко" })).toHaveCount(0);
    await page.getByRole("link", { name: "Наступна" }).click();
    await expect(cards).toHaveCount(1);
    await expect(page).toHaveURL(`${BASE_URL}/search?${query}&page=2`);
    const secondPageName = await cards.locator("h3").innerText();
    expect(firstPageNames).not.toContain(secondPageName);
    expect(new Set([...firstPageNames, secondPageName]).size).toBe(21);
    expect(Object.fromEntries(new URLSearchParams(searchRequests(world).at(-1).search))).toEqual({
      q: "Photoshop",
      categoryId: IDS.categories.design,
      level: "ADVANCED",
      format: "ONLINE",
      mutual: "true",
      page: "2",
    });

    await cards.getByRole("link", { name: /Переглянути профіль:/ }).click();
    await expect(page.getByRole("heading", { name: secondPageName, exact: true })).toBeVisible();
    await page.getByRole("link", { name: "До результатів пошуку" }).click();
    await expect(page).toHaveURL(`${BASE_URL}/search?${query}&page=2`);
    await expect(cards).toHaveCount(1);
    await expect(page.getByLabel("Категорія")).toHaveValue(IDS.categories.design);
    await expect(page.getByLabel("Рівень навички")).toHaveValue("ADVANCED");
    await expect(page.getByLabel("Формат занять")).toHaveValue("ONLINE");
    await expect(page.getByRole("checkbox", { name: "Лише взаємні збіги" })).toBeChecked();
  });

  test("changing a filter from page two starts at page one and distinguishes an empty result from a failed request", async ({
    page,
  }) => {
    const world = createMvpWorld();
    const viewer = seedViewer(world);
    for (let i = 1; i <= 21; i++) {
      seedPartner(world, { email: `partner${i}@students.example.test`, firstName: `Студент${i}` });
    }
    await world.attach(page);
    await world.signIn(page, viewer.email);
    await page.goto("/search?q=Photoshop&page=2");
    await expect(page.locator(".discovery-grid > li")).toHaveCount(1);
    world.addFault({ method: "GET", path: "students", status: 503 });
    await page.getByLabel("Рівень навички").selectOption("BEGINNER");
    await expect(page).toHaveURL(`${BASE_URL}/search?q=Photoshop&level=BEGINNER`);
    await expect(page.locator(".discovery-alert")).toBeVisible();
    await expect(page.getByText(/нікого не знайдено/)).toHaveCount(0);
    await page.getByRole("button", { name: "Спробувати ще раз" }).click();
    await expect(page.getByText("За запитом «Photoshop» нікого не знайдено.")).toBeVisible();
    await expect(page.locator(".discovery-alert")).toHaveCount(0);
    await expect(page.getByLabel("Рівень навички")).toHaveValue("BEGINNER");
    await expect(page.getByLabel("Навичка або ім’я")).toHaveValue("Photoshop");
    await page.getByRole("link", { name: "Скинути фільтри", exact: true }).click();
    await expect(page).toHaveURL(`${BASE_URL}/search`);
    await expect(page.locator(".discovery-grid > li")).toHaveCount(20);
  });
});

test.describe("FR-05 — mutual matches and candidate details", () => {
  for (const [label, formats, city, expected] of [
    ["online across different cities", ["ONLINE"], "Львів", true],
    ["offline in the same city", ["OFFLINE"], "Київ", true],
    ["offline in another city", ["OFFLINE"], "Львів", false],
    ["online and offline across cities", ["ONLINE", "OFFLINE"], "Львів", true],
  ]) {
    test(`common formats: ${label}`, async ({ page }) => {
      const world = createMvpWorld();
      const viewer = seedViewer(world);
      seedPartner(world, { formats, city });
      await world.attach(page);
      await world.signIn(page, viewer.email);
      await page.goto("/matches");
      if (expected) {
        await expect(page.locator(".discovery-grid > li")).toHaveCount(1);
        await expect(
          page.getByRole("heading", { name: "Андрій Шевченко", exact: true }),
        ).toBeVisible();
        await expect(page.locator(".discovery-card")).toContainText("Взаємний збіг");
        await expect(page.locator(".discovery-card")).toContainText(
          city === "Львів" ? "Онлайн" : "Офлайн",
        );
        if (city === "Львів")
          await expect(page.locator(".discovery-card .discovery-meta").last()).not.toContainText(
            "Офлайн",
          );
      } else {
        await expect(page.getByText("Поки немає взаємних збігів.")).toBeVisible();
        await expect(page.locator(".discovery-grid > li")).toHaveCount(0);
        await page.getByRole("link", { name: "Перейти до пошуку", exact: true }).click();
        await expect(
          page.getByRole("heading", { name: "Андрій Шевченко", exact: true }),
        ).toBeVisible();
        await expect(page.locator(".discovery-card .dashboard-score")).toHaveCount(0);
      }
    });
  }

  test("candidate details render both levels, biography, formats and available reviews", async ({
    page,
  }) => {
    const world = createMvpWorld();
    const viewer = seedViewer(world);
    const partner = seedPartner(world);
    world.reviews.set("review-1", {
      id: "review-1",
      exchangeId: "exchange-1",
      authorId: viewer.id,
      targetUserId: partner.id,
      rating: 4,
      comment: "Доступно пояснює роботу з шарами.",
      createdAt: world.nowIso(),
    });
    await world.attach(page);
    await world.signIn(page, viewer.email);
    await page.goto("/matches");
    const card = page.locator(".discovery-card");
    await expect(
      card.getByRole("list", { name: "Навички, яких студент може навчити тебе" }),
    ).toContainText("Photoshop · Просунутий");
    await expect(
      card.getByRole("list", { name: "Навички, які студент хоче вивчити у тебе" }),
    ).toContainText("Гітара · Початковий");
    await card.getByRole("link", { name: /Переглянути профіль:/ }).click();
    await expect(page.getByRole("region", { name: "Про себе", exact: true })).toContainText(
      partner.bio,
    );
    const skills = page.getByRole("region", { name: "Навички", exact: true });
    await expect(skills).toContainText("Photoshop");
    await expect(skills).toContainText("Просунутий");
    await expect(skills).toContainText("Гітара");
    await expect(skills).toContainText("Початковий");
    await expect(page.getByRole("region", { name: "Формат занять", exact: true })).toContainText(
      "Онлайн",
    );
    const reviews = page.getByRole("region", { name: "Відгуки", exact: true });
    await expect(reviews).toContainText("4,0");
    await expect(reviews).toContainText("Доступно пояснює роботу з шарами.");
    await expect(reviews).toContainText("Ольга Коваленко");
    await expect(page.getByRole("heading", { name: "Чому це взаємний збіг" })).toBeVisible();
  });

  test("a partner's skill deletion and restoration update matches in another browser session", async ({
    browser,
  }) => {
    const world = createMvpWorld();
    const viewer = seedViewer(world);
    const partner = seedPartner(world);
    const viewerContext = await browser.newContext();
    const partnerContext = await browser.newContext();
    try {
      await world.attach(viewerContext);
      await world.attach(partnerContext);
      await world.signIn(viewerContext, viewer.email);
      await world.signIn(partnerContext, partner.email);
      const page = await viewerContext.newPage();
      const partnerPage = await partnerContext.newPage();
      await page.goto("/matches");
      await expect(
        page.getByRole("heading", { name: "Андрій Шевченко", exact: true }),
      ).toBeVisible();

      await partnerPage.goto("/profile?tab=skills");
      await partnerPage.getByRole("button", { name: "Видалити Гітара зі списку вивчаю" }).click();
      await expect(partnerPage.getByText("Навичку видалено.")).toBeVisible();
      await page.reload();
      await expect(page.getByText("Поки немає взаємних збігів.")).toBeVisible();
      await expect(page.locator(".discovery-grid > li")).toHaveCount(0);

      await partnerPage.getByRole("button", { name: "+ Додати навичку для вивчення" }).click();
      const addForm = partnerPage.getByRole("form", { name: "Додати навичку" });
      await addForm.getByLabel("Навичка зі спільного каталогу").selectOption(IDS.skills.guitar);
      await addForm.getByRole("button", { name: "Додати", exact: true }).click();
      await expect(partnerPage.getByLabel("Рівень Гітара: вивчаю")).toBeVisible();
      await page.reload();
      await expect(
        page.getByRole("heading", { name: "Андрій Шевченко", exact: true }),
      ).toBeVisible();
      await expect(page.locator(".discovery-grid > li")).toHaveCount(1);
    } finally {
      await viewerContext.close();
      await partnerContext.close();
    }
  });

  test("a failed matches request offers retry instead of a misleading empty list", async ({
    page,
  }) => {
    const world = createMvpWorld();
    const viewer = seedViewer(world);
    seedPartner(world);
    world.addFault({ method: "GET", path: "me/matches", kind: "network" });
    await world.attach(page);
    await world.signIn(page, viewer.email);
    await page.goto("/matches");
    await expect(page.locator(".discovery-alert")).toContainText(
      "Не вдалося зв’язатися із сервером",
    );
    await expect(page.getByText("Поки немає взаємних збігів.")).toHaveCount(0);
    await page.getByRole("button", { name: "Спробувати ще раз" }).click();
    await expect(page.getByRole("heading", { name: "Андрій Шевченко", exact: true })).toBeVisible();
    await expect(page.locator(".discovery-alert")).toHaveCount(0);
  });
});
