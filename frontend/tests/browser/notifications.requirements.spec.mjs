import { expect, mutations, test } from "./trust-requirements-fixture.mjs";

test.describe("FR-13 — notification acceptance gaps", () => {
  test("a failed read keeps the notification unread and a retry updates the feed and bell", async ({
    trust,
  }) => {
    const { world, request, actor, andrii } = trust;
    await request();
    const notification = [...world.notifications.values()].find(
      (item) => item.recipientId === andrii.id,
    );
    const path = `me/notifications/${notification.id}/read`;
    const { page } = await actor(andrii);
    await page.goto("/notifications?filter=unread");
    const feed = page.locator(".notification-feed");
    await expect(feed.locator(".notification-item")).toHaveCount(1);
    world.addFault({ method: "POST", path, status: 503 });
    await feed.getByRole("button", { name: /^Позначити прочитаним:/ }).click();
    await expect(feed.getByRole("alert")).toBeVisible();
    await expect(
      page.getByRole("button", { name: "Сповіщення: 1 непрочитаних", exact: true }),
    ).toBeVisible();
    expect(notification.isRead).toBe(false);
    await feed.getByRole("button", { name: /^Позначити прочитаним:/ }).click();
    await expect(feed.getByText("Непрочитаних сповіщень немає.", { exact: true })).toBeVisible();
    await expect(page.getByRole("button", { name: "Сповіщення", exact: true })).toBeVisible();
    expect(notification.isRead).toBe(true);
    expect(mutations(world, path)).toHaveLength(2);
    await feed.getByRole("link", { name: "Показати всі", exact: true }).click();
    await expect(feed.locator(".notification-item")).toHaveCount(1);
    await expect(feed.getByRole("button", { name: /^Позначити прочитаним:/ })).toHaveCount(0);
  });

  test("read-all retries a failure, disables duplicate actions while pending and leaves another recipient's events unread", async ({
    trust,
  }) => {
    const { world, request, prepare, conversation, actor, olha, andrii } = trust;
    const pending = await request();
    const chat = conversation(pending.id);
    await prepare(olha, `conversations/${chat.id}/messages`, { body: "Повідомлення для Андрія" });
    await prepare(andrii, `conversations/${chat.id}/messages`, {
      body: "Окреме повідомлення для Ольги",
    });
    const foreign = [...world.notifications.values()].filter(
      (item) => item.recipientId === olha.id,
    );
    const { page } = await actor(andrii);
    await page.goto("/notifications?filter=unread");
    const feed = page.locator(".notification-feed");
    await expect(feed.locator(".notification-item")).toHaveCount(2);
    const path = "me/notifications/read-all";
    world.addFault({ method: "POST", path, kind: "network" });
    await feed.getByRole("button", { name: "Позначити всі прочитаними", exact: true }).click();
    await expect(feed.getByRole("alert")).toBeVisible();
    let release;
    const hold = new Promise((resolve) => {
      release = resolve;
    });
    world.addFault({ method: "POST", path, kind: "hold", hold });
    try {
      await feed.getByRole("button", { name: "Позначити всі прочитаними", exact: true }).click();
      await expect(feed.getByRole("button", { name: "Позначаємо…", exact: true })).toBeDisabled();
      const itemButtons = feed.getByRole("button", { name: /^Позначити прочитаним:/ });
      await expect(itemButtons).toHaveCount(2);
      for (const button of await itemButtons.all()) await expect(button).toBeDisabled();
      release();
      await expect(feed.getByText("Непрочитаних сповіщень немає.", { exact: true })).toBeVisible();
      await expect(page.getByRole("button", { name: "Сповіщення", exact: true })).toBeVisible();
      expect(foreign.length).toBeGreaterThan(0);
      expect(foreign.every((item) => !item.isRead)).toBe(true);
      expect(mutations(world, path)).toHaveLength(2);
    } finally {
      release();
    }
  });

  for (const kind of ["decision", "message", "session"]) {
    test(`a ${kind} notification opens its exact object and marks only the addressed event read`, async ({
      trust,
    }) => {
      const { world, request, exchange, prepare, proposal, conversation, actor, olha, andrii } =
        trust;
      let user;
      let type;
      let href;
      let heading;
      if (kind === "decision") {
        const pending = await request();
        await prepare(andrii, `exchange-requests/${pending.id}/decline`);
        user = olha;
        type = "REQUEST_RESPONSE";
        href = `/requests/${pending.id}`;
        heading = "Статус запиту";
      } else if (kind === "message") {
        const pending = await request();
        const chat = conversation(pending.id);
        await prepare(olha, `conversations/${chat.id}/messages`, {
          body: "Повідомлення зі сповіщення",
        });
        user = andrii;
        type = "CHAT_MESSAGE";
        href = `/messages/${chat.id}`;
        heading = "Ольга Коваленко";
      } else {
        const active = await exchange();
        await proposal(active.lessons[0]);
        user = andrii;
        type = "SESSION_UPDATED";
        href = `/sessions/${active.lessons[0].id}`;
        heading = "Основи акордів";
      }
      const notification = [...world.notifications.values()].find(
        (item) => item.recipientId === user.id && item.eventType === type,
      );
      expect(notification).toBeTruthy();
      const foreign = [...world.notifications.values()].filter(
        (item) => item.recipientId !== user.id && !item.isRead,
      );
      const { page } = await actor(user);
      await page.goto("/notifications?filter=unread");
      await page.locator(`.notification-feed .notification-item__link[href="${href}"]`).click();
      await expect(page).toHaveURL(new URL(href, page.url()).href);
      await expect(page.getByRole("heading", { name: heading, exact: true })).toBeVisible();
      expect(notification.isRead).toBe(true);
      expect(notification.readAt).toBeTruthy();
      expect(foreign.every((item) => !item.isRead)).toBe(true);
    });
  }

  test("unread pagination preserves its filter and displays 21 real events without mixing recipients", async ({
    trust,
  }) => {
    const { world, request, conversation, prepare, actor, olha, andrii } = trust;
    const pending = await request();
    const chat = conversation(pending.id);
    for (let i = 1; i <= 20; i++) {
      await prepare(olha, `conversations/${chat.id}/messages`, { body: `Подія для Андрія ${i}` });
    }
    await prepare(andrii, `conversations/${chat.id}/messages`, { body: "Чужа подія для Ольги" });
    const { page } = await actor(andrii);
    await page.goto("/notifications?filter=unread");
    const feed = page.locator(".notification-feed");
    await expect(feed.locator(".notification-item")).toHaveCount(20);
    await expect(feed).toContainText("21 сповіщення · непрочитаних: 21");
    await expect(feed).not.toContainText("Чужа подія для Ольги");
    await page.getByRole("link", { name: "Наступна →", exact: true }).click();
    await expect(page).toHaveURL(/\/notifications\?filter=unread&page=2$/);
    await expect(feed.locator(".notification-item")).toHaveCount(1);
    await page.reload();
    await expect(feed.locator(".notification-item")).toHaveCount(1);
    const secondPage = world.requestLog.find(
      (entry) =>
        entry.path === "me/notifications" && new URLSearchParams(entry.search).get("page") === "2",
    );
    expect(new URLSearchParams(secondPage.search).get("unreadOnly")).toBe("true");
    await page.getByRole("link", { name: "Попередня", exact: false }).click();
    await expect(feed.locator(".notification-item")).toHaveCount(20);
  });
});
