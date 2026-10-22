import { DashboardAvatar } from "./DashboardCards";
import type { Review } from "./DiscoveryApi";
import {
  avatarColor,
  countLabel,
  formatDate,
  formatRating,
  fullName,
  initials,
  reviewForms,
} from "./DiscoveryFormat";
import type { RatingSummary } from "./ReviewApi";
import "../styles/reviews.css";

export function ReviewStar() {
  return <span className="review-star" aria-hidden="true" />;
}

export function ReviewStars({ rating }: { rating: number }) {
  return (
    <span className="review-stars" role="img" aria-label={`Оцінка ${rating} з 5`}>
      {[1, 2, 3, 4, 5].map((value) => (
        <span key={value} data-filled={value <= rating} aria-hidden="true">
          <ReviewStar />
        </span>
      ))}
    </span>
  );
}

export function ReviewRating({ averageRating, reviewCount }: RatingSummary) {
  if (averageRating === null || reviewCount === 0)
    return <p className="profile-hint">Ще немає відгуків</p>;
  return (
    <p className="review-rating">
      <span aria-hidden="true">
        <ReviewStar />
      </span>
      <strong>{formatRating(averageRating)}</strong>
      <span>з 5 · {countLabel(reviewCount, reviewForms)}</span>
    </p>
  );
}

export function ReviewCard({ review, label }: { review: Review; label?: string }) {
  return (
    <article className="review-card" aria-label={label ?? `Відгук: ${fullName(review.author)}`}>
      {label && <h3>{label}</h3>}
      <div className="review-card__head">
        <DashboardAvatar
          initials={initials(review.author)}
          color={avatarColor(review.author.id)}
          size={40}
        />
        <div className="review-card__author">
          <strong>{fullName(review.author)}</strong>
          <time dateTime={review.createdAt}>{formatDate(review.createdAt)}</time>
        </div>
        <ReviewStars rating={review.rating} />
      </div>
      {review.comment ? (
        <p className="profile-biography">{review.comment}</p>
      ) : (
        <p className="profile-hint">Оцінка без текстового відгуку.</p>
      )}
    </article>
  );
}
