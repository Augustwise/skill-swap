import { expect, test } from "@playwright/test";
import { reviewFixture, exchangePath } from "./review-fixture.mjs";

async function openForm(page) {
  await page.goto(exchangePath);
  await page.getByRole("button", { name: "Залишити відгук", exact: true }).click();
  return page.getByRole("dialog");
}

test("review validates rating and Unicode text, saves once and persists after reload", async ({
  page,
}) => {
  const state = await reviewFixture(page);
  const dialog = await openForm(page);
  const send = dialog.getByRole("button", { name: "Надіслати відгук", exact: true });
  await expect(dialog.getByRole("radio").first()).toBeFocused();
  await send.click();
  await expect(dialog.getByRole("alert")).toHaveText("Обери оцінку від 1 до 5.");
  await dialog.getByRole("radio", { name: "5 з 5 — Чудово", exact: true }).check();
  const comment = dialog.getByLabel("Текст відгуку", { exact: false });
  await comment.fill("🙂".repeat(1001));
  await send.click();
  await expect(comment).toHaveAttribute("aria-invalid", "true");
  expect(state.posts).toHaveLength(0);
  await comment.fill("  " + "🙂".repeat(1000) + "  ");
  await expect(dialog.locator(".review-counter")).toHaveText("1000 / 1000 символів");
  let release;
  state.hold = new Promise((resolve) => {
    release = resolve;
  });
  await send.click();
  try {
    await expect(dialog.getByRole("button", { name: "Надсилаємо…" })).toBeDisabled();
    await expect(dialog.getByRole("button", { name: "Закрити вікно" })).toBeDisabled();
    await page.keyboard.press("Escape");
    await expect(dialog).toBeVisible();
  } finally {
    release();
  }
  await expect(dialog).toHaveCount(0);
  await expect(page.getByText("Дякуємо! Твій відгук збережено", { exact: false })).toBeVisible();
  expect(state.posts).toEqual([{ rating: 5, comment: "🙂".repeat(1000) }]);
  await page.reload();
  await expect(page.getByRole("article", { name: "Твій відгук про партнера" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Залишити відгук", exact: true })).toHaveCount(0);
});

test("a rating without text is valid and keyboard selection works", async ({ page }) => {
  const state = await reviewFixture(page);
  const dialog = await openForm(page);
  await page.keyboard.press("ArrowRight");
  await expect(
    dialog.getByRole("radio", { name: "2 з 5 — Могло бути краще", exact: true }),
  ).toBeChecked();
  await dialog.getByRole("button", { name: "Надіслати відгук" }).click();
  await expect(page.getByRole("article", { name: "Твій відгук про партнера" })).toContainText(
    "Оцінка без текстового відгуку.",
  );
  expect(state.posts).toEqual([{ rating: 2, comment: "" }]);
});

for (const failure of ["network", "unavailable", "validation_failed", "unauthenticated"]) {
  test(`failed review retains draft and supports retry: ${failure}`, async ({ page }) => {
    const state = await reviewFixture(page);
    const dialog = await openForm(page);
    await dialog.getByRole("radio", { name: "4 з 5 — Дуже добре", exact: true }).check();
    const comment = dialog.getByLabel("Текст відгуку", { exact: false });
    await comment.fill("Мій збережений текст");
    state.failure = failure;
    await dialog.getByRole("button", { name: "Надіслати відгук" }).click();
    await expect(dialog.getByRole("alert").last()).toBeVisible();
    await expect(comment).toHaveValue("Мій збережений текст");
    await expect(
      dialog.getByRole("radio", { name: "4 з 5 — Дуже добре", exact: true }),
    ).toBeChecked();
    if (failure === "unauthenticated") {
      const href = await dialog
        .getByRole("link", { name: "Увійти", exact: true })
        .getAttribute("href");
      expect(new URL(href, page.url()).searchParams.get("next")).toBe(exchangePath);
    }
    if (failure === "validation_failed")
      await expect(
        dialog.getByText("Введи текст до 1000 символів", { exact: false }),
      ).toBeVisible();
    state.failure = "";
    await dialog.getByRole("button", { name: "Надіслати відгук" }).click();
    await expect(dialog).toHaveCount(0);
    await expect(page.getByRole("article", { name: "Твій відгук про партнера" })).toContainText(
      "Мій збережений текст",
    );
  });
}

test("a concurrent duplicate reloads the existing review without submitting again", async ({
  page,
}) => {
  const state = await reviewFixture(page);
  const dialog = await openForm(page);
  await dialog.getByRole("radio", { name: "5 з 5 — Чудово", exact: true }).check();
  state.mine = { ...state.example, comment: "Вже надіслано в іншій вкладці" };
  state.base.exchange.canReview = false;
  await dialog.getByRole("button", { name: "Надіслати відгук" }).click();
  await expect(dialog).toHaveCount(0);
  await expect(page.getByRole("article", { name: "Твій відгук про партнера" })).toContainText(
    "Вже надіслано в іншій вкладці",
  );
  expect(state.posts).toHaveLength(1);
});

for (const status of ["ACTIVE", "CANCELLED"]) {
  test(`an ${status} exchange has no review form`, async ({ page }) => {
    const state = await reviewFixture(page);
    state.base.exchange.status = status;
    state.base.exchange.canReview = false;
    await page.goto(exchangePath);
    await expect(page.getByRole("heading", { name: "Погоджені умови" })).toBeVisible();
    await expect(page.getByRole("button", { name: "Залишити відгук", exact: true })).toHaveCount(0);
    expect(state.posts).toHaveLength(0);
  });
}

test("a foreign exchange remains inaccessible", async ({ page }) => {
  await reviewFixture(page);
  await page.route("**/api/v1/exchanges/*/sessions", (route) =>
    route.fulfill({ status: 404, json: { error: { code: "exchange_not_found" } } }),
  );
  await page.goto(exchangePath);
  await expect(page.getByText("Обмін не знайдено.", { exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "Залишити відгук", exact: true })).toHaveCount(0);
});

test("both exchange reviews render HTML as plain text and refresh independently", async ({
  page,
}) => {
  const state = await reviewFixture(page);
  state.mine = { ...state.example, comment: '<img src=x onerror="window.bad=true">' };
  state.partner = { ...state.example, id: "partner-review", comment: "Чудові заняття" };
  state.base.exchange.canReview = false;
  await page.goto(exchangePath);
  const mine = page.getByRole("article", { name: "Твій відгук про партнера" });
  await expect(mine).toContainText(state.mine.comment);
  await expect(mine.locator("img")).toHaveCount(0);
  await expect(page.getByRole("article", { name: "Відгук партнера про тебе" })).toContainText(
    "Чудові заняття",
  );
  state.partner.comment = "Оновлений відгук";
  await page.getByRole("button", { name: "Оновити відгуки" }).click();
  await expect(page.getByRole("article", { name: "Відгук партнера про тебе" })).toContainText(
    "Оновлений відгук",
  );
});

test("student reviews paginate in the URL, preserve return filters and show aggregate rating", async ({
  page,
}) => {
  const state = await reviewFixture(page);
  const from = "/search?q=гітара&page=2";
  await page.goto(`${state.studentPath}?from=${encodeURIComponent(from)}`);
  const section = page.getByRole("region", { name: "Відгуки", exact: true });
  await expect(section.getByRole("article")).toHaveCount(20);
  await expect(section.locator(".review-rating")).toContainText("4,5");
  await expect(section.locator(".review-rating")).toContainText("21 відгук");
  await section.getByRole("link", { name: "Наступна →" }).click();
  await expect(section.getByRole("article")).toHaveCount(1);
  await expect(section).toContainText("Відгук 21:");
  expect(new URL(page.url()).searchParams.get("reviewsPage")).toBe("2");
  expect(new URL(page.url()).searchParams.get("from")).toBe(from);
  await page.reload();
  await expect(section).toContainText("Сторінка 2 з 2");
  await page.goBack();
  await expect(section.getByRole("article")).toHaveCount(20);
});

test("own reviews have a persistent profile tab and an empty rating state", async ({ page }) => {
  const state = await reviewFixture(page);
  state.total = 0;
  await page.goto("/profile?tab=reviews");
  await expect(page.getByRole("region", { name: "Відгуки про мене", exact: true })).toContainText(
    "Ще немає відгуків",
  );
  await expect(page.getByRole("link", { name: "Відгуки про мене", exact: true })).toHaveAttribute(
    "aria-current",
    "page",
  );
  await page.reload();
  await expect(page.getByRole("region", { name: "Відгуки про мене", exact: true })).toBeVisible();
  state.total = 21;
  await page.getByRole("button", { name: "Оновити відгуки" }).click();
  await page.getByRole("link", { name: "Наступна →" }).click();
  await expect(page).toHaveURL(/tab=reviews&reviewsPage=2/);
  await expect(page.getByRole("region", { name: "Відгуки про мене", exact: true })).toContainText(
    "Відгук 21:",
  );
});

test("review list failures can be retried and out-of-range pages return to the first page", async ({
  page,
}) => {
  const state = await reviewFixture(page);
  state.getFailure = true;
  await page.goto(`${state.studentPath}?reviewsPage=3`);
  const section = page.getByRole("region", { name: "Відгуки", exact: true });
  await expect(section.getByRole("alert")).toBeVisible();
  state.getFailure = false;
  await section.getByRole("button", { name: "Спробувати ще раз" }).click();
  await expect(
    section.getByText("На цій сторінці немає відгуків.", { exact: false }),
  ).toBeVisible();
  await section.getByRole("link", { name: "До першої сторінки" }).click();
  await expect(section.getByRole("article")).toHaveCount(20);
});

test("a stale completed exchange refreshes its state after a rejected review", async ({ page }) => {
  const state = await reviewFixture(page);
  const dialog = await openForm(page);
  await dialog.getByRole("radio", { name: "5 з 5 — Чудово", exact: true }).check();
  state.failure = "exchange_not_completed";
  state.base.exchange.status = "ACTIVE";
  state.base.exchange.canReview = false;
  await dialog.getByRole("button", { name: "Надіслати відгук" }).click();
  await expect(dialog).toHaveCount(0);
  await expect(page.getByRole("heading", { name: "Погоджені умови" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Залишити відгук", exact: true })).toHaveCount(0);
  expect(state.posts).toHaveLength(1);
});

test("a saved review remains visible if the subsequent read fails", async ({ page }) => {
  const state = await reviewFixture(page);
  const dialog = await openForm(page);
  await dialog.getByRole("radio", { name: "5 з 5 — Чудово", exact: true }).check();
  state.getFailure = true;
  await dialog.getByRole("button", { name: "Надіслати відгук" }).click();
  await expect(page.getByRole("article", { name: "Твій відгук про партнера" })).toBeVisible();
  const section = page.getByRole("region", { name: "Відгуки про обмін" });
  await expect(section.getByRole("alert")).toBeVisible();
  state.getFailure = false;
  await section.getByRole("button", { name: "Спробувати ще раз" }).click();
  await expect(section.getByRole("alert")).toHaveCount(0);
  expect(state.posts).toHaveLength(1);
});

test("a received-review notification marks read and opens the reviewed exchange", async ({
  page,
}) => {
  const state = await reviewFixture(page);
  const notification = {
    id: "70000000-0000-0000-0000-000000000009",
    eventType: "REVIEW_RECEIVED",
    title: "Новий відгук",
    body: "Андрій оцінив обмін на 5 з 5",
    actor: state.base.exchange.partner,
    target: { kind: "EXCHANGE", id: state.base.exchange.id },
    exchangeId: state.base.exchange.id,
    reviewId: "60000000-0000-0000-0000-000000000009",
    isRead: false,
    createdAt: "2026-10-20T10:00:00Z",
  };
  await page.route("**/api/v1/me/notifications**", async (route) => {
    if (route.request().method() === "POST") {
      expect(route.request().headers()["x-csrf-token"]).toBe("test-csrf");
      notification.isRead = true;
      return route.fulfill({ json: { notification, unreadCount: 0 } });
    }
    return route.fulfill({
      json: {
        items: [notification],
        total: 1,
        page: 1,
        pageSize: 20,
        unreadCount: notification.isRead ? 0 : 1,
      },
    });
  });
  await page.goto("/notifications");
  await page
    .locator("main")
    .getByRole("link", { name: /Новий відгук/ })
    .click();
  await expect(page).toHaveURL(exchangePath);
  await expect(page.getByRole("heading", { name: "Відгуки про обмін" })).toBeVisible();
  expect(notification.isRead).toBe(true);
});

for (const width of [360, 768, 1440, 1920]) {
  test(`reviews and dialog fit the viewport at ${width}px`, async ({ page }, testInfo) => {
    const state = await reviewFixture(page);
    await page.setViewportSize({ width, height: 900 });
    const dialog = await openForm(page);
    await dialog.getByRole("radio", { name: "5 з 5 — Чудово", exact: true }).check();
    await dialog
      .getByLabel("Текст відгуку", { exact: false })
      .fill("Дякую за корисний обмін знаннями!");
    await page.screenshot({ path: testInfo.outputPath(`review-dialog-${width}.png`) });
    expect(
      await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth),
    ).toBe(true);
    const bounds = await dialog.boundingBox();
    expect(bounds.x).toBeGreaterThanOrEqual(0);
    expect(bounds.x + bounds.width).toBeLessThanOrEqual(width);
    await page.keyboard.press("Escape");
    await expect(dialog).toHaveCount(0);
    await expect(page.getByRole("button", { name: "Залишити відгук", exact: true })).toBeFocused();
    await page.goto(state.studentPath);
    await expect(
      page.getByRole("region", { name: "Відгуки", exact: true }).getByRole("article"),
    ).toHaveCount(20);
    expect(
      await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth),
    ).toBe(true);
    await page.screenshot({
      path: testInfo.outputPath(`student-reviews-${width}.png`),
      fullPage: true,
    });
  });
}
