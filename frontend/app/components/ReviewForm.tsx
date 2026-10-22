"use client";

import Link from "next/link";
import { useId, useRef, useState, type FormEvent } from "react";
import { AppDialog } from "./AppDialog";
import { AuthError } from "./AuthApi";
import { loginHref } from "./AppRoutes";
import { ReviewStar } from "./ReviewCards";
import type { Review } from "./DiscoveryApi";
import {
  createReview,
  MAX_REVIEW_LENGTH,
  ratingLabels,
  reviewErrorMessage,
  reviewFields,
  reviewServerFields,
  type ReviewFields,
} from "./ReviewApi";

export function ReviewForm({
  exchangeId,
  partnerName,
  csrfToken,
  path,
  onClose,
  onSaved,
  onConflict,
}: {
  exchangeId: string;
  partnerName: string;
  csrfToken: string;
  path: string;
  onClose: () => void;
  onSaved: (review: Review) => void;
  onConflict: (error: AuthError) => void;
}) {
  const id = useId();
  const formRef = useRef<HTMLFormElement>(null);
  const busy = useRef(false);
  const [rating, setRating] = useState(0);
  const [comment, setComment] = useState("");
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const [fields, setFields] = useState<ReviewFields>({});
  const length = Array.from(comment.trim()).length;

  async function submit(event: FormEvent) {
    event.preventDefault();
    if (busy.current) return;
    const problems = reviewFields({ rating, comment });
    setFields(problems);
    setError(null);
    if (Object.keys(problems).length) {
      formRef.current
        ?.querySelector<HTMLElement>(problems.rating ? "input[name=rating]" : "textarea")
        ?.focus();
      return;
    }
    busy.current = true;
    setPending(true);
    try {
      const data = await createReview(exchangeId, { rating, comment }, csrfToken);
      onSaved(data.review);
    } catch (problem) {
      if (problem instanceof AuthError && (problem.status === 409 || problem.status === 404)) {
        onConflict(problem);
        return;
      }
      setError(problem);
      setFields(reviewServerFields(problem));
    } finally {
      busy.current = false;
      setPending(false);
    }
  }

  return (
    <AppDialog
      title="Залишити відгук"
      badge="Обмін завершено"
      description={`Твій партнер — ${partnerName}. Поділися враженнями від навчання.`}
      canClose={!pending}
      onClose={onClose}
      className="review-dialog"
    >
      <form
        ref={formRef}
        className="review-form"
        noValidate
        onSubmit={(event) => void submit(event)}
      >
        <fieldset
          className="review-score"
          disabled={pending}
          role="radiogroup"
          aria-invalid={Boolean(fields.rating)}
          aria-describedby={`${id}-rating-help${fields.rating ? ` ${id}-rating-error` : ""}`}
        >
          <legend>
            Твоя оцінка <span className="profile-hint">(обов’язково)</span>
          </legend>
          <div className="review-score__options">
            {[1, 2, 3, 4, 5].map((value) => (
              <label key={value}>
                <input
                  type="radio"
                  name="rating"
                  value={value}
                  checked={rating === value}
                  required
                  aria-label={`${value} з 5 — ${ratingLabels[value]}`}
                  data-autofocus={value === 1 || undefined}
                  onChange={() => {
                    setRating(value);
                    setFields((previous) => ({ ...previous, rating: undefined }));
                  }}
                />
                <span data-filled={value <= rating} aria-hidden="true">
                  <ReviewStar />
                </span>
              </label>
            ))}
          </div>
          <p id={`${id}-rating-help`} className="profile-hint">
            {rating ? `${rating} з 5 — ${ratingLabels[rating]}` : "Обери від 1 до 5 зірок"}
          </p>
          {fields.rating && (
            <p id={`${id}-rating-error`} className="review-field-error" role="alert">
              {fields.rating}
            </p>
          )}
        </fieldset>
        <div className="review-form__comment">
          <label htmlFor={`${id}-comment`}>
            Текст відгуку <span className="profile-hint">(необов’язково)</span>
          </label>
          <textarea
            id={`${id}-comment`}
            rows={5}
            value={comment}
            disabled={pending}
            placeholder="Що сподобалося? Чим був корисний обмін?"
            aria-invalid={Boolean(fields.comment)}
            aria-describedby={`${id}-counter${fields.comment ? ` ${id}-comment-error` : ""}`}
            onChange={(event) => {
              setComment(event.target.value);
              setFields((previous) => ({ ...previous, comment: undefined }));
            }}
          />
          <p
            id={`${id}-counter`}
            className="review-counter"
            data-invalid={length > MAX_REVIEW_LENGTH}
          >
            {length} / {MAX_REVIEW_LENGTH} символів
          </p>
          {fields.comment && (
            <p id={`${id}-comment-error`} className="review-field-error" role="alert">
              {fields.comment}
            </p>
          )}
        </div>
        <p className="profile-hint">
          Відгук з’явиться у профілі партнера. Для цього обміну його можна надіслати лише один раз;
          змінити оцінку чи текст після надсилання не можна.
        </p>
        {error != null && (
          <p className="review-field-error" role="alert">
            {reviewErrorMessage(error)}{" "}
            {error instanceof AuthError && error.status === 401 && (
              <Link href={loginHref(path)}>Увійти</Link>
            )}
          </p>
        )}
        <div className="review-form__actions">
          <button type="button" className="dashboard-button" disabled={pending} onClick={onClose}>
            Пізніше
          </button>
          <button
            type="submit"
            className="dashboard-button dashboard-button--primary"
            disabled={pending}
          >
            {pending ? "Надсилаємо…" : "Надіслати відгук"}
          </button>
        </div>
      </form>
    </AppDialog>
  );
}
