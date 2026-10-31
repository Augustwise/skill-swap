import { DEMO_PASSWORD, expect, mutations, test } from "./exchange-requirements-fixture.mjs";

async function openChat(scenario) {
  const pending = await scenario.request();
  const conversation = [...scenario.world.conversations.values()].find(
    (item) => item.requestId === pending.id,
  );
  const sender = await scenario.actor();
  await sender.page.goto(`/messages/${conversation.id}`);
  const composer = sender.page.getByLabel("Нове повідомлення, отримувач: Андрій");
  await expect(composer).toBeVisible();
  return { ...sender, conversation, composer, path: `conversations/${conversation.id}/messages` };
}

test.describe("FR-08 — private chat acceptance gaps", () => {
  test("a message of exactly 2000 Unicode code points is sent without truncation", async ({
    scenario,
  }) => {
    const { world } = scenario;
    const { page, conversation, composer, path } = await openChat(scenario);
    const text = "🎸".repeat(2000);
    await composer.fill(text);
    await expect(page.getByText("2000 / 2000", { exact: true })).toBeVisible();
    await page.getByRole("button", { name: "Надіслати", exact: true }).click();
    await expect(page.locator(".chat-message__body")).toHaveText([text]);
    expect(mutations(world, path).map((entry) => entry.body)).toEqual([{ body: text }]);
    expect(world.messages.get(conversation.id)).toHaveLength(1);
    await expect(composer).toHaveValue("");
  });

  test("chat history preserves text, author and timestamp after logout and a new login", async ({
    scenario,
  }) => {
    const { world, actor, olha, andrii } = scenario;
    const { page, conversation, composer } = await openChat(scenario);
    const text = "Домовленість про заняття 🎸";
    await composer.fill(text);
    await page.getByRole("button", { name: "Надіслати", exact: true }).click();
    await expect(page.locator(".chat-message__body")).toHaveText([text]);
    const message = world.messages.get(conversation.id)[0];
    await page.getByRole("button", { name: "Вийти", exact: true }).click();
    await expect(page).toHaveURL(/\/login$/);
    await page.getByLabel("Університетська пошта").fill(olha.email);
    await page.getByLabel("Пароль", { exact: true }).fill(DEMO_PASSWORD);
    await page.getByRole("button", { name: "Увійти", exact: true }).click();
    await expect(page).toHaveURL(/\/main$/);
    await page.goto(`/messages/${conversation.id}`);
    await expect(page.locator(".chat-message__body")).toHaveText([text]);
    const recipient = await actor(andrii);
    await recipient.page.goto(`/messages/${conversation.id}`);
    const received = recipient.page.locator(".chat-message").filter({ hasText: text });
    await expect(received).toContainText("Ольга Коваленко:");
    await expect(received.locator("time")).toHaveAttribute("datetime", message.createdAt);
  });

  test("a failed message keeps its draft, restores focus and retries without a duplicate", async ({
    scenario,
  }) => {
    const { world } = scenario;
    const { page, composer, path, conversation } = await openChat(scenario);
    const draft = "  Матеріали заняття\nhttps://example.test/notes  ";
    await composer.fill(draft);
    world.addFault({ method: "POST", path, kind: "network" });
    await page.getByRole("button", { name: "Надіслати", exact: true }).click();
    await expect(page.locator(".chat-composer").getByRole("alert")).toContainText(
      "Не вдалося зв’язатися із сервером",
    );
    await expect(composer).toHaveValue(draft);
    await expect(composer).toBeFocused();
    expect(world.messages.get(conversation.id)).toHaveLength(0);
    await page.getByRole("button", { name: "Надіслати", exact: true }).click();
    await expect(page.locator(".chat-message__body")).toHaveText([draft.trim()]);
    await expect(composer).toHaveValue("");
    expect(world.messages.get(conversation.id)).toHaveLength(1);
    expect(mutations(world, path)).toHaveLength(2);
  });

  test("a session rejected on send preserves the text and offers login back to the same conversation", async ({
    scenario,
  }) => {
    const { world } = scenario;
    const { page, composer, path, conversation } = await openChat(scenario);
    await composer.fill("Чернетка після завершення сесії");
    world.addFault({ method: "POST", path, status: 401, code: "session_required" });
    await composer.press("Enter");
    const error = page.locator(".chat-composer").getByRole("alert");
    await expect(error).toContainText("Сесія завершилася");
    await expect(error.getByRole("link", { name: "Увійти знову" })).toHaveAttribute(
      "href",
      `/login?next=${encodeURIComponent(`/messages/${conversation.id}`)}`,
    );
    await expect(composer).toHaveValue("Чернетка після завершення сесії");
    expect(world.messages.get(conversation.id)).toHaveLength(0);
  });

  test("the default five-second polling fetches a new partner message without reloading the page", async ({
    scenario,
  }) => {
    const { world, request, prepare, andrii, actor } = scenario;
    const pending = await request();
    const conversation = [...world.conversations.values()].find(
      (item) => item.requestId === pending.id,
    );
    const path = `conversations/${conversation.id}/messages`;
    const initial = await prepare(andrii, path, { body: "Перше повідомлення" });
    const { page } = await actor();
    await page.goto(`/messages/${conversation.id}`);
    await expect(page.locator(".chat-message__body")).toHaveText(["Перше повідомлення"]);
    const documentRequests = [];
    page.on("request", (entry) => {
      if (entry.isNavigationRequest() && entry.frame() === page.mainFrame())
        documentRequests.push(entry.url());
    });
    await prepare(andrii, path, { body: "Нове повідомлення від партнера" });
    await expect(
      page
        .locator(".chat-message__body")
        .getByText("Нове повідомлення від партнера", { exact: true }),
    ).toBeVisible({ timeout: 10_000 });
    expect(world.config.pollIntervalSeconds).toBe(5);
    expect(
      world.requestLog.some(
        (entry) =>
          entry.method === "GET" &&
          entry.path === path &&
          new URLSearchParams(entry.search).get("after") === initial.message.id,
      ),
    ).toBe(true);
    expect(documentRequests).toEqual([]);
  });

  test("an older-history failure keeps current messages and can be retried without duplicating history", async ({
    scenario,
  }) => {
    const { world, request, prepare, andrii, actor } = scenario;
    const pending = await request();
    const conversation = [...world.conversations.values()].find(
      (item) => item.requestId === pending.id,
    );
    const path = `conversations/${conversation.id}/messages`;
    for (let i = 1; i <= 40; i++) await prepare(andrii, path, { body: `Повідомлення ${i}` });
    const { page } = await actor();
    await page.goto(`/messages/${conversation.id}`);
    await expect(
      page.locator(".chat-message__body").getByText("Повідомлення 40", { exact: true }),
    ).toBeVisible();
    await expect(
      page.locator(".chat-message__body").getByText("Повідомлення 1", { exact: true }),
    ).toHaveCount(0);
    world.addFault({ method: "GET", path, status: 503 });
    const earlier = page.getByRole("button", { name: "Показати попередні повідомлення" });
    await earlier.click();
    await expect(
      page.getByText("Не вдалося завантажити попередні повідомлення. Спробуй ще раз."),
    ).toBeVisible();
    await expect(
      page.locator(".chat-message__body").getByText("Повідомлення 40", { exact: true }),
    ).toBeVisible();
    await earlier.click();
    await expect(page.locator(".chat-message__body")).toHaveCount(40);
    await expect(
      page.locator(".chat-message__body").getByText("Повідомлення 1", { exact: true }),
    ).toHaveCount(1);
    await expect(
      page.locator(".chat-message__body").getByText("Повідомлення 40", { exact: true }),
    ).toHaveCount(1);
    await expect(earlier).toHaveCount(0);
  });
});
