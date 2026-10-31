import { expect, test } from "@playwright/test";
import { BASE_URL, DEMO_PASSWORD, IDS, createMvpWorld } from "./mvp-fixture.mjs";

function seedStudent(world, overrides = {}) {
  return world.seedUser({
    email: "student@students.example.test",
    firstName: "Ольга",
    lastName: "Коваленко",
    city: "Київ",
    formats: ["ONLINE"],
    teachingSkills: [{ skillId: IDS.skills.guitar, level: "ADVANCED" }],
    learningSkills: [{ skillId: IDS.skills.photoshop, level: "BEGINNER" }],
    ...overrides,
  });
}

async function login(page, email, password = DEMO_PASSWORD) {
  await page.goto("/login");
  await page.getByLabel("Університетська пошта").fill(email);
  await page.getByLabel("Пароль", { exact: true }).fill(password);
  await page.getByRole("button", { name: "Увійти", exact: true }).click();
}

function writes(world, path) {
  return world.requestLog.filter((request) => request.method !== "GET" && request.path === path);
}

test.describe("FR-01 — account acceptance gaps", () => {
  for (const kind of ["verify", "reset"]) {
    test(`expired ${kind} link shows an invalid state and cannot change the account`, async ({
      page,
    }) => {
      const world = createMvpWorld();
      const student = seedStudent(world, { emailVerified: false });
      const mail = world.issueMailToken(kind, student);
      const path = kind === "verify" ? "auth/verify-email" : "auth/reset-password";
      world.addFault({ method: "POST", path, status: 400, code: "invalid_token" });
      await world.attach(page);

      await page.goto(mail.url);
      if (kind === "reset") {
        await page.locator("#reset-password").fill("NewSecurePassword2026");
        await page.locator("#reset-confirmation").fill("NewSecurePassword2026");
        await page.getByRole("button", { name: "Зберегти пароль" }).click();
      }

      await expect(
        page.getByRole("heading", {
          name: kind === "verify" ? "Посилання не працює" : "Посилання недійсне",
        }),
      ).toBeVisible();
      await expect(page).toHaveURL(
        `${BASE_URL}/${kind === "verify" ? "verify-email" : "reset-password"}`,
      );
      expect(writes(world, path)).toHaveLength(1);
      expect(student.emailVerified).toBe(false);
      expect(student.password).toBe(DEMO_PASSWORD);
      await expect(page.getByRole("button", { name: "Зберегти пароль" })).toHaveCount(0);
    });
  }

  test("verification retries a temporary server failure with the original link", async ({
    page,
  }) => {
    const world = createMvpWorld();
    const student = seedStudent(world, { emailVerified: false });
    const mail = world.issueMailToken("verify", student);
    world.addFault({ method: "POST", path: "auth/verify-email", status: 503 });
    await world.attach(page);

    await page.goto(mail.url);
    await expect(page.getByRole("heading", { name: "Не вдалося перевірити пошту" })).toBeVisible();
    expect(student.emailVerified).toBe(false);
    await page.getByRole("button", { name: "Спробувати ще раз" }).click();
    await expect(page.getByRole("heading", { name: "Пошту підтверджено" })).toBeVisible();
    await expect(page).toHaveURL(`${BASE_URL}/verify-email`);
    expect(writes(world, "auth/verify-email").map((request) => request.body)).toEqual([
      { token: mail.token },
      { token: mail.token },
    ]);
  });

  test("an unverified session cannot reach any exchange creation entry point", async ({ page }) => {
    const world = createMvpWorld();
    const student = seedStudent(world, { emailVerified: false });
    const partner = seedStudent(world, { email: "partner@students.example.test" });
    await world.attach(page);
    await world.signIn(page, student.email);

    for (const path of [
      "/search",
      "/matches",
      `/students/${partner.id}`,
      "/requests?direction=outgoing",
    ]) {
      await page.goto(path);
      await expect(page).toHaveURL(`${BASE_URL}/verify-email`);
      await expect(page.getByRole("heading", { name: "Підтвердь свою пошту" })).toBeVisible();
      await expect(page.getByRole("button", { name: "Запропонувати обмін" })).toHaveCount(0);
    }
    expect(writes(world, "exchange-requests")).toEqual([]);
  });

  test("password recovery gives the same public response for known and unknown email addresses", async ({
    page,
  }) => {
    const world = createMvpWorld();
    const student = seedStudent(world);
    await world.attach(page);

    for (const email of [student.email, "unknown@students.example.test"]) {
      await page.goto("/login");
      await page.getByRole("button", { name: "Забув пароль?" }).click();
      await page.getByLabel("Університетська пошта").fill(email);
      await page.getByRole("button", { name: "Надіслати посилання" }).click();
      await expect(page.getByRole("heading", { name: "Перевір пошту" })).toBeVisible();
      await expect(
        page.getByText(`Якщо акаунт для ${email} існує, ми надіслали лист для відновлення пароля.`),
      ).toBeVisible();
      await expect(page.getByRole("main").getByRole("alert")).toHaveCount(0);
    }
    expect(world.mailbox.map((mail) => mail.to)).toEqual([student.email]);
  });
});

test.describe("FR-02 — profile acceptance gaps", () => {
  test("every saved field survives logout and a new login", async ({ page }) => {
    const world = createMvpWorld();
    const student = seedStudent(world, { facultyId: IDS.faculties.fiot });
    await world.attach(page);
    await world.signIn(page, student.email);
    await page.goto("/profile");

    const bio = "ї".repeat(600);
    await page.locator('input[name="firstName"]').fill("Марія");
    await page.locator('input[name="lastName"]').fill("Бондаренко");
    await page.locator('select[name="universityId"]').selectOption(IDS.universities.knu);
    await expect(page.locator('select[name="facultyId"]')).toHaveValue("");
    await page.locator('select[name="facultyId"]').selectOption(IDS.faculties.fcsc);
    await page.locator('select[name="course"]').selectOption("6");
    await page.locator('input[name="city"]').fill("Львів");
    await page.locator('textarea[name="bio"]').fill(bio);
    await page.getByRole("checkbox", { name: "Офлайн", exact: true }).check();
    await page.getByRole("button", { name: "Зберегти зміни" }).click();
    await expect(page.getByText("Зміни збережено.")).toBeVisible();
    await page.getByRole("button", { name: "Вийти", exact: true }).click();
    await expect(page).toHaveURL(`${BASE_URL}/login`);
    await login(page, student.email);
    await expect(page).toHaveURL(`${BASE_URL}/main`);
    await page.goto("/profile");

    for (const [selector, value] of [
      ['input[name="firstName"]', "Марія"],
      ['input[name="lastName"]', "Бондаренко"],
      ['select[name="universityId"]', IDS.universities.knu],
      ['select[name="facultyId"]', IDS.faculties.fcsc],
      ['select[name="course"]', "6"],
      ['input[name="city"]', "Львів"],
      ['textarea[name="bio"]', bio],
    ]) {
      await expect(page.locator(selector)).toHaveValue(value);
    }
    await expect(page.getByRole("checkbox", { name: "Онлайн", exact: true })).toBeChecked();
    await expect(page.getByRole("checkbox", { name: "Офлайн", exact: true })).toBeChecked();
    expect(writes(world, "me/profile")).toHaveLength(1);
    expect(writes(world, "me/profile")[0].body).toEqual({
      firstName: "Марія",
      lastName: "Бондаренко",
      universityId: IDS.universities.knu,
      facultyId: IDS.faculties.fcsc,
      course: 6,
      city: "Львів",
      bio,
      formats: ["ONLINE", "OFFLINE"],
    });
  });

  test("required names are explained locally and server field errors preserve the draft", async ({
    page,
  }) => {
    const world = createMvpWorld();
    const student = seedStudent(world);
    await world.attach(page);
    await world.signIn(page, student.email);
    await page.goto("/profile");
    const form = page.getByRole("form", { name: "Редагування профілю" });
    const firstName = form.locator('input[name="firstName"]');
    const lastName = form.locator('input[name="lastName"]');
    await firstName.fill("   ");
    await lastName.fill("");
    await form.getByRole("button", { name: "Зберегти зміни" }).click();
    await expect(firstName).toBeFocused();
    await expect(firstName).toHaveAttribute("aria-invalid", "true");
    await expect(lastName).toHaveAttribute("aria-invalid", "true");
    await expect(form.getByText("Введи ім’я: від 1 до 100 символів.")).toBeVisible();
    await expect(form.getByText("Введи прізвище: від 1 до 100 символів.")).toBeVisible();
    expect(writes(world, "me/profile")).toHaveLength(0);

    await firstName.fill("Марія");
    await lastName.fill("Бондаренко");
    world.addFault({
      method: "PATCH",
      path: "me/profile",
      status: 422,
      code: "validation_failed",
      fields: { universityId: "invalid" },
    });
    await form.getByRole("button", { name: "Зберегти зміни" }).click();
    await expect(form.getByText("Обери університет зі списку.")).toBeVisible();
    await expect(form.locator('select[name="universityId"]')).toBeFocused();
    await expect(firstName).toHaveValue("Марія");
    expect(student.firstName).toBe("Ольга");
    await form.getByRole("button", { name: "Зберегти зміни" }).click();
    await expect(form.getByText("Зміни збережено.")).toBeVisible();
    await expect(form.locator('[aria-invalid="true"]')).toHaveCount(0);
  });

  test("a candidate is read-only and a foreign query parameter cannot select the profile being edited", async ({
    page,
  }) => {
    const world = createMvpWorld();
    const student = seedStudent(world);
    const partner = seedStudent(world, {
      email: "partner@students.example.test",
      firstName: "Андрій",
    });
    await world.attach(page);
    await world.signIn(page, student.email);
    await page.goto(`/students/${partner.id}`);
    await expect(page.getByRole("heading", { name: "Андрій Коваленко" })).toBeVisible();
    await expect(page.getByRole("form", { name: "Редагування профілю" })).toHaveCount(0);
    await expect(page.getByRole("button", { name: "Зберегти зміни" })).toHaveCount(0);

    await page.goto(`/profile?userId=${partner.id}`);
    await expect(page.locator('input[name="firstName"]')).toHaveValue("Ольга");
    await page.locator('input[name="firstName"]').fill("Марія");
    await page.getByRole("button", { name: "Зберегти зміни" }).click();
    await expect(page.getByText("Зміни збережено.")).toBeVisible();
    expect(student.firstName).toBe("Марія");
    expect(partner.firstName).toBe("Андрій");
    expect(writes(world, "me/profile")).toHaveLength(1);
    expect(writes(world, "me/profile")[0].body).not.toHaveProperty("userId");
  });
});

test.describe("FR-03 — separate skill lists", () => {
  test("the same catalog skill has independent levels and deletion in both lists", async ({
    page,
  }) => {
    const world = createMvpWorld();
    const student = seedStudent(world);
    await world.attach(page);
    await world.signIn(page, student.email);
    await page.goto("/profile?tab=skills");
    await page.getByRole("button", { name: "+ Додати навичку для вивчення" }).click();
    const addForm = page.getByRole("form", { name: "Додати навичку" });
    await addForm.getByLabel("Навичка зі спільного каталогу").selectOption(IDS.skills.guitar);
    await addForm.getByLabel("Мій поточний рівень").selectOption("BEGINNER");
    await addForm.getByRole("button", { name: "Додати", exact: true }).click();
    await expect(page.getByLabel("Рівень Гітара: вивчаю")).toHaveValue("BEGINNER");
    await expect(page.getByLabel("Рівень Гітара: навчаю")).toHaveValue("ADVANCED");
    await page.getByLabel("Рівень Гітара: вивчаю").selectOption("INTERMEDIATE");
    await expect
      .poll(
        () => student.learningSkills.find((skill) => skill.skillId === IDS.skills.guitar)?.level,
      )
      .toBe("INTERMEDIATE");
    await page.reload();
    await expect(page.getByLabel("Рівень Гітара: вивчаю")).toHaveValue("INTERMEDIATE");
    await expect(page.getByLabel("Рівень Гітара: навчаю")).toHaveValue("ADVANCED");
    await page.getByRole("button", { name: "Видалити Гітара зі списку вивчаю" }).click();
    await expect(page.getByLabel("Рівень Гітара: вивчаю")).toHaveCount(0);
    await expect(page.getByLabel("Рівень Гітара: навчаю")).toHaveValue("ADVANCED");
    await page.reload();
    await expect(page.getByLabel("Рівень Гітара: вивчаю")).toHaveCount(0);
    await expect(page.getByLabel("Рівень Photoshop: вивчаю")).toHaveValue("BEGINNER");
    expect(writes(world, "me/learning-skills")[0].body).toEqual({
      skillId: IDS.skills.guitar,
      level: "BEGINNER",
    });
    expect(
      writes(world, `me/learning-skills/${IDS.skills.guitar}`).map((request) => request.method),
    ).toEqual(["PATCH", "DELETE"]);
  });

  for (const list of ["teaching", "learning"]) {
    test(`${list} duplicate conflict reconciles the profile without displaying two rows`, async ({
      page,
    }) => {
      const world = createMvpWorld();
      const student = seedStudent(world);
      await world.attach(page);
      await world.signIn(page, student.email);
      await page.goto("/profile?tab=skills");
      await page
        .getByRole("button", {
          name:
            list === "teaching"
              ? "+ Додати навичку, якої я можу навчити"
              : "+ Додати навичку для вивчення",
        })
        .click();
      const addForm = page.getByRole("form", { name: "Додати навичку" });
      await addForm.getByLabel("Навичка зі спільного каталогу").selectOption(IDS.skills.python);

      student[list === "teaching" ? "teachingSkills" : "learningSkills"].push({
        skillId: IDS.skills.python,
        categoryId: IDS.categories.tech,
        name: "Python",
        level: "ADVANCED",
      });
      await addForm.getByRole("button", { name: "Додати", exact: true }).click();
      await expect(page.getByText("Ця навичка вже є в списку.")).toBeVisible();
      const level = page.getByLabel(`Рівень Python: ${list === "teaching" ? "навчаю" : "вивчаю"}`);
      await expect(level).toHaveCount(1);
      await expect(level).toHaveValue("ADVANCED");
      await expect(
        addForm
          .getByLabel("Навичка зі спільного каталогу")
          .locator(`option[value="${IDS.skills.python}"]`),
      ).toHaveCount(0);
      expect(writes(world, `me/${list}-skills`)).toHaveLength(1);
      await addForm.getByRole("button", { name: "Скасувати" }).click();
      await page.reload();
      await expect(level).toHaveCount(1);
    });
  }

  for (const [label, missing] of [
    ["teaching skills", { teachingSkills: [] }],
    ["learning skills", { learningSkills: [] }],
    ["lesson format", { formats: [] }],
    ["offline city", { formats: ["OFFLINE"], city: "" }],
  ]) {
    test(`matching explains an incomplete profile without ${label}`, async ({ page }) => {
      const world = createMvpWorld();
      const student = seedStudent(world, missing);
      await world.attach(page);
      await world.signIn(page, student.email);
      await page.goto("/matches");
      await expect(page.getByText("Профіль ще не готовий до підбору")).toBeVisible();
      await expect(page.getByText("Заповни профіль, щоб бачити взаємні збіги.")).toBeVisible();
      await expect(page.locator(".discovery-grid > li")).toHaveCount(0);
      await page.getByRole("link", { name: "Перейти до профілю", exact: true }).click();
      await expect(page).toHaveURL(`${BASE_URL}/profile`);
    });
  }
});
