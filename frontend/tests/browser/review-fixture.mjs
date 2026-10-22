import { expect } from "@playwright/test";
import { completionFixture, exchangeId, exchangePath } from "./completion-fixture.mjs";

export { exchangePath };
export async function reviewFixture(page) {
  const base = await completionFixture(page);
  const partner = base.exchange.partner;
  const me = base.exchange.requester;
  Object.assign(base.exchange, {
    status: "COMPLETED",
    canCancel: false,
    canReview: true,
    reviewedByMe: false,
    reviewedByPartner: false,
  });
  const state = {
    base,
    mine: null,
    partner: null,
    posts: [],
    failure: "",
    getFailure: false,
    total: 21,
    hold: null,
  };
  const review = (index, author = partner) => ({
    id: `review-${index}`,
    exchangeId,
    author,
    rating: 5,
    comment: `Відгук ${index}: дякую за навчання!`,
    createdAt: "2026-10-20T10:00:00Z",
  });
  state.example = review(1);
  state.studentPath = `/students/${partner.id}`;
  await page.route("**/api/v1/**", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const path = url.pathname.replace("/api/v1/", "");
    const send = (json, status = 200) => route.fulfill({ json, status });
    if (path === `exchanges/${exchangeId}/reviews`) {
      if (request.method() === "POST") {
        const draft = request.postDataJSON();
        state.posts.push(draft);
        expect(request.headers()["x-csrf-token"]).toBe("test-csrf");
        if (state.hold) await state.hold;
        if (state.failure === "network") return route.abort("failed");
        if (state.failure)
          return send(
            {
              error: {
                code: state.failure,
                fields:
                  state.failure === "validation_failed"
                    ? { comment: "English server validation" }
                    : {},
              },
            },
            state.failure === "unauthenticated"
              ? 401
              : state.failure === "validation_failed"
                ? 422
                : state.failure === "unavailable"
                  ? 503
                  : 409,
          );
        if (state.mine) return send({ error: { code: "review_already_exists" } }, 409);
        state.mine = { ...review(100, me), ...draft };
        base.exchange.canReview = false;
        base.exchange.reviewedByMe = true;
        return send({ review: state.mine, exchange: base.exchange }, 201);
      }
      if (state.getFailure) return send({ error: { code: "unavailable" } }, 503);
      return send({
        myReview: state.mine,
        partnerReview: state.partner,
        canReview: !state.mine && base.exchange.canReview,
      });
    }
    if (path === "me/reviews" || path === `students/${partner.id}/reviews`) {
      if (state.getFailure) return send({ error: { code: "unavailable" } }, 503);
      const current = Number(url.searchParams.get("page")) || 1;
      return send({
        items: Array.from(
          { length: Math.min(20, Math.max(0, state.total - (current - 1) * 20)) },
          (_, i) => review((current - 1) * 20 + i + 1),
        ),
        total: state.total,
        page: current,
        pageSize: 20,
        averageRating: state.total ? 4.5 : null,
        reviewCount: state.total,
      });
    }
    if (path === `students/${partner.id}`)
      return send({
        student: {
          ...partner,
          faculty: null,
          course: 3,
          bio: "Навчаємося разом",
          teachingSkills: [],
          learningSkills: [],
          formats: ["ONLINE"],
          mutual: null,
          averageRating: state.total ? 4.5 : null,
          reviewCount: state.total,
          reviews: [],
        },
      });
    if (path === "me/profile")
      return send({
        profile: {
          ...me,
          university: { id: exchangeId, name: "КПІ" },
          faculty: null,
          bio: "Вчуся й ділюся знаннями",
          course: 3,
          teachingSkills: [],
          learningSkills: [],
          formats: ["ONLINE"],
        },
      });
    if (path === "universities" || path.includes("/faculties") || path === "skills")
      return send({ items: [] });
    return route.fallback();
  });
  return state;
}
