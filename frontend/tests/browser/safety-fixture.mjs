import { expect } from "@playwright/test";
import { completionFixture, exchangeId } from "./completion-fixture.mjs";

export const reportId = "a0000000-0000-0000-0000-000000000010";
export const categoryId = "b0000000-0000-0000-0000-000000000010";
export const skillId = "c0000000-0000-0000-0000-000000000010";
export const conversationId = "d0000000-0000-0000-0000-000000000010";
export const reportPath = `/admin/reports/${reportId}`;

export async function safetyFixture(page, role = "STUDENT") {
  const base = await completionFixture(page);
  const me = base.exchange.requester;
  const partner = base.exchange.partner;
  const createdAt = "2026-10-20T10:00:00Z";
  const state = {
    base,
    role,
    me,
    partner,
    privacy: { isDiscoverable: true, acceptsRequests: true, requestScope: "ANYONE" },
    blocks: [],
    reports: [],
    mutations: [],
    reads: [],
    actions: [],
    failure: "",
    failFields: {},
    readFailure: "",
    hold: null,
    blocked: false,
    report: {
      id: reportId,
      reporter: me,
      reportedUser: { ...partner, accountStatus: "ACTIVE" },
      reason: "Неодноразові образи в чаті",
      status: "OPEN",
      createdAt,
      resolvedAt: null,
      decision: null,
    },
    skills: [
      {
        id: skillId,
        categoryId,
        categoryName: "Технології",
        name: "Вебдизайн",
        slug: "web-design",
        description: "Робота з інтерфейсами",
        isActive: true,
      },
    ],
    reportTotal: null,
  };
  const conversation = () => ({
    ...base.exchange,
    id: conversationId,
    type: "EXCHANGE",
    requestId: null,
    exchangeId,
    exchangeStatus: base.exchange.status,
    requestStatus: null,
    lastMessage: null,
    lastMessageAt: null,
    unreadCount: 0,
    canSend: !state.blocked,
    blockedByMe: state.blocked,
    createdAt,
  });
  const action = (type, reason, report = null, skill = null) => {
    const result = {
      id: `action-${state.actions.length}`,
      reportId: report,
      moderator: me,
      subject: skill ? null : partner,
      skill,
      action: type,
      reason,
      createdAt,
    };
    state.actions.unshift(result);
    return result;
  };
  await page.route("**/api/v1/**", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const path = url.pathname.replace("/api/v1/", "");
    const method = request.method();
    const send = (json, status = 200) => route.fulfill({ json, status });
    const paginated = (items, pageSize = 20, total = items.length) => {
      const current = Number(url.searchParams.get("page")) || 1;
      return {
        items: items.slice((current - 1) * pageSize, current * pageSize),
        page: current,
        pageSize,
        total,
      };
    };
    const handled =
      path === "me/privacy" ||
      path === "me/blocks" ||
      path === "me/reports" ||
      path === "me/account" ||
      path === "auth/change-password" ||
      path === "reports" ||
      path.endsWith("/block") ||
      path.startsWith("admin/");
    if (handled && method !== "GET") {
      expect(request.headers()["x-csrf-token"]).toBe("test-csrf");
      state.mutations.push({
        path,
        method,
        body: request.postData() ? request.postDataJSON() : null,
      });
      if (state.hold) await state.hold;
      if (state.failure === "network") return route.abort("failed");
      if (state.failure)
        return send(
          { error: { code: state.failure, fields: state.failFields } },
          state.failure === "unauthenticated"
            ? 401
            : state.failure === "validation_failed"
              ? 422
              : state.failure === "unavailable"
                ? 503
                : state.failure === "admin_required"
                  ? 403
                  : 409,
        );
    } else if (method === "GET") {
      state.reads.push(path);
      if (state.readFailure === path) return send({ error: { code: "unavailable" } }, 503);
    }
    if (path === "auth/me")
      return send({
        user: {
          id: me.id,
          email: "demo.olha@students.example.test",
          emailVerified: true,
          role: state.role,
        },
        csrfToken: "test-csrf",
      });
    if (path === "me/privacy") {
      if (method === "PATCH") state.privacy = request.postDataJSON();
      return send({ privacy: state.privacy });
    }
    if (path === "auth/change-password") return send({ status: "password_changed" });
    if (path === "me/account") return route.fulfill({ status: 204 });
    if (path === `students/${partner.id}/block`) {
      state.blocked = method === "POST";
      state.blocks = state.blocked ? [{ user: partner, createdAt }] : [];
      if (method === "DELETE") return route.fulfill({ status: 204 });
      base.exchange.status = "CANCELLED";
      base.exchange.canCancel = false;
      base.exchange.cancellationReason = "Один з учасників заблокував іншого";
      base.lesson.status = "CANCELLED";
      base.other.status = "CANCELLED";
      return send({ block: state.blocks[0], cancelledExchanges: 1, closedRequests: 2 });
    }
    if (path === "me/blocks") return send(paginated(state.blocks));
    if (path === "reports") {
      const body = request.postDataJSON();
      const report = { ...state.report, reason: body.reason };
      state.reports.unshift(report);
      return send({ report }, 201);
    }
    if (path === "me/reports") return send(paginated(state.reports));
    if (path === "admin/reports") {
      const status = url.searchParams.get("status");
      const items = !status || state.report.status === status ? [state.report] : [];
      return send(paginated(items, 20, state.reportTotal ?? items.length));
    }
    if (path === `admin/reports/${reportId}`)
      return send({ report: state.report, history: state.actions });
    if (path === `admin/reports/${reportId}/resolve`) {
      const { decision, reason } = request.postDataJSON();
      state.report.status = decision === "DISMISS" ? "DISMISSED" : "RESOLVED";
      state.report.resolvedAt = createdAt;
      if (decision === "SUSPEND") state.report.reportedUser.accountStatus = "SUSPENDED";
      state.report.decision = action(
        decision === "DISMISS"
          ? "DISMISS_REPORT"
          : decision === "RESOLVE"
            ? "RESOLVE_REPORT"
            : "SUSPEND_ACCOUNT",
        reason,
        reportId,
      );
      return send({ report: state.report });
    }
    if (path.startsWith("admin/users/")) {
      const suspend = path.endsWith("suspend");
      state.report.reportedUser.accountStatus = suspend ? "SUSPENDED" : "ACTIVE";
      const result = action(
        suspend ? "SUSPEND_ACCOUNT" : "RESTORE_ACCOUNT",
        request.postDataJSON().reason,
      );
      return send({ user: state.report.reportedUser, action: result });
    }
    if (path === "admin/moderation-actions") return send(paginated(state.actions));
    if (path === "admin/skills" && method === "GET") {
      const q = url.searchParams.get("q")?.toLocaleLowerCase("uk") ?? "";
      return send(
        paginated(
          state.skills.filter((skill) => skill.name.toLocaleLowerCase("uk").includes(q)),
          50,
        ),
      );
    }
    if (path === "admin/skills" || path.startsWith("admin/skills/")) {
      const body = request.postDataJSON();
      const skill = {
        ...body,
        id: method === "POST" ? "c0000000-0000-0000-0000-000000000011" : skillId,
        categoryName: "Технології",
      };
      if (method === "POST") state.skills.push(skill);
      else state.skills[0] = skill;
      action(
        method === "POST" ? "CREATE_SKILL" : "UPDATE_SKILL",
        `Змінено навичку «${skill.name}»`,
        null,
        { id: skill.id, name: skill.name },
      );
      return send({ skill }, method === "POST" ? 201 : 200);
    }
    if (path === "skill-categories")
      return send({ items: [{ id: categoryId, name: "Технології", slug: "technology" }] });
    if (path === `students/${partner.id}`)
      return state.blocked
        ? send({ error: { code: "student_not_found" } }, 404)
        : send({
            student: {
              ...partner,
              course: 3,
              bio: "Навчаємося разом",
              formats: ["ONLINE"],
              teachingSkills: [],
              learningSkills: [],
              mutual: null,
              averageRating: null,
              reviewCount: 0,
            },
          });
    if (path === `students/${partner.id}/reviews`)
      return send({ ...paginated([]), averageRating: null, reviewCount: 0 });
    if (path === `conversations/${conversationId}`) return send({ conversation: conversation() });
    if (path === "me/conversations") return send(paginated([conversation()]));
    if (path === `conversations/${conversationId}/messages`)
      return send({ ...paginated([]), pollIntervalSeconds: 5 });
    if (path === `exchanges/${exchangeId}/conversation`)
      return send({ conversation: conversation() });
    if (path === `exchange-requests/${exchangeId}`)
      return send({
        request: {
          ...base.exchange,
          id: exchangeId,
          status: state.blocked ? "WITHDRAWN" : "PENDING",
          message: "Навчаймося разом",
          createdAt,
        },
        history: [],
      });
    if (path === `exchange-requests/${exchangeId}/conversation`)
      return send({ conversation: conversation() });
    return route.fallback();
  });
  state.partnerPath = `/students/${partner.id}`;
  state.chatPath = `/messages/${conversationId}`;
  state.exchangePath = `/exchanges/${exchangeId}`;
  state.requestPath = `/requests/${exchangeId}`;
  return state;
}
