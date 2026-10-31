import { expect, mutations, submitReview, test } from "./trust-requirements-fixture.mjs";

test.describe("FR-11 — completion acceptance gaps", () => {
  test("separate participants confirm a lesson once each and see the same persisted progress", async ({
    trust,
  }) => {
    const { world, exchange, schedule, actor, olha, andrii } = trust;
    const active = await exchange();
    const lesson = await schedule(active.lessons[0]);
    world.advanceMinutes(300);
    const sender = await actor(olha);
    const recipient = await actor(andrii);
    await sender.page.goto(`/sessions/${lesson.id}`);
    await sender.page.getByRole("button", { name: "Заняття відбулося", exact: true }).click();
    await expect(
      sender.page.locator('.completion-confirmations [data-confirmed="true"]'),
    ).toHaveCount(1);
    await sender.page.reload();
    await expect(
      sender.page.getByRole("button", { name: "Заняття відбулося", exact: true }),
    ).toHaveCount(0);
    expect(mutations(world, `sessions/${lesson.id}/confirm`)).toHaveLength(1);
    await sender.page.goto(`/exchanges/${active.id}`);
    await expect(sender.page.locator(".exchange-progress__percent")).toHaveText("0%");
    await recipient.page.goto(`/sessions/${lesson.id}`);
    await recipient.page.getByRole("button", { name: "Заняття відбулося", exact: true }).click();
    await expect(
      recipient.page.locator('.completion-confirmations [data-confirmed="true"]'),
    ).toHaveCount(2);
    for (const current of [sender, recipient]) {
      await current.page.goto(`/exchanges/${active.id}`);
      await expect(current.page.locator(".exchange-progress__percent")).toHaveText("50%");
      await expect(
        current.page.getByRole("button", { name: "Підтвердити завершення", exact: true }),
      ).toHaveCount(0);
    }
  });

  for (const kind of ["lesson", "exchange"]) {
    test(`a failed ${kind} confirmation remains available and can be retried`, async ({
      trust,
    }) => {
      const { world, actor } = trust;
      let active;
      let target;
      if (kind === "lesson") {
        active = await trust.exchange();
        target = await trust.schedule(active.lessons[0]);
        world.advanceMinutes(300);
      } else {
        active = await trust.finishedPlan();
        target = world.exchanges.get(active.id);
      }
      const endpoint =
        kind === "lesson" ? `sessions/${target.id}/confirm` : `exchanges/${target.id}/complete`;
      const path = kind === "lesson" ? `/sessions/${target.id}` : `/exchanges/${target.id}`;
      const label = kind === "lesson" ? "Заняття відбулося" : "Підтвердити завершення";
      world.addFault({ method: "POST", path: endpoint, kind: "network" });
      const { page } = await actor();
      await page.goto(path);
      await page.getByRole("button", { name: label, exact: true }).click();
      await expect(page.locator("main .exchange-error")).toContainText(
        "Не вдалося зв’язатися із сервером",
      );
      expect(target.completionConfirmedBy.size).toBe(0);
      await page.getByRole("button", { name: label, exact: true }).click();
      await expect(
        page.getByText("Ти підтвердив. Очікуємо підтвердження партнера.", { exact: true }).first(),
      ).toBeVisible();
      await expect(page.getByRole("button", { name: label, exact: true })).toHaveCount(0);
      expect(target.completionConfirmedBy.size).toBe(1);
      expect(mutations(world, endpoint)).toHaveLength(2);
    });
  }

  test("two final confirmations move an exchange to both participants' history and unlock reviews", async ({
    trust,
  }) => {
    const { world, actor, olha, andrii, finishedPlan } = trust;
    const active = await finishedPlan();
    const sender = await actor(olha);
    const recipient = await actor(andrii);
    await sender.page.goto(`/exchanges/${active.id}`);
    await sender.page.getByRole("button", { name: "Підтвердити завершення", exact: true }).click();
    await expect(
      sender.page
        .getByText("Ти підтвердив. Очікуємо підтвердження партнера.", { exact: true })
        .first(),
    ).toBeVisible();
    await expect(
      sender.page.getByRole("button", { name: "Залишити відгук", exact: true }),
    ).toHaveCount(0);
    expect(world.exchanges.get(active.id).status).toBe("ACTIVE");
    await recipient.page.goto(`/exchanges/${active.id}`);
    await recipient.page
      .getByRole("button", { name: "Підтвердити завершення", exact: true })
      .click();
    await expect(
      recipient.page.getByText("Обмін успішно завершено!", { exact: true }).first(),
    ).toBeVisible();
    for (const current of [sender, recipient]) {
      await current.page.goto(`/exchanges/${active.id}`);
      await expect(
        current.page.getByRole("button", { name: "Залишити відгук", exact: true }),
      ).toBeVisible();
      await expect(
        current.page.getByRole("button", { name: "Скасувати обмін", exact: true }),
      ).toHaveCount(0);
      await current.page.getByRole("link", { name: "До історії обмінів →", exact: true }).click();
      await expect(
        current.page
          .getByRole("list", { name: "Завершені обміни", exact: true })
          .locator(":scope > li"),
      ).toHaveCount(1);
    }
  });

  test("a partner cancellation preserves a completed lesson and cancels the remaining future lesson", async ({
    trust,
  }) => {
    const { world, exchange, schedule, prepare, actor, olha, andrii } = trust;
    const active = await exchange();
    await schedule(active.lessons[0]);
    world.advanceMinutes(300);
    await prepare(olha, `sessions/${active.lessons[0].id}/confirm`);
    await prepare(andrii, `sessions/${active.lessons[0].id}/confirm`);
    await schedule(active.lessons[1], { title: "Майбутнє заняття" });
    const recipient = await actor(andrii);
    await recipient.page.goto(`/exchanges/${active.id}`);
    await recipient.page.getByRole("button", { name: "Скасувати обмін", exact: true }).click();
    const dialog = recipient.page.getByRole("dialog");
    await dialog.getByLabel("Причина скасування", { exact: true }).fill("  Змінився мій розклад  ");
    await dialog.getByRole("button", { name: "Скасувати обмін", exact: true }).click();
    await expect(dialog).toHaveCount(0);
    const sender = await actor(olha);
    await sender.page.goto(`/exchanges/${active.id}`);
    await expect(sender.page.locator(".session-history")).toContainText("Андрій Шевченко");
    await expect(sender.page.locator(".session-history")).toContainText("Змінився мій розклад");
    await expect(sender.page.locator(".session-plan .session-status--completed")).toHaveCount(1);
    await expect(sender.page.locator(".session-plan .session-status--cancelled")).toHaveCount(1);
    await expect(sender.page.locator(".exchange-progress__percent")).toHaveText("50%");
    await expect(
      sender.page.getByRole("button", { name: "Підтвердити завершення", exact: true }),
    ).toHaveCount(0);
    await sender.page.goto("/schedule");
    await expect(
      sender.page.getByText("У розкладі поки немає занять.", { exact: true }),
    ).toBeVisible();
  });
});

test.describe("FR-12 — review acceptance gaps", () => {
  test("three actual review submissions display a rounded average and their real count", async ({
    trust,
  }) => {
    test.setTimeout(60_000);
    const { actor, andrii, completed } = trust;
    const { page } = await actor();
    for (const rating of [1, 4, 5]) {
      const active = await completed();
      await submitReview(page, active.id, rating, `Оцінка за окремий обмін: ${rating}`);
    }
    await page.goto(`/students/${andrii.id}`);
    const reviews = page.getByRole("region", { name: "Відгуки", exact: true });
    await expect(reviews.locator(".review-rating strong")).toHaveText("3,3");
    await expect(reviews.locator(".review-rating")).toContainText("3 відгуки");
    await expect(reviews.getByRole("article")).toHaveCount(3);
  });

  for (const rating of [1, 5]) {
    test(`the rating boundary ${rating} and exactly 1000 Unicode code points persist without a second review`, async ({
      trust,
    }) => {
      const { world, completed, actor } = trust;
      const active = await completed();
      const { page } = await actor();
      const comment = "🎸".repeat(1000);
      await submitReview(page, active.id, rating, comment);
      expect(mutations(world, `exchanges/${active.id}/reviews`).map((entry) => entry.body)).toEqual(
        [{ rating, comment }],
      );
      await page.reload();
      const review = page.getByRole("article", { name: "Твій відгук про партнера", exact: true });
      await expect(review).toContainText(comment);
      await expect(
        review.getByRole("img", { name: `Оцінка ${rating} з 5`, exact: true }),
      ).toBeVisible();
      await expect(page.getByRole("button", { name: "Залишити відгук", exact: true })).toHaveCount(
        0,
      );
      expect(world.reviews.size).toBe(1);
    });
  }

  test("a submitted review changes only the partner's rating and remains public to another student", async ({
    trust,
  }) => {
    const { completed, actor, olha, andrii, stranger } = trust;
    const active = await completed();
    const sender = await actor(olha);
    await submitReview(sender.page, active.id, 4, "Пояснює зрозуміло та послідовно");
    await sender.page.goto("/profile?tab=reviews");
    await expect(
      sender.page.getByRole("region", { name: "Відгуки про мене", exact: true }),
    ).toContainText("Ще немає відгуків");
    const outsider = await actor(stranger);
    await outsider.page.goto(`/students/${andrii.id}`);
    const reviews = outsider.page.getByRole("region", { name: "Відгуки", exact: true });
    await expect(reviews.locator(".review-rating strong")).toHaveText("4,0");
    await expect(reviews.getByRole("article")).toContainText("Ольга Коваленко");
    await expect(reviews.getByRole("article")).toContainText("Пояснює зрозуміло та послідовно");
  });
});
