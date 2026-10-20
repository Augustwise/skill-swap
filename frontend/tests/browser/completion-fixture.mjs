import { expect } from "@playwright/test";

export const exchangeId = "80000000-0000-0000-0000-000000000008";
export const lessonId = "90000000-0000-0000-0000-000000000008";
export const exchangePath = `/exchanges/${exchangeId}`;
export const lessonPath = `/sessions/${lessonId}`;

export async function completionFixture(page) {
  const person = {
    id: exchangeId,
    firstName: "Ольга",
    lastName: "Тест",
    university: { name: "КПІ" },
    city: "Київ",
  };
  const partner = { ...person, id: lessonId, firstName: "Андрій" };
  const terms = {
    skill: { name: "Гітара" },
    teacherLevel: "ADVANCED",
    learnerLevel: "BEGINNER",
    sessions: 1,
    durationMinutes: 60,
  };
  const lesson = {
    id: lessonId,
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
    startsAt: "2020-01-01T09:00:00Z",
    endsAt: "2020-01-01T10:00:00Z",
    plannedDurationMinutes: 60,
    canCancel: true,
    canConfirmCompletion: true,
    completionConfirmedByMe: false,
    completionConfirmedByPartner: false,
  };
  const other = {
    ...lesson,
    id: "90000000-0000-0000-0000-000000000009",
    title: "Наступне заняття",
    planOrder: 2,
    status: "DRAFT",
    canConfirmCompletion: false,
  };
  const exchange = {
    id: exchangeId,
    status: "ACTIVE",
    format: "ONLINE",
    totalSessions: 2,
    requester: person,
    recipient: partner,
    partner,
    requesterTeaches: terms,
    recipientTeaches: terms,
    canCancel: true,
    canConfirmCompletion: false,
    completionConfirmedByMe: false,
    completionConfirmedByPartner: false,
    nextSession: null,
    progress: {
      completedSessions: 0,
      totalSessions: 2,
      percent: 0,
      requesterCompletedSessions: 0,
      requesterPlannedSessions: 1,
      recipientCompletedSessions: 0,
      recipientPlannedSessions: 1,
      awaitingConfirmationSessions: 1,
      planCompleted: false,
    },
  };
  const state = {
    lesson,
    other,
    exchange,
    posts: [],
    failure: "",
    total: 1,
    history: [{ status: "ACTIVE", changedBy: person, createdAt: "2020-01-01T08:00:00Z" }],
  };
  await page.route("**/api/v1/**", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const path = url.pathname.replace("/api/v1/", "");
    const send = (json, status = 200) => route.fulfill({ status, json });
    if (request.method() === "POST") {
      state.posts.push(path);
      if (state.hold) await state.hold;
      expect(request.headers()["x-csrf-token"]).toBe("test-csrf");
      if (state.failure)
        return send(
          { error: { code: state.failure } },
          state.failure === "unauthenticated" ? 401 : state.failure === "unavailable" ? 503 : 409,
        );
      if (path === `sessions/${lessonId}/confirm`) {
        lesson.completionConfirmedByMe = true;
        lesson.canConfirmCompletion = false;
        if (lesson.completionConfirmedByPartner) {
          lesson.status = "COMPLETED";
          lesson.completedAt = "2020-01-01T10:01:00Z";
          exchange.progress.completedSessions = 1;
          exchange.progress.requesterCompletedSessions = 1;
          exchange.progress.percent = 50;
        }
        return send({ session: lesson, exchange });
      }
      if (path === `exchanges/${exchangeId}/complete`) {
        exchange.completionConfirmedByMe = true;
        exchange.canConfirmCompletion = false;
        if (exchange.completionConfirmedByPartner) {
          exchange.status = "COMPLETED";
          exchange.completedAt = "2020-01-01T10:02:00Z";
          exchange.canCancel = false;
          state.history.push({
            status: "COMPLETED",
            changedBy: person,
            createdAt: exchange.completedAt,
          });
        }
        return send({ exchange });
      }
      if (path === `exchanges/${exchangeId}/cancel`) {
        exchange.status = "CANCELLED";
        exchange.canCancel = false;
        exchange.cancellationReason = request.postDataJSON().reason;
        exchange.cancelledAt = "2020-01-01T10:03:00Z";
        for (const item of [lesson, other])
          if (item.status !== "COMPLETED") item.status = "CANCELLED";
        state.history.push({
          status: "CANCELLED",
          changedBy: person,
          reason: exchange.cancellationReason,
          createdAt: exchange.cancelledAt,
        });
        return send({ exchange });
      }
    }
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
    if (path === `sessions/${lessonId}`) return send({ session: lesson, proposals: [] });
    if (path === `exchanges/${exchangeId}/sessions`)
      return send({ exchange, items: [lesson, other] });
    if (path === `exchanges/${exchangeId}`) return send({ exchange, history: state.history });
    if (path === "me/exchanges")
      return send({
        items: url.searchParams.get("status") === exchange.status ? [exchange] : [],
        total: state.total,
        page: Number(url.searchParams.get("page")) || 1,
        pageSize: 20,
      });
    return send({ items: [], total: 0, unreadCount: 0 });
  });
  return state;
}
