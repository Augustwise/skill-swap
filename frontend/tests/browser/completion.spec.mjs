import { expect, test } from "@playwright/test";
import { completionFixture, exchangePath, lessonPath } from "./completion-fixture.mjs";

for (const partnerConfirmed of [false, true]) {
  test(`lesson confirmation counts progress only when partner confirmed: ${partnerConfirmed}`, async ({
    page,
  }) => {
    const state = await completionFixture(page);
    state.lesson.completionConfirmedByPartner = partnerConfirmed;
    await page.goto(lessonPath);
    let release;
    state.hold = new Promise((resolve) => {
      release = resolve;
    });
    await page.getByRole("button", { name: "Заняття відбулося" }).click();
    try {
      await expect(page.getByRole("button", { name: "Підтверджуємо…" })).toBeDisabled();
    } finally {
      release();
    }
    await expect(page.getByRole("button", { name: "Заняття відбулося" })).toHaveCount(0);
    await expect(page.locator(".completion-confirmations")).toContainText("Підтверджено ✓");
    await page.reload();
    await expect(page.locator(".completion-confirmations")).toContainText("Підтверджено ✓");
    expect(state.posts).toHaveLength(1);
    await page.goto(exchangePath);
    await expect(page.locator(".exchange-progress__percent")).toHaveText(
      partnerConfirmed ? "50%" : "0%",
    );
    await expect(page.getByRole("button", { name: "Підтвердити завершення" })).toHaveCount(0);
  });
}

test("future lessons and incomplete plans do not offer completion", async ({ page }) => {
  const state = await completionFixture(page);
  state.lesson.canConfirmCompletion = false;
  state.lesson.endsAt = "2099-01-01T10:00:00Z";
  await page.goto(lessonPath);
  await expect(page.getByRole("heading", { name: "Підтвердження проведення" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Заняття відбулося" })).toHaveCount(0);
  await page.goto(exchangePath);
  await expect(
    page.getByText("Для завершення обміну виконайте план занять в обох напрямках."),
  ).toBeVisible();
  expect(state.posts).toHaveLength(0);
});

for (const partnerConfirmed of [false, true]) {
  test(`final exchange completion requires both participants: ${partnerConfirmed}`, async ({
    page,
  }) => {
    const state = await completionFixture(page);
    Object.assign(state.exchange.progress, {
      percent: 100,
      completedSessions: 2,
      planCompleted: true,
    });
    state.exchange.canConfirmCompletion = true;
    state.exchange.completionConfirmedByPartner = partnerConfirmed;
    await page.goto(exchangePath);
    await page.getByRole("button", { name: "Підтвердити завершення" }).click();
    await expect(page.getByRole("button", { name: "Підтвердити завершення" })).toHaveCount(0);
    await expect(
      page
        .getByText(
          partnerConfirmed
            ? "Обмін успішно завершено!"
            : "Ти підтвердив. Очікуємо підтвердження партнера.",
        )
        .first(),
    ).toBeVisible();
    if (partnerConfirmed) {
      await expect(page.getByRole("button", { name: "Скасувати обмін", exact: true })).toHaveCount(
        0,
      );
      await expect(page.getByRole("link", { name: "До історії обмінів →" })).toHaveAttribute(
        "href",
        "/exchanges?status=COMPLETED",
      );
      await expect(page.getByRole("heading", { name: "Історія обміну" })).toBeVisible();
    }
    expect(state.exchange.status).toBe(partnerConfirmed ? "COMPLETED" : "ACTIVE");
  });
}

test("stale decisions refresh the lesson and expired sessions preserve the return path", async ({
  page,
}) => {
  const state = await completionFixture(page);
  const path = `${lessonPath}?from=${encodeURIComponent("/schedule?upcoming=false&page=2")}`;
  await page.goto(path);
  await expect(page.getByRole("button", { name: "Заняття відбулося" })).toBeVisible();
  state.failure = "session_not_confirmable";
  state.lesson.canConfirmCompletion = false;
  await page.getByRole("button", { name: "Заняття відбулося" }).click();
  await expect(page.locator("main").getByRole("alert")).toContainText("вже не можна підтвердити");
  await expect(page.getByRole("button", { name: "Заняття відбулося" })).toHaveCount(0);
  state.lesson.canConfirmCompletion = true;
  state.failure = "unauthenticated";
  await page.getByRole("button", { name: "Оновити дані" }).click();
  await page.getByRole("button", { name: "Заняття відбулося" }).click();
  const login = page.getByRole("link", { name: "Увійти", exact: true });
  await expect(login).toBeVisible();
  expect(new URL(await login.getAttribute("href"), page.url()).searchParams.get("next")).toBe(path);
});

test("cancellation validates, retains a failed draft and preserves completed lessons and history", async ({
  page,
}) => {
  const state = await completionFixture(page);
  state.lesson.status = "COMPLETED";
  state.lesson.canConfirmCompletion = false;
  await page.goto(exchangePath);
  await page.getByRole("button", { name: "Скасувати обмін", exact: true }).click();
  const dialog = page.getByRole("dialog");
  const reason = dialog.getByLabel("Причина скасування", { exact: true });
  await expect(reason).toBeFocused();
  const submit = dialog.getByRole("button", { name: "Скасувати обмін", exact: true });
  await submit.click();
  await expect(reason).toHaveAttribute("aria-invalid", "true");
  await reason.fill("я".repeat(501));
  await submit.click();
  expect(state.posts).toHaveLength(0);
  await reason.fill("  Змінився розклад  ");
  state.failure = "unavailable";
  await submit.click();
  await expect(dialog.getByRole("alert")).toBeVisible();
  await expect(reason).toHaveValue("  Змінився розклад  ");
  state.failure = "";
  await submit.click();
  await expect(dialog).toHaveCount(0);
  await expect(page.getByRole("heading", { name: "Причина скасування" })).toBeVisible();
  await expect(page.locator(".session-history")).toContainText("Змінився розклад");
  await expect(page.locator(".session-history")).toContainText("Ольга Тест");
  await expect(page.locator(".session-plan")).toContainText("Проведено");
  await expect(page.locator(".session-plan")).toContainText("Скасовано");
  await expect(page.getByRole("button", { name: "Скасувати обмін", exact: true })).toHaveCount(0);
  expect(state.exchange.cancellationReason).toBe("Змінився розклад");
});

test("expired session in cancellation keeps the reason and offers login in the dialog", async ({
  page,
}) => {
  const state = await completionFixture(page);
  await page.goto(exchangePath);
  await page.getByRole("button", { name: "Скасувати обмін", exact: true }).click();
  const dialog = page.getByRole("dialog");
  await dialog.getByLabel("Причина скасування", { exact: true }).fill("Змінився розклад");
  state.failure = "unauthenticated";
  await dialog.getByRole("button", { name: "Скасувати обмін", exact: true }).click();
  const login = dialog.getByRole("link", { name: "Увійти", exact: true });
  await expect(login).toBeVisible();
  expect(new URL(await login.getAttribute("href"), page.url()).searchParams.get("next")).toBe(
    exchangePath,
  );
  await expect(dialog.getByLabel("Причина скасування", { exact: true })).toHaveValue(
    "Змінився розклад",
  );
});

test("archive tabs retain pagination and return to the same list", async ({ page }) => {
  const state = await completionFixture(page);
  state.exchange.status = "COMPLETED";
  state.exchange.canCancel = false;
  state.total = 41;
  await page.goto("/exchanges?status=COMPLETED&page=2");
  await expect(page.getByRole("link", { name: "Завершені", exact: true })).toHaveAttribute(
    "aria-current",
    "page",
  );
  await expect(page.getByRole("link", { name: "Наступна →" })).toHaveAttribute(
    "href",
    "/exchanges?status=COMPLETED&page=3",
  );
  await page.getByRole("link", { name: "Умови та план занять →" }).click();
  const back = page.getByRole("link", { name: "← До списку обмінів" });
  await expect(back).toHaveAttribute("href", "/exchanges?status=COMPLETED&page=2");
  await back.click();
  await expect(page).toHaveURL(/status=COMPLETED&page=2$/);
});

for (const width of [360, 768, 1440]) {
  test(`completion and cancellation remain usable at ${width}px`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 1000 });
    await completionFixture(page);
    await page.goto(lessonPath);
    await expect(page.getByRole("button", { name: "Заняття відбулося" })).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(
      true,
    );
    await page.screenshot({ path: testInfo.outputPath(`lesson-${width}.png`), fullPage: true });
    await page.goto(exchangePath);
    await page.getByRole("button", { name: "Скасувати обмін", exact: true }).click();
    const dialog = page.getByRole("dialog");
    await expect(dialog).toBeVisible();
    expect(await dialog.evaluate((node) => node.scrollWidth <= node.clientWidth)).toBe(true);
    await page.screenshot({
      path: testInfo.outputPath(`cancellation-${width}.png`),
      fullPage: true,
    });
    await dialog.getByRole("button", { name: "Залишити обмін" }).click();
    await expect(dialog).toHaveCount(0);
  });
}
