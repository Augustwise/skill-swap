import { expect, test } from "@playwright/test";

const exchangeId = "80000000-0000-0000-0000-000000000001";
const sessionId = "90000000-0000-0000-0000-000000000001";
const person = {
  id: exchangeId,
  firstName: "Ольга",
  lastName: "Тест",
  university: { name: "КПІ" },
  city: "Київ",
};
const partner = { ...person, id: sessionId, firstName: "Андрій" };
const terms = {
  skill: { name: "Гітара" },
  teacherLevel: "ADVANCED",
  learnerLevel: "BEGINNER",
  sessions: 2,
  durationMinutes: 60,
};

async function fixture(page) {
  const lesson = {
    id: sessionId,
    exchangeId,
    planOrder: 1,
    title: "Перші акорди",
    skill: terms.skill,
    teacher: person,
    learner: partner,
    partner,
    role: "TEACHER",
    status: "SCHEDULED",
    format: "ONLINE",
    meetingUrl: "https://meet.example.com/lesson",
    startsAt: "2099-05-12T09:00:00Z",
    endsAt: "2099-05-12T10:00:00Z",
    plannedDurationMinutes: 60,
    canCancel: true,
  };
  const progress = {
    completedSessions: 0,
    totalSessions: 4,
    percent: 0,
    requesterCompletedSessions: 0,
    requesterPlannedSessions: 2,
    recipientCompletedSessions: 0,
    recipientPlannedSessions: 2,
  };
  const state = { lesson, progress, failed: false, expired: false };
  await page.route("**/api/v1/**", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const path = url.pathname.replace("/api/v1/", "");
    const send = (json, status = 200) => route.fulfill({ status, json });
    if (state.expired && request.method() === "POST")
      return send({ error: { code: "unauthenticated" } }, 401);
    if (path === "auth/me") return send({ user: { emailVerified: true }, csrfToken: "test-csrf" });
    if (path === "me/profile")
      return send({
        profile: {
          ...person,
          course: 3,
          teachingSkills: [],
          learningSkills: [],
          formats: ["ONLINE"],
        },
      });
    if (path === `exchanges/${exchangeId}/sessions`) {
      if (state.failed) return send({ error: { code: "unavailable" } }, 503);
      return send({
        exchange: {
          id: exchangeId,
          status: "ACTIVE",
          format: "ONLINE",
          totalSessions: 4,
          requester: person,
          recipient: partner,
          partner,
          requesterTeaches: terms,
          recipientTeaches: terms,
          progress,
          nextSession: lesson.status === "SCHEDULED" ? lesson : null,
        },
        items: [lesson],
      });
    }
    if (path === `sessions/${sessionId}/cancel`) {
      expect(request.headers()["x-csrf-token"]).toBe("test-csrf");
      lesson.status = "CANCELLED";
      lesson.canCancel = false;
      lesson.cancellationReason = request.postDataJSON().reason;
      return send({ session: lesson });
    }
    if (path === `sessions/${sessionId}`) return send({ session: lesson, proposals: [] });
    if (path === `exchanges/${exchangeId}`) return send({ exchange: { status: "ACTIVE" } });
    if (path === "me/schedule")
      return send({
        items: lesson.status === "CANCELLED" ? [] : [lesson],
        total: 41,
        page: Number(url.searchParams.get("page")) || 1,
        pageSize: 20,
        timezone: "Europe/Kyiv",
        pendingProposalsCount: 0,
      });
    return send({ items: [], total: 0, unreadCount: 0 });
  });
  return state;
}

test("refresh updates the plan, progress and nearest meeting together, including retry", async ({
  page,
}) => {
  const state = await fixture(page);
  await page.goto(`/exchanges/${exchangeId}`);
  await expect(page.locator(".exchange-next")).toContainText("Перші акорди");
  state.lesson.status = "COMPLETED";
  state.progress.completedSessions = 1;
  state.progress.requesterCompletedSessions = 1;
  state.progress.percent = 25;
  await page.getByRole("button", { name: "Оновити", exact: true }).click();
  await expect(page.locator(".exchange-progress__percent")).toHaveText("25%");
  await expect(page.locator(".session-plan")).toContainText("Проведено");
  await expect(page.locator(".exchange-next")).toHaveCount(0);
  state.failed = true;
  await page.getByRole("button", { name: "Оновити", exact: true }).click();
  await expect(page.getByRole("button", { name: "Спробувати ще раз" })).toBeVisible();
  state.failed = false;
  await page.getByRole("button", { name: "Спробувати ще раз" }).click();
  await expect(page.locator(".exchange-progress__percent")).toHaveText("25%");
});

test("schedule context survives a lesson cancellation and return", async ({ page }) => {
  await fixture(page);
  const schedule = "/schedule?status=SCHEDULED&upcoming=false&page=2";
  await page.goto(schedule);
  await page.getByRole("link", { name: "Перші акорди", exact: true }).click();
  await expect(page).toHaveURL(new RegExp(`/sessions/${sessionId}\\?from=`));
  await expect(page.getByRole("link", { name: "← До розкладу" })).toHaveAttribute("href", schedule);
  await page.getByRole("button", { name: "Скасувати заняття", exact: true }).click();
  const dialog = page.getByRole("dialog");
  await dialog.getByLabel("Причина скасування", { exact: true }).fill("Змінилися плани");
  await dialog.getByRole("button", { name: "Скасувати заняття", exact: true }).click();
  await expect(
    page.getByText("Заняття скасовано. Історію збережено.", { exact: true }),
  ).toBeVisible();
  await page.getByRole("link", { name: "← До розкладу" }).click();
  await expect(page).toHaveURL(`http://127.0.0.1:3100${schedule}`);
  await expect(page.getByText("У розкладі поки немає занять.", { exact: true })).toBeVisible();
});

for (const action of ["Скасувати заняття", "Перенести заняття"]) {
  test(`expired session retains the return URL in ${action}`, async ({ page }) => {
    const state = await fixture(page);
    const from = "/schedule?status=ALL&upcoming=false&page=3";
    const path = `/sessions/${sessionId}?from=${encodeURIComponent(from)}`;
    await page.goto(path);
    await page.getByRole("button", { name: action, exact: true }).click();
    const dialog = page.getByRole("dialog");
    const cancelling = action === "Скасувати заняття";
    if (cancelling)
      await dialog.getByLabel("Причина скасування", { exact: true }).fill("Змінилися плани");
    state.expired = true;
    await dialog
      .getByRole("button", { name: cancelling ? action : "Надіслати пропозицію", exact: true })
      .click();
    const login = dialog.getByRole("link", { name: "Увійти", exact: true });
    await expect(login).toBeVisible();
    expect(new URL(await login.getAttribute("href"), page.url()).searchParams.get("next")).toBe(
      path,
    );
  });
}
