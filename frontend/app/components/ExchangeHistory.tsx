import { exchangeHistory } from "./CompletionApi";
import { DiscoveryError } from "./DiscoveryError";
import { fullName } from "./DiscoveryFormat";
import { exchangeHref, type Exchange } from "./ExchangeApi";
import { exchangeStatusLabels, formatDateTime } from "./ExchangeFormat";
import { useApiQuery } from "./useApiQuery";

export function ExchangeHistory({ exchange }: { exchange: Exchange }) {
  const result = useApiQuery(`${exchange.id}/${exchange.status}`, (signal) =>
    exchangeHistory(exchange.id, signal),
  );
  return (
    <section className="account-card" aria-labelledby="exchange-history-title">
      <h2 id="exchange-history-title">Історія обміну</h2>
      {result.state.status === "loading" && (
        <p className="profile-hint" role="status">
          Завантажуємо історію…
        </p>
      )}
      {result.state.status === "error" && (
        <DiscoveryError
          error={result.state.error}
          path={exchangeHref(exchange.id)}
          onRetry={result.retry}
        />
      )}
      {result.state.status === "ready" &&
        (result.state.data.history?.length ? (
          <ol className="session-history">
            {result.state.data.history.map((entry, index) => (
              <li key={`${entry.createdAt}-${index}`}>
                <strong>{exchangeStatusLabels[entry.status]}</strong>
                <p className="profile-hint">
                  <time dateTime={entry.createdAt}>{formatDateTime(entry.createdAt)}</time>
                  {entry.changedBy && ` · ${fullName(entry.changedBy)}`}
                </p>
                {entry.reason && <p className="profile-biography">{entry.reason}</p>}
              </li>
            ))}
          </ol>
        ) : (
          <p className="profile-hint">Змін статусу ще немає.</p>
        ))}
    </section>
  );
}
