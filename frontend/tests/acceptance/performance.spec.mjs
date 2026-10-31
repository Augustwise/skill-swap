import { randomUUID } from "node:crypto";
import { expect, test } from "../helpers/browser-version.mjs";
import { assertPageBudget, measurePageLoads } from "../helpers/performance.mjs";

const password = "SkillSwapDemo2026";
const andriiId = "60000000-0000-0000-0000-000000000002";

async function login(page, name) {
  await page.goto("/login");
  await page.getByLabel("Університетська пошта").fill(`demo.${name}@students.example.test`);
  await page.getByLabel("Пароль", { exact: true }).fill(password);
  await page.getByRole("button", { name: "Увійти", exact: true }).click();
  await expect(page).toHaveURL(/\/main$/);
}

test("30 unthrottled production page loads include real API data and meet the 3-second p95", async ({
  page,
}, testInfo) => {
  await login(page, "olha");
  const samples = await measurePageLoads(page, [
    {
      path: "/main",
      ready: async (page) => {
        await expect(page.getByRole("heading", { name: "Привіт, Ольга!" })).toBeVisible();
        await expect(page.locator("#dashboard-matches")).toContainText("Андрій Коваль");
        await expect(page.locator('main [aria-busy="true"]')).toHaveCount(0);
      },
    },
    {
      path: "/search?q=Photoshop",
      ready: async (page) => {
        await expect(page.getByRole("heading", { name: "Андрій Коваль" })).toBeVisible();
        await expect(page.locator(".discovery-results")).toHaveAttribute("aria-busy", "false");
        await expect(page.getByRole("combobox", { name: /^Категорія/ })).toBeEnabled();
      },
    },
    {
      path: "/profile",
      ready: async (page) => {
        await expect(page.getByLabel("Ім’я", { exact: true })).toHaveValue("Ольга");
        await expect(
          page.getByText("demo.olha@students.example.test", { exact: true }),
        ).toBeVisible();
      },
    },
  ]);
  await assertPageBudget(samples, testInfo, { api: "real Go + PostgreSQL", mode: "next start" });
});

test("a real partner message appears in the other browser context within 10 seconds without navigation", async ({
  browser,
}, testInfo) => {
  const sender = await browser.newContext({ baseURL: "http://localhost:3000" });
  const recipient = await browser.newContext({ baseURL: "http://localhost:3000" });
  const senderPage = await sender.newPage();
  let requestPath;
  try {
    const recipientPage = await recipient.newPage();
    await login(senderPage, "olha");
    await login(recipientPage, "andrii");
    await senderPage.goto(`/students/${andriiId}`);
    await senderPage.getByRole("button", { name: "Запропонувати обмін" }).click();
    await senderPage
      .getByRole("dialog")
      .getByRole("radio", { name: "Онлайн", exact: true })
      .check();
    await senderPage.getByRole("dialog").getByRole("button", { name: "Надіслати запит" }).click();
    await senderPage.getByRole("dialog").getByRole("link", { name: "Переглянути запит" }).click();
    await expect(senderPage).toHaveURL(/\/requests\/[0-9a-f-]+$/);
    requestPath = new URL(senderPage.url()).pathname;
    await senderPage.getByRole("link", { name: "Написати повідомлення" }).click();
    await expect(senderPage).toHaveURL(/\/messages\/[0-9a-f-]+$/);
    const chatPath = new URL(senderPage.url()).pathname;
    await recipientPage.goto(chatPath);
    const composer = senderPage.getByLabel("Нове повідомлення, отримувач: Андрій");
    await expect(recipientPage.getByLabel("Нове повідомлення, отримувач: Ольга")).toBeVisible();
    const navigations = [];
    recipientPage.on("request", (request) => {
      if (request.isNavigationRequest() && request.frame() === recipientPage.mainFrame())
        navigations.push(request.url());
    });
    const body = `Перевірка доставки ${randomUUID()}`;
    await composer.fill(body);
    const started = performance.now();
    await composer.press("Enter");
    await expect(
      recipientPage.locator(".chat-message__body").getByText(body, { exact: true }),
    ).toBeVisible({ timeout: 10_000 });
    const deliveryMs = performance.now() - started;
    await testInfo.attach("chat-delivery-measurement", {
      body: JSON.stringify({ api: "real Go + PostgreSQL", deliveryMs, contexts: 2 }),
      contentType: "application/json",
    });
    expect(deliveryMs).toBeLessThanOrEqual(10_000);
    expect(navigations).toEqual([]);
  } finally {
    try {
      if (requestPath) {
        await senderPage.goto(requestPath);
        await senderPage.getByRole("button", { name: "Відкликати", exact: true }).click();
        await senderPage
          .getByRole("dialog")
          .getByRole("button", { name: "Відкликати запит", exact: true })
          .click();
        await expect(senderPage.getByText("Запит відкликано.", { exact: true })).toBeVisible();
      }
    } finally {
      await sender.close();
      await recipient.close();
    }
  }
});
