import {
  blockStudent,
  DEMO_PASSWORD,
  expect,
  IDS,
  mutations,
  openDeletion,
  test,
} from "./trust-requirements-fixture.mjs";

test.describe("FR-14 — moderation and blocking acceptance gaps", () => {
  test("a personal block closes a pending request and cancels active lessons while retaining both histories", async ({
    trust,
  }) => {
    const { world, exchange, schedule, request, actor, andrii } = trust;
    const active = await exchange();
    await schedule(active.lessons[0]);
    const pending = await request();
    const sender = await actor();
    await blockStudent(sender.page, andrii.id);
    const recipient = await actor(andrii);
    await recipient.page.goto(`/requests/${pending.id}`);
    await expect(recipient.page.locator(".exchange-history")).toContainText("Запит відкликано");
    await expect(recipient.page.getByRole("button", { name: "Прийняти", exact: true })).toHaveCount(
      0,
    );
    await recipient.page.goto(`/exchanges/${active.id}`);
    await expect(recipient.page.locator(".session-history")).toContainText(
      "Один з учасників заблокував іншого",
    );
    await expect(recipient.page.locator(".session-plan .session-status--cancelled")).toHaveCount(2);
    await expect(
      recipient.page.getByRole("button", { name: "Залишити відгук", exact: true }),
    ).toHaveCount(0);
    await recipient.page.goto("/schedule");
    await expect(
      recipient.page.getByText("У розкладі поки немає занять.", { exact: true }),
    ).toBeVisible();
    expect(world.exchanges.get(active.id).status).toBe("CANCELLED");
  });

  test("a moderator suspension records its author and reason and revokes both open student sessions", async ({
    trust,
  }) => {
    const { world, exchange, prepare, actor, olha, andrii, admin, secondSession } = trust;
    const active = await exchange();
    const reported = await actor(andrii);
    const other = await secondSession(andrii);
    await reported.page.goto("/main");
    await other.page.goto("/main");
    const { report } = await prepare(olha, "reports", {
      reportedUserId: andrii.id,
      reason: "Повторні порушення правил",
    });
    const moderator = await actor(admin);
    await moderator.page.goto(`/admin/reports/${report.id}`);
    await moderator.page.getByRole("button", { name: "Ухвалити рішення", exact: true }).click();
    const dialog = moderator.page.getByRole("dialog");
    await dialog.getByRole("combobox", { name: "Рішення", exact: true }).selectOption("SUSPEND");
    await dialog.getByLabel("Причина рішення").fill("Порушення підтверджені модератором");
    await dialog.getByRole("button", { name: "Зберегти рішення", exact: true }).click();
    await expect(
      moderator.page.getByRole("heading", { name: "Рішення за скаргою", exact: true }),
    ).toBeVisible();
    await moderator.page.goto("/admin?tab=history");
    const log = moderator.page.locator(".safety-list");
    await expect(log).toContainText("Ірина Модераторка");
    await expect(log).toContainText("Андрій Шевченко");
    await expect(log).toContainText("Порушення підтверджені модератором");
    expect(world.moderationActions[0].createdAt).toBeTruthy();
    await expect(log.locator("time")).toHaveAttribute(
      "datetime",
      world.moderationActions[0].createdAt,
    );
    for (const current of [reported, other]) {
      await current.page.goto("/main");
      await expect(
        current.page.getByRole("heading", { name: "Увійди, щоб продовжити", exact: true }),
      ).toBeVisible();
    }
    expect(world.exchanges.get(active.id).status).toBe("CANCELLED");
  });
});

test.describe("FR-15 — privacy and account acceptance gaps", () => {
  test("a wrong deletion password leaves the account and active exchange intact and allows cancelling the dialog", async ({
    trust,
  }) => {
    const { world, exchange, actor } = trust;
    const active = await exchange();
    const { page } = await actor();
    const dialog = await openDeletion(page, "IncorrectPassword2026");
    await dialog.getByRole("button", { name: "Підтвердити видалення", exact: true }).click();
    await expect(dialog.getByText("Пароль неправильний.", { exact: true })).toBeVisible();
    await expect(dialog.getByRole("checkbox")).toBeChecked();
    await expect(dialog.getByLabel("Поточний пароль")).toHaveValue("IncorrectPassword2026");
    expect(world.exchanges.get(active.id).status).toBe("ACTIVE");
    await dialog.getByRole("button", { name: "Залишити акаунт", exact: true }).click();
    await expect(page.getByRole("button", { name: "Видалити акаунт", exact: true })).toBeFocused();
    await page.goto(`/exchanges/${active.id}`);
    await expect(page.getByRole("region", { name: "Погоджені умови", exact: true })).toBeVisible();
    expect(mutations(world, "me/account")).toHaveLength(1);
  });

  test("account deletion revokes a second session, cancels future learning and anonymizes the partner's retained history", async ({
    trust,
  }) => {
    const { world, schedule, prepare, actor, olha, andrii, conversation, secondSession } = trust;
    const { request } = await prepare(andrii, "exchange-requests", {
      recipientId: olha.id,
      teachSkillId: IDS.skills.photoshop,
      learnSkillId: IDS.skills.guitar,
      format: "ONLINE",
      teachSessions: 1,
      teachDurationMinutes: 60,
      learnSessions: 1,
      learnDurationMinutes: 60,
    });
    const { exchange: active } = await prepare(olha, `exchange-requests/${request.id}/accept`);
    const lesson = [...world.lessons.values()].find((item) => item.exchangeId === active.id);
    await schedule(lesson, { title: "Майбутнє заняття після видалення" });
    const chat = conversation(active.requestId);
    const secretText = "Особисте повідомлення Ольги до видалення";
    await prepare(olha, `conversations/${chat.id}/messages`, { body: secretText });
    const other = await secondSession();
    await other.page.goto("/main");
    const sender = await actor();
    const dialog = await openDeletion(sender.page, DEMO_PASSWORD);
    await dialog.getByRole("button", { name: "Підтвердити видалення", exact: true }).click();
    await expect(sender.page).toHaveURL(/\/login\?deleted=1$/);
    await other.page.reload();
    await expect(
      other.page.getByRole("heading", { name: "Увійди, щоб продовжити", exact: true }),
    ).toBeVisible();
    const recipient = await actor(andrii);
    await recipient.page.goto(`/exchanges/${active.id}`);
    await expect(recipient.page.locator("#exchange-content")).toContainText("Видалений користувач");
    await expect(recipient.page.locator(".session-history > li")).toHaveCount(2);
    await expect(recipient.page.locator(".session-history")).toContainText("Видалений користувач");
    await expect(recipient.page.locator("#exchange-content")).not.toContainText("Ольга Коваленко");
    await expect(recipient.page.locator(".session-plan .session-status--cancelled")).toHaveCount(2);
    await recipient.page.goto(`/requests/${request.id}`);
    await expect(recipient.page.locator(".exchange-history")).toContainText("Видалений користувач");
    await expect(recipient.page.locator("#request-content")).not.toContainText("Ольга Коваленко");
    await recipient.page.goto(`/messages/${chat.id}`);
    await expect(recipient.page.locator(".chat-message__body")).toHaveText([
      "Повідомлення видалено",
    ]);
    await expect(recipient.page.locator(".chat-panel")).not.toContainText(secretText);
    await expect(recipient.page.locator(".chat-panel")).not.toContainText(olha.email);
    await sender.page.getByLabel("Університетська пошта").fill(olha.email);
    await sender.page.getByLabel("Пароль", { exact: true }).fill(DEMO_PASSWORD);
    await sender.page.getByRole("button", { name: "Увійти", exact: true }).click();
    await expect(sender.page.locator(".register-form__error")).toHaveText(
      "Невірна пошта або пароль.",
    );
    expect(world.exchanges.get(active.id).status).toBe("CANCELLED");
  });
});
