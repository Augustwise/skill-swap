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

  test("a failed block preserves the relationship and retry makes old chat history read-only for both students", async ({
    trust,
  }) => {
    const { world, request, conversation, prepare, actor, olha, andrii } = trust;
    const pending = await request();
    const chat = conversation(pending.id);
    await prepare(olha, `conversations/${chat.id}/messages`, {
      body: "Збережена історія до блокування",
    });
    const sender = await actor();
    await sender.page.goto(`/students/${andrii.id}`);
    await sender.page.getByRole("button", { name: "Заблокувати", exact: true }).click();
    const dialog = sender.page.getByRole("dialog");
    world.addFault({ method: "POST", path: `students/${andrii.id}/block`, kind: "network" });
    await dialog.getByRole("button", { name: "Підтвердити блокування", exact: true }).click();
    await expect(dialog.getByRole("alert")).toBeVisible();
    expect(world.blocks).toHaveLength(0);
    expect(world.requests.get(pending.id).status).toBe("PENDING");
    await dialog.getByRole("button", { name: "Підтвердити блокування", exact: true }).click();
    await expect(sender.page).toHaveURL(/\/settings\?tab=blocks$/);
    for (const user of [olha, andrii]) {
      const { page } = await actor(user);
      await page.goto(`/messages/${chat.id}`);
      await expect(page.locator(".chat-message__body")).toHaveText([
        "Збережена історія до блокування",
      ]);
      await expect(page.locator(".chat-blocked")).toBeVisible();
      await expect(page.getByRole("textbox", { name: /^Нове повідомлення/ })).toHaveCount(0);
    }
  });

  test("a report is visible to its author and does not automatically block the reported student", async ({
    trust,
  }) => {
    const { world, actor, olha, andrii, stranger } = trust;
    const { page } = await actor(olha);
    const reason = "Прошу перевірити поведінку під час заняття";
    await page.goto(`/students/${andrii.id}`);
    await page.getByRole("button", { name: "Поскаржитися", exact: true }).click();
    const dialog = page.getByRole("dialog");
    await dialog.getByLabel("Причина скарги").fill(reason);
    await dialog.getByRole("button", { name: "Надіслати скаргу", exact: true }).click();
    await expect(dialog).toHaveCount(0);
    await page.goto("/settings?tab=reports");
    await expect(page.locator(".safety-list")).toContainText(reason);
    expect(world.blocks).toHaveLength(0);
    for (const user of [andrii, stranger]) {
      const current = await actor(user);
      await current.page.goto("/settings?tab=reports");
      await expect(
        current.page.getByRole("heading", { name: "Ти ще не надсилав скарг", exact: true }),
      ).toBeVisible();
      await expect(current.page.locator("main")).not.toContainText(reason);
    }
  });

  test("hiding a catalog skill removes it from new selections while an agreed exchange keeps its snapshot", async ({
    trust,
  }) => {
    const { exchange, actor, admin, stranger } = trust;
    const active = await exchange();
    const student = await actor(stranger);
    await student.page.goto("/profile?tab=skills");
    await student.page
      .getByRole("button", { name: "+ Додати навичку для вивчення", exact: true })
      .click();
    let picker = student.page.getByRole("form", { name: "Додати навичку", exact: true });
    await expect(picker.locator(`option[value="${IDS.skills.guitar}"]`)).toHaveCount(1);
    await picker.getByRole("button", { name: "Скасувати", exact: true }).click();
    const moderator = await actor(admin);
    await moderator.page.goto("/admin?tab=skills");
    await moderator.page.getByRole("button", { name: "Редагувати Гітара", exact: true }).click();
    const dialog = moderator.page.getByRole("dialog");
    await dialog.getByLabel("Показувати навичку в каталозі").uncheck();
    await dialog.getByRole("button", { name: "Зберегти навичку", exact: true }).click();
    await expect(dialog).toHaveCount(0);
    await student.page
      .getByRole("button", { name: "+ Додати навичку для вивчення", exact: true })
      .click();
    picker = student.page.getByRole("form", { name: "Додати навичку", exact: true });
    await expect(
      picker.getByRole("combobox", { name: "Навичка зі спільного каталогу", exact: true }),
    ).toBeVisible();
    await expect(picker.locator(`option[value="${IDS.skills.guitar}"]`)).toHaveCount(0);
    const sender = await actor();
    await sender.page.goto(`/exchanges/${active.id}`);
    await expect(
      sender.page.getByRole("region", { name: "Погоджені умови", exact: true }),
    ).toContainText("Гітара");
    await expect(
      sender.page.getByRole("region", { name: "План занять", exact: true }),
    ).toContainText("Гітара");
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
  test("hiding a profile removes it from discovery while its partner retains the active exchange, lesson and chat", async ({
    trust,
  }) => {
    const { exchange, schedule, prepare, conversation, actor, olha, andrii } = trust;
    const active = await exchange();
    const lesson = await schedule(active.lessons[0]);
    const chat = conversation(active.requestId);
    await prepare(olha, `conversations/${chat.id}/messages`, {
      body: "Домовленість у чинному обміні",
    });
    const sender = await actor();
    await sender.page.goto("/settings");
    await sender.page
      .getByRole("switch", { name: "Показувати мій профіль", exact: false })
      .uncheck();
    await sender.page.getByRole("button", { name: "Зберегти налаштування", exact: true }).click();
    await expect(
      sender.page.getByText("Налаштування приватності збережено.", { exact: true }),
    ).toBeVisible();
    const recipient = await actor(andrii);
    await recipient.page.goto("/search?q=Ольга");
    await expect(
      recipient.page.getByText("За запитом «Ольга» нікого не знайдено.", { exact: true }),
    ).toBeVisible();
    await recipient.page.goto(`/students/${olha.id}`);
    await expect(recipient.page.getByText("Профіль недоступний.", { exact: true })).toBeVisible();
    await recipient.page.goto(`/exchanges/${active.id}`);
    await expect(
      recipient.page.getByRole("region", { name: "Погоджені умови", exact: true }),
    ).toBeVisible();
    await recipient.page.goto(`/sessions/${lesson.id}`);
    await expect(recipient.page.locator(".session-status--scheduled")).toBeVisible();
    await recipient.page.goto(`/messages/${chat.id}`);
    await expect(recipient.page.locator(".chat-message__body")).toHaveText([
      "Домовленість у чинному обміні",
    ]);
    await expect(
      recipient.page.getByRole("textbox", {
        name: "Нове повідомлення, отримувач: Ольга",
        exact: true,
      }),
    ).toBeVisible();
  });

  test("changing a password keeps the current session active, revokes another session and replaces the login password", async ({
    trust,
  }) => {
    const { actor, olha, secondSession } = trust;
    const current = await actor();
    const other = await secondSession();
    await other.page.goto("/main");
    await current.page.goto("/settings?tab=security");
    const password = "NewSecureSkillSwap2026";
    await current.page.getByLabel("Поточний пароль", { exact: true }).fill(DEMO_PASSWORD);
    await current.page.getByLabel("Новий пароль", { exact: true }).fill(password);
    await current.page.getByLabel("Повтори новий пароль", { exact: true }).fill(password);
    await current.page.getByRole("button", { name: "Змінити пароль", exact: true }).click();
    await expect(
      current.page.getByText("Пароль змінено. Інші сесії завершено.", { exact: true }),
    ).toBeVisible();
    await current.page.goto("/main");
    await expect(
      current.page.getByRole("heading", { name: "Привіт, Ольга!", exact: true }),
    ).toBeVisible();
    await other.page.reload();
    await expect(
      other.page.getByRole("heading", { name: "Увійди, щоб продовжити", exact: true }),
    ).toBeVisible();
    await other.page.goto("/login");
    await other.page.getByLabel("Університетська пошта").fill(olha.email);
    await other.page.getByLabel("Пароль", { exact: true }).fill(DEMO_PASSWORD);
    await other.page.getByRole("button", { name: "Увійти", exact: true }).click();
    await expect(other.page.locator(".register-form__error")).toHaveText(
      "Невірна пошта або пароль.",
    );
    await other.page.getByLabel("Пароль", { exact: true }).fill(password);
    await other.page.getByRole("button", { name: "Увійти", exact: true }).click();
    await expect(other.page).toHaveURL(/\/main$/);
  });

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

  test("another student sees neither email nor the exact meeting place and cannot open the private lesson", async ({
    trust,
  }) => {
    const { exchange, schedule, actor, olha, stranger } = trust;
    const active = await exchange({ format: "OFFLINE" });
    const place = "Секретна кімната 412, другий вхід";
    const lesson = await schedule(active.lessons[0], {
      format: "OFFLINE",
      meetingPlace: place,
      meetingUrl: "",
    });
    const outsider = await actor(stranger);
    await outsider.page.goto(`/students/${olha.id}`);
    await expect(
      outsider.page.getByRole("heading", { name: "Ольга Коваленко", exact: true }),
    ).toBeVisible();
    const profile = outsider.page.locator("#student-content");
    await expect(profile).not.toContainText(olha.email);
    await expect(profile).not.toContainText(place);
    await expect(profile.locator('a[href^="mailto:"]')).toHaveCount(0);
    await outsider.page.goto(`/sessions/${lesson.id}`);
    await expect(outsider.page.getByText("Заняття не знайдено.", { exact: true })).toBeVisible();
    await expect(outsider.page.locator("main")).not.toContainText(place);
  });
});
