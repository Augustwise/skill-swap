import Link from "next/link";
import { completionMessage, exchangesHref } from "./CompletionApi";
import { CompletionConfirmations } from "./CompletionFeedback";
import type { Exchange } from "./ExchangeApi";
import { formatDateTime } from "./ExchangeFormat";
import "../styles/completion.css";

export function ExchangeCompletion({
  exchange,
  pending,
  onConfirm,
}: {
  exchange: Exchange;
  pending: boolean;
  onConfirm: () => void;
}) {
  const terminal = exchange.status === "COMPLETED" || exchange.status === "CANCELLED";
  return (
    <section className="account-card" aria-labelledby="exchange-completion-title">
      <h2 id="exchange-completion-title">Завершення обміну</h2>
      <div className="completion-panel">
        <p>{completionMessage(exchange, "exchange")}</p>
        {exchange.status !== "CANCELLED" && <CompletionConfirmations item={exchange} />}
        {exchange.canConfirmCompletion && (
          <button
            className="dashboard-button dashboard-button--primary"
            disabled={pending}
            onClick={onConfirm}
          >
            {pending ? "Зберігаємо…" : "Підтвердити завершення"}
          </button>
        )}
        {exchange.status === "ACTIVE" && (
          <p className="profile-hint">
            Обмін залишатиметься активним, доки ви обоє не підтвердите його завершення.
          </p>
        )}
        {exchange.completedAt && (
          <p className="profile-hint">
            Завершено{" "}
            <time dateTime={exchange.completedAt}>{formatDateTime(exchange.completedAt)}</time>
          </p>
        )}
        {terminal && (
          <Link
            className="dashboard-button"
            href={exchangesHref(exchange.status === "COMPLETED" ? "COMPLETED" : "CANCELLED")}
          >
            До історії обмінів →
          </Link>
        )}
      </div>
    </section>
  );
}
