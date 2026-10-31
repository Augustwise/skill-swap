import { expect, IDS, mutations, openProposal, test } from "./exchange-requirements-fixture.mjs";

test.describe("FR-06 — request creation acceptance gaps", () => {
  test("an offline request preserves both commitments and the 500-character message in both inboxes", async ({
    scenario,
  }) => {
    const { world, actor, olha, andrii } = scenario;
    const sender = await actor(olha);
    const recipient = await actor(andrii);
    const dialog = await openProposal(sender.page, andrii.id);
    const message = "Ї".repeat(500);
    await dialog.getByRole("radio", { name: /Офлайн/ }).check();
    await dialog.getByRole("checkbox", { name: "Однакові умови в обидва боки" }).uncheck();
    await dialog.getByLabel("Ти проводиш: Занять").selectOption("2");
    await dialog.getByLabel("Ти проводиш: Тривалість").selectOption("60");
    await dialog.getByLabel("Андрій проводить: Занять").selectOption("4");
    await dialog.getByLabel("Андрій проводить: Тривалість").selectOption("30");
    await dialog.getByLabel(/Повідомлення/).fill(message);
    await dialog.getByRole("button", { name: "Надіслати запит" }).click();
    await expect(dialog.getByRole("heading", { name: "Запит надіслано" })).toBeVisible();
    expect(mutations(world, "exchange-requests").map((entry) => entry.body)).toEqual([
      {
        recipientId: andrii.id,
        teachSkillId: IDS.skills.guitar,
        learnSkillId: IDS.skills.photoshop,
        format: "OFFLINE",
        teachSessions: 2,
        teachDurationMinutes: 60,
        learnSessions: 4,
        learnDurationMinutes: 30,
        message,
      },
    ]);
    const requestId = [...world.requests.keys()][0];
    for (const [current, direction, partnerName] of [
      [sender, "outgoing", "Андрій Шевченко"],
      [recipient, "incoming", "Ольга Коваленко"],
    ]) {
      await current.page.goto(`/requests?direction=${direction}&status=PENDING`);
      const card = current.page.locator(".exchange-grid > li");
      await expect(card).toHaveCount(1);
      await expect(card).toContainText(partnerName);
      await expect(card).toContainText("Гітара");
      await expect(card).toContainText("Photoshop");
      await current.page.goto(`/requests/${requestId}`);
      await expect(current.page.locator("#request-content")).toContainText(message);
      await expect(current.page.locator("#request-content")).toContainText("Офлайн");
      await expect(current.page.locator("#request-content")).toContainText("6 занять");
    }
  });

  for (const [list, field] of [
    ["learningSkills", "teachSkillId"],
    ["teachingSkills", "learnSkillId"],
  ]) {
    test(`a changed partner ${list} list is explained without losing the request draft`, async ({
      scenario,
    }) => {
      const { world, actor, andrii } = scenario;
      const { page } = await actor();
      const dialog = await openProposal(page, andrii.id);
      await dialog.getByLabel(/Повідомлення/).fill("Збережи цю чернетку");
      const original = andrii[list];
      andrii[list] = [];
      await dialog.getByRole("button", { name: "Надіслати запит" }).click();
      await expect(
        dialog.getByText("Ця навичка вже не підходить для обміну. Онови сторінку й обери іншу."),
      ).toBeVisible();
      await expect(dialog.getByLabel(/Повідомлення/)).toHaveValue("Збережи цю чернетку");
      const skill = dialog.getByRole("combobox", {
        name: field === "teachSkillId" ? "Ти навчиш" : "Андрій навчить тебе",
        exact: true,
      });
      await expect(skill).toHaveAttribute("aria-invalid", "true");
      expect(world.requests.size).toBe(0);
      andrii[list] = original;
      await dialog.getByRole("button", { name: "Надіслати запит" }).click();
      await expect(dialog.getByRole("heading", { name: "Запит надіслано" })).toBeVisible();
    });
  }

  test("a format removed while the proposal is open produces a field error and preserves the plan", async ({
    scenario,
  }) => {
    const { world, actor, andrii } = scenario;
    const { page } = await actor();
    const dialog = await openProposal(page, andrii.id);
    await dialog.getByRole("radio", { name: /Офлайн/ }).check();
    await dialog.getByLabel("Ти проводиш: Занять").selectOption("3");
    andrii.formats = ["ONLINE"];
    await dialog.getByRole("button", { name: "Надіслати запит" }).click();
    await expect(
      dialog.getByText("Цей формат більше не спільний для вас. Онови сторінку."),
    ).toBeVisible();
    await expect(dialog.getByLabel("Ти проводиш: Занять")).toHaveValue("3");
    expect(world.requests.size).toBe(0);
    await dialog.getByRole("radio", { name: "Онлайн", exact: true }).check();
    await dialog.getByRole("button", { name: "Надіслати запит" }).click();
    await expect(dialog.getByRole("heading", { name: "Запит надіслано" })).toBeVisible();
    expect([...world.requests.values()][0].format).toBe("ONLINE");
  });

  test("a pending submission prevents repeated form submits and creates one request", async ({
    scenario,
  }) => {
    const { world, actor, andrii } = scenario;
    const { page } = await actor();
    const dialog = await openProposal(page, andrii.id);
    let release;
    const hold = new Promise((resolve) => {
      release = resolve;
    });
    world.addFault({ method: "POST", path: "exchange-requests", kind: "hold", hold });
    try {
      await dialog.getByRole("button", { name: "Надіслати запит" }).click();
      await expect(dialog.getByRole("button", { name: "Надсилаємо…" })).toBeDisabled();
      await dialog.locator("form").evaluate((form) => {
        form.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
        form.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
      });
      expect(mutations(world, "exchange-requests")).toHaveLength(1);
      release();
      await expect(dialog.getByRole("heading", { name: "Запит надіслано" })).toBeVisible();
      expect(world.requests.size).toBe(1);
      expect(mutations(world, "exchange-requests")).toHaveLength(1);
    } finally {
      release();
    }
  });
});

test.describe("FR-07 — request decision acceptance gaps", () => {
  test("author and recipient see only their own actions and a stranger sees no request", async ({
    scenario,
  }) => {
    const { actor, olha, andrii, stranger, request } = scenario;
    const pending = await request();
    for (const [user, allowed, forbidden] of [
      [olha, ["Відкликати"], ["Прийняти", "Відхилити"]],
      [andrii, ["Прийняти", "Відхилити"], ["Відкликати"]],
    ]) {
      const { page } = await actor(user);
      await page.goto(`/requests/${pending.id}`);
      for (const name of allowed)
        await expect(page.getByRole("button", { name, exact: true })).toBeVisible();
      for (const name of forbidden)
        await expect(page.getByRole("button", { name, exact: true })).toHaveCount(0);
    }
    const { page } = await actor(stranger);
    await page.goto(`/requests/${pending.id}`);
    await expect(page.getByText("Запит не знайдено.", { exact: true })).toBeVisible();
    await expect(page.getByRole("button", { name: "Прийняти", exact: true })).toHaveCount(0);
  });

  for (const [action, label, confirmation, status, notice] of [
    ["decline", "Відхилити", "Відхилити запит", "DECLINED", "Запит відхилено."],
    ["withdraw", "Відкликати", "Відкликати запит", "WITHDRAWN", "Запит відкликано."],
  ]) {
    test(`cancelling ${action} leaves the request pending and confirming preserves its final history`, async ({
      scenario,
    }) => {
      const { world, actor, olha, andrii, request } = scenario;
      const pending = await request();
      const { page } = await actor(action === "withdraw" ? olha : andrii);
      await page.goto(`/requests/${pending.id}`);
      await page.getByRole("button", { name: label, exact: true }).click();
      await page
        .getByRole("dialog")
        .getByRole("button", { name: "Скасувати", exact: true })
        .click();
      await expect(page.getByRole("dialog")).toHaveCount(0);
      await expect(page.getByRole("button", { name: label, exact: true })).toBeFocused();
      expect(mutations(world, `exchange-requests/${pending.id}/${action}`)).toHaveLength(0);
      expect(world.requests.get(pending.id).status).toBe("PENDING");
      await page.getByRole("button", { name: label, exact: true }).click();
      await page
        .getByRole("dialog")
        .getByRole("button", { name: confirmation, exact: true })
        .click();
      await expect(page.getByText(notice, { exact: true })).toBeVisible();
      await page.reload();
      await expect(page.locator(".exchange-history > li")).toHaveCount(2);
      for (const name of ["Прийняти", "Відхилити", "Відкликати"]) {
        await expect(page.getByRole("button", { name, exact: true })).toHaveCount(0);
      }
      expect(world.requests.get(pending.id).status).toBe(status);
      expect(world.exchanges.size).toBe(0);
    });
  }

  test("accepting a request withdrawn in another session refreshes its state without creating an exchange", async ({
    scenario,
  }) => {
    const { world, actor, olha, andrii, request, prepare } = scenario;
    const pending = await request();
    const { page } = await actor(andrii);
    await page.goto(`/requests/${pending.id}`);
    await expect(page.getByRole("button", { name: "Прийняти", exact: true })).toBeVisible();
    await prepare(olha, `exchange-requests/${pending.id}/withdraw`);
    await page.getByRole("button", { name: "Прийняти", exact: true }).click();
    await expect(page.getByText("На запит уже відповіли. Ми оновили його стан.")).toBeVisible();
    await expect(page.locator(".exchange-history")).toContainText("Запит відкликано");
    await expect(page.getByRole("button", { name: "Прийняти", exact: true })).toHaveCount(0);
    expect(world.exchanges.size).toBe(0);
  });

  test("an accept failure can be retried and the resulting exchange contains the complete agreed plan", async ({
    scenario,
  }) => {
    const { world, actor, andrii, request } = scenario;
    const pending = await request({ teachSessions: 2, learnSessions: 4, learnDurationMinutes: 30 });
    const { page } = await actor(andrii);
    world.addFault({
      method: "POST",
      path: `exchange-requests/${pending.id}/accept`,
      kind: "network",
    });
    await page.goto(`/requests/${pending.id}`);
    await page.getByRole("button", { name: "Прийняти", exact: true }).click();
    await expect(page.locator(".exchange-error")).toContainText(
      "Не вдалося зв’язатися із сервером",
    );
    expect(world.exchanges.size).toBe(0);
    await page.getByRole("button", { name: "Прийняти", exact: true }).click();
    await expect(page).toHaveURL(/\/exchanges\/[0-9a-f-]+\?started=1$/);
    await expect(
      page.getByRole("region", { name: "План занять", exact: true }).locator("li"),
    ).toHaveCount(6);
    await expect(page.locator(".exchange-progress__percent")).toHaveText("0%");
    expect(world.exchanges.size).toBe(1);
    await page.goto(`/requests/${pending.id}`);
    await expect(page.locator(".exchange-history")).toContainText("Запит прийнято");
    await expect(page.getByRole("link", { name: "Перейти до обміну" })).toBeVisible();
    await expect(page.getByRole("button", { name: "Прийняти", exact: true })).toHaveCount(0);
  });
});
