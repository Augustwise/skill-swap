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
});
