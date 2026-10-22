"use client";

import { useState } from "react";
import type { Review } from "./DiscoveryApi";
import type { Exchange } from "./ExchangeApi";
import { DiscoveryError } from "./DiscoveryError";
import { fullName } from "./DiscoveryFormat";
import { exchangeReviews, reviewErrorMessage } from "./ReviewApi";
import { ReviewCard } from "./ReviewCards";
import { ReviewForm } from "./ReviewForm";
import { useApiQuery } from "./useApiQuery";

export function ExchangeReviews({
  exchange,
  csrfToken,
  path,
  onRefresh,
}: {
  exchange: Exchange;
  csrfToken: string;
  path: string;
  onRefresh: () => void;
}) {
  const result = useApiQuery(`exchange-reviews/${exchange.id}`, (signal) =>
    exchangeReviews(exchange.id, signal),
  );
  const [open, setOpen] = useState(false);
  const [saved, setSaved] = useState<Review | null>(null);
  const [notice, setNotice] = useState("");
  const data = result.state.status === "ready" ? result.state.data : null;
  const mine = saved ?? data?.myReview;
  const canReview = Boolean(
    data?.canReview && exchange.canReview && !mine && exchange.status === "COMPLETED",
  );
  return (
    <section className="account-card received-reviews" aria-labelledby="exchange-reviews-title">
      <div className="dashboard-section-heading">
        <h2 id="exchange-reviews-title">Відгуки про обмін</h2>
        <button
          className="dashboard-button"
          disabled={result.state.status === "loading"}
          onClick={result.retry}
        >
          Оновити відгуки
        </button>
      </div>
      {notice && (
        <p className="profile-success" role="status">
          {notice}
        </p>
      )}
      {result.state.status === "loading" && (
        <p className="profile-hint" role="status">
          Завантажуємо відгуки…
        </p>
      )}
      {result.state.status === "error" && (
        <DiscoveryError error={result.state.error} path={path} onRetry={result.retry} />
      )}
      {mine && <ReviewCard review={mine} label="Твій відгук про партнера" />}
      {data && (
        <>
          {!mine && (
            <div className="review-invitation">
              <p>
                Поділися враженнями від навчання — твій відгук допоможе іншим студентам обрати
                партнера.
              </p>
              {canReview && (
                <button
                  className="dashboard-button dashboard-button--primary"
                  aria-haspopup="dialog"
                  onClick={() => setOpen(true)}
                >
                  Залишити відгук
                </button>
              )}
              {!canReview && (
                <p className="profile-hint">
                  Зараз залишити відгук неможливо. Онови дані обміну, щоб перевірити його стан.
                </p>
              )}
            </div>
          )}
          {data.partnerReview ? (
            <ReviewCard review={data.partnerReview} label="Відгук партнера про тебе" />
          ) : (
            <p className="profile-hint">Партнер ще не залишив відгук про тебе.</p>
          )}
        </>
      )}
      {open && canReview && (
        <ReviewForm
          exchangeId={exchange.id}
          partnerName={fullName(exchange.partner)}
          csrfToken={csrfToken}
          path={path}
          onClose={() => setOpen(false)}
          onSaved={(review) => {
            setSaved(review);
            setOpen(false);
            setNotice("Дякуємо! Твій відгук збережено та додано до рейтингу партнера.");
            result.retry();
          }}
          onConflict={(error) => {
            setOpen(false);
            if (error.code === "review_already_exists") {
              setNotice(reviewErrorMessage(error));
              result.retry();
            } else {
              onRefresh();
            }
          }}
        />
      )}
    </section>
  );
}
