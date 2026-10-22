"use client";

import Link from "next/link";
import { useId } from "react";
import { Pagination } from "./DiscoveryCards";
import { DiscoveryError } from "./DiscoveryError";
import { pageCount } from "./DiscoveryApi";
import { receivedReviews, reviewsHref } from "./ReviewApi";
import { ReviewCard, ReviewRating } from "./ReviewCards";
import { Skeleton } from "./Skeleton";
import { useApiQuery } from "./useApiQuery";

export function ReceivedReviews({
  studentId,
  page,
  path,
}: {
  studentId: string | null;
  page: number;
  path: string;
}) {
  const titleId = useId();
  const result = useApiQuery(`reviews/${studentId ?? "me"}/${page}`, (signal) =>
    receivedReviews(studentId, page, signal),
  );
  const data = result.state.status === "ready" ? result.state.data : null;
  return (
    <section className="account-card received-reviews" aria-labelledby={titleId}>
      <div className="dashboard-section-heading">
        <h2 id={titleId}>{studentId === null ? "Відгуки про мене" : "Відгуки"}</h2>
        <button
          className="dashboard-button"
          type="button"
          onClick={result.retry}
          disabled={result.state.status === "loading"}
        >
          Оновити відгуки
        </button>
      </div>
      {result.state.status === "loading" && (
        <>
          <p className="profile-hint" role="status">
            Завантажуємо відгуки…
          </p>
          <Skeleton height={80} />
        </>
      )}
      {result.state.status === "error" && (
        <DiscoveryError
          error={result.state.error}
          path={reviewsHref(path, page)}
          onRetry={result.retry}
          notFound={<p role="alert">Профіль студента недоступний.</p>}
        />
      )}
      {data && (
        <>
          <ReviewRating averageRating={data.averageRating} reviewCount={data.reviewCount} />
          {data.items.length > 0 ? (
            <ul className="review-list">
              {data.items.map((review) => (
                <li key={review.id}>
                  <ReviewCard review={review} />
                </li>
              ))}
            </ul>
          ) : data.total > 0 || page > 1 ? (
            <p className="profile-hint">
              На цій сторінці немає відгуків.{" "}
              <Link href={reviewsHref(path)}>До першої сторінки</Link>
            </p>
          ) : (
            <p className="profile-hint">Відгуки з’являться після завершених обмінів навичками.</p>
          )}
          <Pagination
            page={page}
            pages={pageCount(data.total, data.pageSize)}
            hrefFor={(next) => reviewsHref(path, next)}
          />
        </>
      )}
    </section>
  );
}

export function ProfileReputation() {
  const result = useApiQuery("reviews/me/summary", (signal) => receivedReviews(null, 1, signal));
  return (
    <section className="account-card received-reviews" aria-label="Мій рейтинг">
      <h2>Мій рейтинг</h2>
      {result.state.status === "ready" ? (
        <ReviewRating {...result.state.data} />
      ) : result.state.status === "loading" ? (
        <p className="profile-hint" role="status">
          Завантажуємо рейтинг…
        </p>
      ) : (
        <p className="profile-hint">
          Не вдалося завантажити рейтинг.{" "}
          <button className="review-text-button" onClick={result.retry}>
            Спробувати ще раз
          </button>
        </p>
      )}
      <Link className="profile-tip__link" href="/profile?tab=reviews">
        Усі відгуки про мене →
      </Link>
    </section>
  );
}
