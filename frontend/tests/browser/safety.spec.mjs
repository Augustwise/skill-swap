import { expect, test } from "@playwright/test";
import { safetyFixture, reportPath, categoryId } from "./safety-fixture.mjs";

test("privacy is saved from server values, survives reload and keeps all-status controls", async ({
  page,
}) => {
  const state = await safetyFixture(page);
  await page.goto("/settings");
  const visible = page.getByRole("switch", { name: "Показувати мій профіль", exact: false });
  await expect(visible).toBeChecked();
  await visible.uncheck();
  await expect(
    page.getByText("Чинні обміни, заняття та чати збережуться.", { exact: false }),
  ).toBeVisible();
  await page.getByLabel("Хто може надсилати запити").selectOption("SAME_UNIVERSITY");
  await page.getByRole("button", { name: "Зберегти налаштування" }).click();
  await expect(page.getByRole("status")).toContainText("збережено");
  expect(state.privacy).toEqual({
    isDiscoverable: false,
    acceptsRequests: true,
    requestScope: "SAME_UNIVERSITY",
  });
  await page.reload();
  await expect(visible).not.toBeChecked();
  await expect(page.getByLabel("Хто може надсилати запити")).toHaveValue("SAME_UNIVERSITY");
});

test("failed privacy reads retry and failed writes preserve the chosen values", async ({
  page,
}) => {
  const state = await safetyFixture(page);
  state.readFailure = "me/privacy";
  await page.goto("/settings");
  await expect(page.locator(".safety-feedback")).toBeVisible();
  state.readFailure = "";
  await page.getByRole("button", { name: "Спробувати ще раз" }).click();
  await page.getByRole("switch", { name: "Приймати нові запити", exact: false }).uncheck();
  state.failure = "unavailable";
  await page.getByRole("button", { name: "Зберегти налаштування" }).click();
  await expect(page.locator(".safety-feedback")).toBeVisible();
  await expect(
    page.getByRole("switch", { name: "Приймати нові запити", exact: false }),
  ).not.toBeChecked();
  expect(state.privacy.acceptsRequests).toBe(true);
  state.failure = "";
  await page.getByRole("button", { name: "Зберегти налаштування" }).click();
  await expect(page.getByRole("status")).toContainText("збережено");
});

test("password form validates confirmation, displays wrong password and clears secrets on success", async ({
  page,
}) => {
  const state = await safetyFixture(page);
  await page.goto("/settings?tab=security");
  await page.getByLabel("Поточний пароль", { exact: true }).fill("old password");
  await page.getByLabel("Новий пароль", { exact: true }).fill("new password 123");
  await page.getByLabel("Повтори новий пароль").fill("different");
  await page.getByRole("button", { name: "Змінити пароль", exact: true }).click();
  await expect(page.getByText("Паролі не збігаються.")).toBeVisible();
  expect(state.mutations).toHaveLength(0);
  await page.getByLabel("Повтори новий пароль").fill("new password 123");
  state.failure = "validation_failed";
  state.failFields = { currentPassword: "English validation" };
  await page.getByRole("button", { name: "Змінити пароль", exact: true }).click();
  await expect(page.getByText("Поточний пароль неправильний.")).toBeVisible();
  await expect(page.getByLabel("Новий пароль", { exact: true })).toHaveValue("new password 123");
  state.failure = "";
  await page.getByRole("button", { name: "Змінити пароль", exact: true }).click();
  await expect(page.getByRole("status")).toContainText("Пароль змінено");
  await expect(page.getByLabel("Поточний пароль", { exact: true })).toHaveValue("");
  await expect(page.getByLabel("Новий пароль", { exact: true })).toHaveValue("");
});

test("deletion requires password and explicit acknowledgement, cancel restores focus, pending blocks Escape", async ({
  page,
}) => {
  const state = await safetyFixture(page);
  await page.goto("/settings?tab=security");
  const trigger = page.getByRole("button", { name: "Видалити акаунт", exact: true });
  await trigger.click();
  let dialog = page.getByRole("dialog");
  await expect(dialog.getByRole("button", { name: "Залишити акаунт" })).toBeFocused();
  await expect(dialog.getByRole("button", { name: "Підтвердити видалення" })).toBeDisabled();
  await page.keyboard.press("Escape");
  await expect(trigger).toBeFocused();
  expect(state.mutations).toHaveLength(0);
  await trigger.click();
  dialog = page.getByRole("dialog");
  await dialog.getByRole("checkbox").check();
  await dialog.getByRole("button", { name: "Підтвердити видалення" }).click();
  await expect(dialog.getByLabel("Поточний пароль")).toBeFocused();
  await dialog.getByLabel("Поточний пароль").fill("old password");
  let release;
  state.hold = new Promise((resolve) => {
    release = resolve;
  });
  await dialog.getByRole("button", { name: "Підтвердити видалення" }).click();
  try {
    await expect(dialog.getByRole("button", { name: "Видаляємо…" })).toBeDisabled();
    await page.keyboard.press("Escape");
    await expect(dialog).toBeVisible();
  } finally {
    release();
  }
  await expect(page).toHaveURL(/\/login\?deleted=1$/);
  await expect(
    page.getByText("Акаунт видалено, профіль знеособлено. Усі сесії завершено."),
  ).toBeVisible();
  expect(state.mutations).toEqual([
    { path: "me/account", method: "DELETE", body: { password: "old password" } },
  ]);
});

for (const failure of ["network", "unauthenticated", "report_already_open"]) {
  test(`report retains reason on ${failure} and provides recovery`, async ({ page }) => {
    const state = await safetyFixture(page);
    await page.goto(state.partnerPath);
    await page.getByRole("button", { name: "Поскаржитися" }).click();
    const dialog = page.getByRole("dialog");
    const reason = dialog.getByLabel("Причина скарги");
    await reason.fill(" Моя скарга ");
    state.failure = failure;
    await dialog.getByRole("button", { name: "Надіслати скаргу" }).click();
    await expect(dialog.getByRole("alert")).toBeVisible();
    await expect(reason).toHaveValue(" Моя скарга ");
    if (failure === "unauthenticated")
      await expect(dialog.getByRole("link", { name: "Увійти знову" })).toHaveAttribute(
        "href",
        /next=%2Fstudents/,
      );
    if (failure === "report_already_open")
      await expect(dialog.getByRole("alert")).toContainText("вже є відкрита скарга");
    state.failure = "";
    await dialog.getByRole("button", { name: "Надіслати скаргу" }).click();
    await expect(dialog).toHaveCount(0);
    expect(state.reports[0].reason).toBe("Моя скарга");
  });
}

test("report validates Unicode boundary, submits plain text once and status is visible in settings", async ({
  page,
}) => {
  const state = await safetyFixture(page);
  await page.goto(state.partnerPath);
  await page.getByRole("button", { name: "Поскаржитися" }).click();
  const dialog = page.getByRole("dialog");
  await dialog.getByRole("button", { name: "Надіслати скаргу" }).click();
  await expect(dialog.getByLabel("Причина скарги")).toBeFocused();
  await dialog.getByLabel("Причина скарги").fill("🙂".repeat(1001));
  await dialog.getByRole("button", { name: "Надіслати скаргу" }).click();
  await expect(dialog.getByLabel("Причина скарги")).toHaveAttribute("aria-invalid", "true");
  expect(state.mutations).toHaveLength(0);
  const text = "<script>window.hacked = true</script>\nПричина скарги";
  await dialog.getByLabel("Причина скарги").fill(text);
  await dialog.getByRole("button", { name: "Надіслати скаргу" }).click();
  await expect(dialog).toHaveCount(0);
  await page.goto("/settings?tab=reports");
  await expect(page.locator(".safety-text")).toHaveText(text);
  await expect(page.getByText("Очікує розгляду", { exact: true })).toBeVisible();
  expect(await page.evaluate(() => window.hacked)).toBeUndefined();
  expect(state.mutations).toHaveLength(1);
});

for (const source of ["partnerPath", "exchangePath", "requestPath", "chatPath"]) {
  test(`blocking from ${source} updates server state and history stays reachable`, async ({
    page,
  }) => {
    const state = await safetyFixture(page);
    await page.goto(state[source]);
    await page.getByRole("button", { name: "Заблокувати", exact: true }).click();
    const dialog = page.getByRole("dialog");
    await expect(dialog).toContainText("Усі спільні активні обміни");
    await dialog.getByRole("button", { name: "Підтвердити блокування" }).click();
    await expect(dialog).toHaveCount(0);
    expect(state.blocked).toBe(true);
    expect(state.base.exchange.status).toBe("CANCELLED");
    if (source === "partnerPath") await expect(page).toHaveURL(/settings\?tab=blocks/);
    if (source === "chatPath") {
      await expect(page.locator(".chat-blocked")).toBeVisible();
      await expect(page.getByRole("button", { name: "Розблокувати", exact: true })).toBeVisible();
      await page.getByRole("button", { name: "Розблокувати", exact: true }).click();
      await page
        .getByRole("dialog")
        .getByRole("button", { name: "Підтвердити розблокування" })
        .click();
      await expect(page.locator(".chat-blocked")).toHaveCount(0);
      expect(state.base.exchange.status).toBe("CANCELLED");
    }
    await page.goto(state.exchangePath);
    await expect(
      page.getByText("Один з учасників заблокував іншого", { exact: false }).first(),
    ).toBeVisible();
  });
}

test("blocked list paginates through URL and unblocking does not resurrect exchanges", async ({
  page,
}) => {
  const state = await safetyFixture(page);
  state.blocked = true;
  state.blocks = Array.from({ length: 21 }, (_, index) => ({
    user: {
      ...state.partner,
      id:
        index === 20
          ? state.partner.id
          : `a0000000-0000-0000-0000-${String(index).padStart(12, "0")}`,
      firstName: `Студент ${index + 1}`,
    },
    createdAt: "2026-10-20T10:00:00Z",
  }));
  await page.goto("/settings?tab=blocks");
  await page.getByRole("link", { name: "Наступна →", exact: true }).click();
  await expect(page).toHaveURL(/tab=blocks&page=2/);
  await expect(page.getByRole("heading", { name: "Студент 21 Тест" })).toBeVisible();
  await page.getByRole("button", { name: "Розблокувати", exact: true }).click();
  await page.getByRole("dialog").getByRole("button", { name: "Підтвердити розблокування" }).click();
  await expect(page.locator(".profile-success")).toContainText("Скасовані обміни не відновлено");
  expect(state.blocked).toBe(false);
});

test("a successful block stays acknowledged when the following exchange read fails", async ({
  page,
}) => {
  const state = await safetyFixture(page);
  await page.goto(state.exchangePath);
  await page.getByRole("button", { name: "Заблокувати", exact: true }).click();
  state.readFailure = `${state.exchangePath.slice(1)}/sessions`;
  await page.getByRole("dialog").getByRole("button", { name: "Підтвердити блокування" }).click();
  await expect(page.getByText("Студента заблоковано.", { exact: false })).toBeVisible();
  await expect(page.locator(".discovery-alert")).toBeVisible();
  expect(state.mutations).toHaveLength(1);
});

test("student cannot fetch admin records by opening an admin URL or see moderation navigation", async ({
  page,
}) => {
  const state = await safetyFixture(page);
  await page.goto(reportPath);
  await expect(
    page.getByRole("heading", { name: "Доступ лише для адміністраторів" }),
  ).toBeVisible();
  expect(state.reads.some((path) => path.startsWith("admin/"))).toBe(false);
  await page.getByRole("button", { name: "Меню користувача" }).click();
  await expect(page.getByRole("link", { name: "Модерація", exact: true })).toHaveCount(0);
});

for (const decision of ["DISMISS", "RESOLVE", "SUSPEND"]) {
  test(`administrator records ${decision} with author, date and reason`, async ({ page }) => {
    const state = await safetyFixture(page, "ADMIN");
    await page.goto(reportPath);
    await page.getByRole("button", { name: "Ухвалити рішення" }).click();
    const dialog = page.getByRole("dialog");
    await dialog.getByRole("combobox", { name: "Рішення", exact: true }).selectOption(decision);
    await dialog.getByRole("button", { name: "Зберегти рішення" }).click();
    await expect(dialog.getByLabel("Причина рішення")).toBeFocused();
    await dialog.getByLabel("Причина рішення").fill(" Порушення перевірено ");
    await dialog.getByRole("button", { name: "Зберегти рішення" }).click();
    await expect(dialog).toHaveCount(0);
    await expect(page.getByRole("heading", { name: "Рішення за скаргою" })).toBeVisible();
    await expect(page.getByRole("button", { name: "Ухвалити рішення" })).toHaveCount(0);
    expect(state.actions[0].reason).toBe("Порушення перевірено");
    expect(state.actions[0].moderator.id).toBe(state.me.id);
    expect(state.actions[0].createdAt).toBeTruthy();
    expect(state.report.status).toBe(decision === "DISMISS" ? "DISMISSED" : "RESOLVED");
    if (decision === "SUSPEND") expect(state.report.reportedUser.accountStatus).toBe("SUSPENDED");
    await page.goto("/admin?tab=history");
    await expect(page.locator(".safety-list")).toContainText("Порушення перевірено");
  });
}

test("concurrent report conflict retains draft and lets administrator reload the decided report", async ({
  page,
}) => {
  const state = await safetyFixture(page, "ADMIN");
  await page.goto(reportPath);
  await page.getByRole("button", { name: "Ухвалити рішення" }).click();
  const dialog = page.getByRole("dialog");
  await dialog.getByLabel("Причина рішення").fill("Моя причина");
  state.failure = "report_already_resolved";
  state.report.status = "RESOLVED";
  await dialog.getByRole("button", { name: "Зберегти рішення" }).click();
  await expect(dialog.getByRole("alert")).toContainText("Інший адміністратор");
  await expect(dialog.getByLabel("Причина рішення")).toHaveValue("Моя причина");
  await dialog.getByRole("button", { name: "Оновити дані" }).click();
  await expect(page.getByRole("button", { name: "Ухвалити рішення" })).toHaveCount(0);
});

test("report filter includes all statuses and pagination retains the filter", async ({ page }) => {
  const state = await safetyFixture(page, "ADMIN");
  state.reportTotal = 21;
  await page.goto("/admin");
  await expect(page.getByRole("combobox", { name: "Статус", exact: true })).toHaveValue("OPEN");
  await page.getByRole("combobox", { name: "Статус", exact: true }).selectOption("");
  await expect(page).toHaveURL(/\/admin\?status=$/);
  await expect(page.getByRole("combobox", { name: "Статус", exact: true })).toHaveValue("");
  await page.getByRole("link", { name: "Наступна →", exact: true }).click();
  await expect(page).toHaveURL(/status=&page=2$/);
});

test("administrator can suspend and restore an account with reasons", async ({ page }) => {
  const state = await safetyFixture(page, "ADMIN");
  await page.goto(reportPath);
  for (const name of ["Призупинити акаунт", "Відновити акаунт"]) {
    await page.getByRole("button", { name, exact: true }).click();
    const dialog = page.getByRole("dialog");
    await dialog.getByLabel("Причина рішення").fill("Перевірена причина");
    await dialog.getByRole("button", { name: "Підтвердити рішення" }).click();
    await expect(dialog).toHaveCount(0);
  }
  expect(state.report.reportedUser.accountStatus).toBe("ACTIVE");
  expect(state.actions.map((item) => item.action)).toEqual(["RESTORE_ACCOUNT", "SUSPEND_ACCOUNT"]);
});

test("administrator cannot suspend themselves and server role revocation is explained", async ({
  page,
}) => {
  const state = await safetyFixture(page, "ADMIN");
  state.report.reportedUser = { ...state.me, accountStatus: "ACTIVE" };
  await page.goto(reportPath);
  await expect(page.getByRole("button", { name: "Призупинити акаунт", exact: true })).toHaveCount(
    0,
  );
  await page.getByRole("button", { name: "Ухвалити рішення" }).click();
  const dialog = page.getByRole("dialog");
  await expect(
    dialog.getByRole("option", { name: "Підтвердити й призупинити акаунт" }),
  ).toHaveAttribute("disabled", "");
  await dialog.getByLabel("Причина рішення").fill("Перевірена причина");
  state.failure = "admin_required";
  await dialog.getByRole("button", { name: "Зберегти рішення" }).click();
  await expect(dialog.getByRole("alert")).toContainText("лише адміністраторам");
});

test("catalog creates, edits, hides and restores skills and keeps failed drafts", async ({
  page,
}) => {
  const state = await safetyFixture(page, "ADMIN");
  await page.goto("/admin?tab=skills");
  await page.getByRole("button", { name: "Додати навичку" }).click();
  let dialog = page.getByRole("dialog");
  await dialog.getByRole("button", { name: "Зберегти навичку" }).click();
  await expect(dialog.getByRole("combobox", { name: "Категорія", exact: true })).toBeFocused();
  await dialog.getByRole("combobox", { name: "Категорія", exact: true }).selectOption(categoryId);
  await dialog.getByLabel("Назва навички").fill("Новий дизайн");
  await dialog.getByLabel("Ідентифікатор навички").fill("new-design");
  state.failure = "skill_already_exists";
  await dialog.getByRole("button", { name: "Зберегти навичку" }).click();
  await expect(dialog.getByRole("alert")).toContainText("уже зайняті");
  await expect(dialog.getByLabel("Назва навички")).toHaveValue("Новий дизайн");
  state.failure = "";
  await dialog.getByRole("button", { name: "Зберегти навичку" }).click();
  await expect(dialog).toHaveCount(0);
  await expect(page.getByRole("heading", { name: "Новий дизайн", exact: true })).toBeVisible();
  for (const active of [false, true]) {
    await page.getByRole("button", { name: "Редагувати Вебдизайн", exact: true }).click();
    dialog = page.getByRole("dialog");
    await dialog.getByLabel("Показувати навичку в каталозі").setChecked(active);
    await dialog.getByRole("button", { name: "Зберегти навичку" }).click();
    await expect(dialog).toHaveCount(0);
    expect(state.skills[0].isActive).toBe(active);
  }
  await page.getByLabel("Пошук у каталозі").fill("Веб");
  await page.getByRole("button", { name: "Знайти", exact: true }).click();
  await expect(page).toHaveURL(/tab=skills&q=/);
  await expect(page.getByRole("heading", { name: "Новий дизайн", exact: true })).toHaveCount(0);
  expect(state.actions).toHaveLength(3);
});

test("settings and moderation remain accessible at 360, 768, 1440 and 1920 px", async ({
  page,
}) => {
  test.setTimeout(90_000);
  await safetyFixture(page, "ADMIN");
  for (const width of [360, 768, 1440, 1920]) {
    await page.setViewportSize({ width, height: 1000 });
    for (const path of [
      "/settings",
      "/settings?tab=security",
      "/settings?tab=blocks",
      "/settings?tab=reports",
      "/admin?status=OPEN",
      "/admin?tab=skills",
      reportPath,
    ]) {
      await page.goto(path);
      await expect(page.locator("#app-content, #settings-content, #admin-content")).toBeVisible();
      await expect(
        page
          .getByRole("heading", { name: "Налаштування", exact: true })
          .or(page.getByRole("heading", { name: "Модерація", exact: true })),
      ).toBeVisible();
      await expect
        .poll(() => page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth))
        .toBe(true);
      if (path === "/settings" || path === "/admin?status=OPEN") {
        await page
          .getByRole("heading", {
            name: path === "/settings" ? "Видимість і нові запити" : /Скарги студентів/,
          })
          .waitFor();
        await page.screenshot({
          path: `test-results/sprint10-${path === "/settings" ? "settings" : "moderation"}-${width}.png`,
          fullPage: true,
        });
      }
    }
  }
});

test("safety dialogs fit narrow screens and keep keyboard focus inside", async ({ page }) => {
  const state = await safetyFixture(page, "ADMIN");
  for (const width of [360, 1440]) {
    await page.setViewportSize({ width, height: 900 });
    for (const [path, trigger, kind] of [
      ["/settings?tab=security", "Видалити акаунт", "delete"],
      [state.partnerPath, "Поскаржитися", "report"],
      [reportPath, "Ухвалити рішення", "decision"],
      ["/admin?tab=skills", "Додати навичку", "skill"],
    ]) {
      await page.goto(path);
      await page.getByRole("button", { name: trigger, exact: true }).click();
      const dialog = page.getByRole("dialog");
      await expect(dialog).toBeVisible();
      await page.keyboard.press("Tab");
      expect(await dialog.evaluate((node) => node.contains(document.activeElement))).toBe(true);
      const bounds = await dialog.boundingBox();
      expect(bounds.x).toBeGreaterThanOrEqual(0);
      expect(bounds.width).toBeLessThanOrEqual(width);
      expect(bounds.height).toBeLessThanOrEqual(900);
      await dialog.screenshot({ path: `test-results/sprint10-dialog-${kind}-${width}.png` });
      await page.keyboard.press("Escape");
      await expect(dialog).toHaveCount(0);
    }
  }
});
