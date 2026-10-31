import { expect, test } from "./trust-requirements-fixture.mjs";
import { activate, assertLayout, tabTo, type } from "../helpers/keyboard.mjs";

for (const width of [360, 768, 1440]) {
  test(`rescheduling, agreement, confirmations and mutual reviews work with the keyboard at ${width}px`, async ({
    trust,
  }) => {
    test.setTimeout(90_000);
    const { world, exchange, schedule, slot, actor, olha, andrii } = trust;
    const active = await exchange();
    for (const [index, lesson] of active.lessons.entries())
      await schedule(lesson, { ...slot(120 + index * 120), title: `Зустріч ${index + 1}` });
    const sender = await actor(olha);
    const recipient = await actor(andrii);
    for (const current of [sender, recipient])
      await current.page.setViewportSize({ width, height: 900 });

    const firstLesson = active.lessons[0];
    await sender.page.goto(`/sessions/${firstLesson.id}`);
    await activate(
      sender.page,
      sender.page.getByRole("button", { name: "Перенести заняття", exact: true }),
    );
    const planning = sender.page.getByRole("dialog");
    await type(sender.page, planning.getByLabel("Тема заняття"), "Узгоджено клавіатурою");
    await type(
      sender.page,
      planning.getByLabel("Посилання на зустріч"),
      "https://meet.example.test/keyboard",
    );
    await assertLayout(sender.page);
    await activate(sender.page, planning.getByRole("button", { name: "Надіслати пропозицію" }));
    await expect(planning).toHaveCount(0);
    await recipient.page.goto(`/sessions/${firstLesson.id}`);
    await activate(
      recipient.page,
      recipient.page.getByRole("button", { name: "Погодити час", exact: true }),
    );
    await expect(recipient.page.getByText(/Час погоджено/)).toBeVisible();

    world.advanceMinutes(600);
    for (const lesson of active.lessons) {
      for (const current of [sender, recipient]) {
        await current.page.goto(`/sessions/${lesson.id}`);
        await activate(
          current.page,
          current.page.getByRole("button", { name: "Заняття відбулося", exact: true }),
        );
        await expect(
          current.page.locator('.completion-confirmations [data-confirmed="true"]'),
        ).toHaveCount(current === sender ? 1 : 2);
        await assertLayout(current.page);
      }
    }
    for (const current of [sender, recipient]) {
      await activate(
        current.page,
        current.page.getByRole("link", { name: "← До обміну", exact: true }),
      );
      await expect(current.page.locator(".exchange-progress__percent")).toHaveText("100%");
      await activate(
        current.page,
        current.page.getByRole("button", { name: "Підтвердити завершення", exact: true }),
      );
      await expect(
        current.page
          .getByText(
            current === sender
              ? "Ти підтвердив. Очікуємо підтвердження партнера."
              : "Обмін успішно завершено!",
          )
          .first(),
      ).toBeVisible();
    }
    for (const current of [sender, recipient]) {
      await activate(
        current.page,
        current.page.getByRole("button", { name: "Оновити", exact: true }),
      );
      await activate(
        current.page,
        current.page.getByRole("button", { name: "Залишити відгук", exact: true }),
      );
      const review = current.page.getByRole("dialog");
      await tabTo(current.page, review.getByRole("radio", { name: /^1 з 5/ }));
      await current.page.keyboard.press("Space");
      await current.page.keyboard.press("ArrowRight");
      await expect(review.getByRole("radio", { name: /^2 з 5/ })).toBeChecked();
      await type(
        current.page,
        review.getByLabel("Текст відгуку", { exact: false }),
        "Відгук, написаний клавіатурою",
      );
      await assertLayout(current.page);
      await activate(
        current.page,
        review.getByRole("button", { name: "Надіслати відгук", exact: true }),
      );
      await expect(review).toHaveCount(0);
      await expect(
        current.page.getByRole("article", { name: "Твій відгук про партнера" }),
      ).toContainText("Відгук, написаний клавіатурою");
      await assertLayout(current.page);
    }
  });
}
