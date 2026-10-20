import assert from "node:assert/strict";
import { test } from "node:test";
import { loadSource } from "./load-source.mjs";

const api = loadSource("app/components/CompletionApi.ts");
const { AuthError } = loadSource("app/components/AuthApi.ts");
const base = {
  status: "SCHEDULED",
  completionConfirmedByMe: false,
  completionConfirmedByPartner: false,
  canConfirmCompletion: false,
};

test("completion states distinguish first and second confirmation from actual completion", () => {
  assert.match(api.completionMessage(base, "lesson"), /після завершення/);
  assert.match(
    api.completionMessage({ ...base, canConfirmCompletion: true }, "lesson"),
    /відбулося/,
  );
  assert.match(
    api.completionMessage({ ...base, completionConfirmedByMe: true }, "lesson"),
    /Очікуємо/,
  );
  assert.match(
    api.completionMessage({ ...base, completionConfirmedByPartner: true }, "lesson"),
    /твоя черга/,
  );
  assert.match(api.completionMessage({ ...base, status: "COMPLETED" }, "lesson"), /зараховано/);
  assert.match(api.completionMessage({ ...base, status: "CANCELLED" }, "lesson"), /Історію/);
});

test("exchange completion explains the two-direction plan and mutual decision", () => {
  assert.match(api.completionMessage({ ...base, status: "ACTIVE" }, "exchange"), /обох напрямках/);
  assert.match(
    api.completionMessage({ ...base, canConfirmCompletion: true }, "exchange"),
    /План виконано/,
  );
  assert.match(api.completionMessage({ ...base, status: "COMPLETED" }, "exchange"), /успішно/);
});

test("cancellation validates Unicode length, whitespace and control characters", () => {
  for (const invalid of [
    "",
    "  \n ",
    "я".repeat(501),
    "🙂".repeat(501),
    "bad\u0000reason",
    "bad\u0085reason",
  ]) {
    assert.ok(api.cancellationError(invalid));
  }
  for (const valid of [" Змінився розклад ", "🙂".repeat(500), "Причина\nДругий рядок"]) {
    assert.equal(api.cancellationError(valid), "");
  }
});

test("history filters reject unknown keys and preserve pagination", () => {
  for (const status of ["ACTIVE", "COMPLETED", "CANCELLED"]) {
    const filters = api.readExchangeFilters(new URLSearchParams({ status, page: "2" }));
    assert.deepEqual(filters, { status, page: 2 });
    assert.deepEqual(
      api.readExchangeFilters(new URL(api.exchangesHref(status, 2), "https://local").searchParams),
      filters,
    );
  }
  for (const status of ["__proto__", "toString", "unknown"]) {
    assert.equal(api.readExchangeFilters(new URLSearchParams({ status })).status, "ACTIVE");
  }
  assert.equal(api.exchangesHref(), "/exchanges");
});

test("completion endpoints use CSRF, return server data and trim cancellation reasons", async () => {
  const original = globalThis.fetch;
  const calls = [];
  globalThis.fetch = async (url, options) => {
    calls.push({ url, options });
    return Response.json({ session: { status: "SCHEDULED" }, exchange: { status: "ACTIVE" } });
  };
  try {
    const result = await api.confirmLesson("lesson-id", "csrf");
    assert.equal(result.session.status, "SCHEDULED");
    await api.completeExchange("exchange-id", "csrf");
    await api.cancelExchange("exchange-id", "  Причина  ", "csrf");
    assert.deepEqual(
      calls.map((call) => call.url),
      [
        "/api/v1/sessions/lesson-id/confirm",
        "/api/v1/exchanges/exchange-id/complete",
        "/api/v1/exchanges/exchange-id/cancel",
      ],
    );
    for (const { options } of calls) {
      assert.equal(options.method, "POST");
      assert.equal(options.headers.get("X-CSRF-Token"), "csrf");
      assert.equal(options.credentials, "same-origin");
    }
    assert.deepEqual(JSON.parse(calls[2].options.body), { reason: "Причина" });
    await assert.rejects(api.exchangeHistory("invalid"), { code: "exchange_not_found" });
  } finally {
    globalThis.fetch = original;
  }
});

test("completion errors explain stale state, incomplete plan and expired sessions", () => {
  assert.match(api.completionErrorMessage(new AuthError(409, "plan_not_completed")), /всі заняття/);
  assert.match(api.completionErrorMessage(new AuthError(409, "exchange_not_active")), /оновили/);
  assert.match(api.completionErrorMessage(new AuthError(401, "unauthenticated")), /Увійди/);
  assert.match(api.completionErrorMessage(new TypeError("offline")), /з’єднання/);
});
