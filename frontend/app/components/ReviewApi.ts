import { AuthError, authRequest, requestError, writeRequest } from "./AuthApi";
import { isUuid, safeAppPath } from "./AppRoutes";
import { readPage, type ResultPage, type Review } from "./DiscoveryApi";
import type { Exchange } from "./ExchangeApi";

export type ReviewDraft = { rating: number; comment: string };
export type ReviewFields = Partial<Record<keyof ReviewDraft, string>>;
export type RatingSummary = { averageRating: number | null; reviewCount: number };
export type ReviewPage = ResultPage<Review> & RatingSummary;
export type ExchangeReviews = {
  myReview: Review | null;
  partnerReview: Review | null;
  canReview: boolean;
};
export type CreatedReview = { review: Review; exchange: Exchange };

export const MAX_REVIEW_LENGTH = 1000;
export const ratingLabels = [
  "",
  "Незадовільно",
  "Могло бути краще",
  "Добре",
  "Дуже добре",
  "Чудово",
];

export function reviewFields(draft: ReviewDraft): ReviewFields {
  const fields: ReviewFields = {};
  if (!Number.isInteger(draft.rating) || draft.rating < 1 || draft.rating > 5) {
    fields.rating = "Обери оцінку від 1 до 5.";
  }
  const comment = draft.comment.trim();
  if (Array.from(comment).length > MAX_REVIEW_LENGTH) {
    fields.comment = "Текст відгуку має містити не більше 1000 символів.";
  } else if (/[\u0000-\u0008\u000b\u000c\u000e-\u001f\u007f-\u009f]/u.test(comment)) {
    fields.comment = "Прибери недопустимі керівні символи з тексту.";
  }
  return fields;
}

export function reviewsHref(path: string, page = 1) {
  const url = new URL(safeAppPath(path, "/profile?tab=reviews"), "http://skillswap.local");
  url.searchParams.delete("reviewsPage");
  const normalized = readPage(String(page));
  if (normalized > 1) url.searchParams.set("reviewsPage", String(normalized));
  return `${url.pathname}${url.search}`;
}

export function receivedReviews(studentId: string | null, page = 1, signal?: AbortSignal) {
  if (studentId !== null && !isUuid(studentId)) {
    return Promise.reject(new AuthError(404, "student_not_found"));
  }
  const path = studentId === null ? "me/reviews" : `students/${studentId}/reviews`;
  return authRequest<ReviewPage>(`${path}?page=${readPage(String(page))}`, { signal });
}

export function exchangeReviews(exchangeId: string, signal?: AbortSignal) {
  if (!isUuid(exchangeId)) return Promise.reject(new AuthError(404, "exchange_not_found"));
  return authRequest<ExchangeReviews>(`exchanges/${exchangeId}/reviews`, { signal });
}

export function createReview(exchangeId: string, draft: ReviewDraft, csrfToken: string) {
  if (!isUuid(exchangeId)) return Promise.reject(new AuthError(404, "exchange_not_found"));
  const fields = reviewFields(draft);
  if (Object.keys(fields).length) {
    return Promise.reject(
      new AuthError(422, "validation_failed", fields as Record<string, string>),
    );
  }
  return writeRequest<CreatedReview>(
    `exchanges/${exchangeId}/reviews`,
    { body: JSON.stringify({ rating: draft.rating, comment: draft.comment.trim() }) },
    csrfToken,
  );
}

export function reviewErrorMessage(error: unknown) {
  if (error instanceof AuthError) {
    switch (error.code) {
      case "review_already_exists":
        return "Ти вже залишив відгук про цей обмін. Оновлюємо збережені відгуки.";
      case "exchange_not_completed":
        return "Відгук можна залишити лише після завершення обміну обома учасниками.";
      case "exchange_not_found":
        return "Обмін недоступний або ти не береш у ньому участі.";
      case "student_not_found":
        return "Профіль студента недоступний.";
      case "validation_failed":
        return "Перевір оцінку й текст відгуку.";
    }
  }
  return requestError(error);
}

export function reviewServerFields(error: unknown): ReviewFields {
  if (!(error instanceof AuthError) || error.code !== "validation_failed") return {};
  return {
    ...(error.fields.rating ? { rating: "Обери цілу оцінку від 1 до 5." } : {}),
    ...(error.fields.comment
      ? { comment: "Введи текст до 1000 символів без керівних символів." }
      : {}),
  };
}
