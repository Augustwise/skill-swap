import assert from "node:assert/strict";
import { test } from "node:test";
import { loadSource } from "./load-source.mjs";

const api = loadSource("app/components/ReviewApi.ts");
const { AuthError } = loadSource("app/components/AuthApi.ts");
const { notificationHref } = loadSource("app/components/NotificationApi.ts");
const id = "80000000-0000-0000-0000-000000000009";

test("reviews require a whole rating from 1 to 5 and allow no comment", () => {
  for (const rating of [0, 6, -1, 2.5, NaN, Infinity, "5"]) {
    assert.ok(api.reviewFields({ rating, comment: "" }).rating);
  }
  for (const rating of [1, 2, 3, 4, 5]) {
    assert.deepEqual(api.reviewFields({ rating, comment: " \n " }), {});
  }
});

test("review text mirrors the server Unicode limit without truncating a draft", () => {
  for (const comment of [
    "я".repeat(1000),
    "🙂".repeat(1000),
    "  Дякую\nза\tнавчання!  ",
    "<script>bad()</script>",
  ]) {
    assert.deepEqual(api.reviewFields({ rating: 5, comment }), {});
  }
  for (const comment of ["я".repeat(1001), "🙂".repeat(1001), "bad\0comment", "bad\u0085comment"]) {
    assert.ok(api.reviewFields({ rating: 5, comment }).comment);
  }
});

test("review pagination preserves profile tabs and safe return paths", () => {
  assert.equal(api.reviewsHref("/profile?tab=reviews", 2), "/profile?tab=reviews&reviewsPage=2");
  const path = `/students/${id}?from=${encodeURIComponent("/search?q=гітара&page=2")}`;
  const next = api.reviewsHref(path, 3);
  assert.equal(new URL(next, "https://local").searchParams.get("from"), "/search?q=гітара&page=2");
  assert.equal(api.reviewsHref(next), path);
  assert.equal(api.reviewsHref("https://evil.test", 2), "/profile?tab=reviews&reviewsPage=2");
  assert.equal(api.reviewsHref("/profile?tab=reviews", 501), "/profile?tab=reviews");
});

test("received and exchange reviews use private API reads and normalized pagination", async () => {
  const original = globalThis.fetch;
  const calls = [];
  const controller = new AbortController();
  globalThis.fetch = async (url, options) => {
    calls.push({ url, options });
    return Response.json({ items: [], averageRating: null, reviewCount: 0 });
  };
  try {
    await api.receivedReviews(null, 2, controller.signal);
    await api.receivedReviews(id, 500);
    await api.receivedReviews(null, -1);
    await api.exchangeReviews(id);
    assert.deepEqual(
      calls.map(({ url }) => url),
      [
        "/api/v1/me/reviews?page=2",
        `/api/v1/students/${id}/reviews?page=500`,
        "/api/v1/me/reviews?page=1",
        `/api/v1/exchanges/${id}/reviews`,
      ],
    );
    for (const { options } of calls) {
      assert.equal(options.credentials, "same-origin");
      assert.equal(options.cache, "no-store");
    }
    controller.abort();
    assert.equal(calls[0].options.signal.aborted, true);
    await assert.rejects(api.receivedReviews("invalid"), { code: "student_not_found" });
    await assert.rejects(api.exchangeReviews("invalid"), { code: "exchange_not_found" });
    assert.equal(calls.length, 4);
  } finally {
    globalThis.fetch = original;
  }
});

test("review creation sends only partner rating and text with CSRF and returns persisted state", async () => {
  const original = globalThis.fetch;
  const calls = [];
  globalThis.fetch = async (url, options) => {
    calls.push({ url, options });
    return Response.json({ review: { id: "saved", rating: 5 }, exchange: { canReview: false } });
  };
  try {
    const result = await api.createReview(id, { rating: 5, comment: "  Дякую!\n  " }, "csrf");
    assert.equal(result.review.id, "saved");
    assert.equal(result.exchange.canReview, false);
    assert.equal(calls[0].url, `/api/v1/exchanges/${id}/reviews`);
    assert.equal(calls[0].options.method, "POST");
    assert.equal(calls[0].options.headers.get("X-CSRF-Token"), "csrf");
    assert.deepEqual(JSON.parse(calls[0].options.body), { rating: 5, comment: "Дякую!" });
    await assert.rejects(api.createReview(id, { rating: 0, comment: "" }, "csrf"), {
      code: "validation_failed",
    });
    await assert.rejects(api.createReview("bad", { rating: 5, comment: "" }, "csrf"), {
      code: "exchange_not_found",
    });
    assert.equal(calls.length, 1);
  } finally {
    globalThis.fetch = original;
  }
});

test("stale CSRF is refreshed once while a duplicate review stays a conflict", async () => {
  const original = globalThis.fetch;
  const calls = [];
  globalThis.fetch = async (url, options) => {
    calls.push({ url, options });
    if (url.endsWith("auth/me")) return Response.json({ csrfToken: "fresh" });
    return calls.length === 1
      ? Response.json({ error: { code: "csrf_invalid" } }, { status: 403 })
      : Response.json({ error: { code: "review_already_exists" } }, { status: 409 });
  };
  try {
    await assert.rejects(api.createReview(id, { rating: 4, comment: "Чернетка" }, "stale"), {
      code: "review_already_exists",
    });
    assert.equal(calls.length, 3);
    assert.equal(calls[2].options.headers.get("X-CSRF-Token"), "fresh");
    assert.equal(calls[0].options.body, calls[2].options.body);
  } finally {
    globalThis.fetch = original;
  }
});

test("review failures and server field validation have Ukrainian messages", () => {
  for (const [status, code, expected] of [
    [409, "review_already_exists", /вже залишив/],
    [409, "exchange_not_completed", /завершення/],
    [404, "exchange_not_found", /не береш/],
    [404, "student_not_found", /недоступний/],
    [401, "unauthenticated", /Увійди/],
    [429, "rate_limited", /Забагато/],
  ]) {
    assert.match(api.reviewErrorMessage(new AuthError(status, code)), expected);
  }
  assert.match(api.reviewErrorMessage(new TypeError("offline")), /з’єднання/);
  const fields = api.reviewServerFields(
    new AuthError(422, "validation_failed", { rating: "English", comment: "English" }),
  );
  assert.match(fields.rating, /цілу/);
  assert.match(fields.comment, /1000/);
});

test("a review notification opens only a valid exchange", () => {
  assert.equal(notificationHref({ target: { kind: "EXCHANGE", id } }), `/exchanges/${id}`);
  assert.equal(notificationHref({ target: { kind: "EXCHANGE", id: "invalid" } }), "/notifications");
});
