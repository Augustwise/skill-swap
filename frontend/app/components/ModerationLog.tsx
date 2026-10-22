"use client";

import Link from "next/link";
import { actionLabels, adminHref, moderationLog, type ModerationAction } from "./AdminApi";
import { pageCount } from "./DiscoveryApi";
import { Pagination } from "./DiscoveryCards";
import { fullName } from "./DiscoveryFormat";
import { formatDateTime } from "./ExchangeFormat";
import { SafetyFeedback } from "./SafetyFeedback";
import { useApiQuery } from "./useApiQuery";

export function ActionHistory({ items }: { items: ModerationAction[] }) {
  return (
    <ol className="safety-list">
      {items.map((item) => (
        <li key={item.id} className="safety-row safety-row--stack">
          <h3>{actionLabels[item.action]}</h3>
          <p className="profile-hint">
            {fullName(item.moderator)} ·{" "}
            <time dateTime={item.createdAt}>{formatDateTime(item.createdAt)}</time>
          </p>
          <p>
            {item.subject ? fullName(item.subject) : (item.skill?.name ?? "Об’єкт недоступний")}
          </p>
          <p className="safety-text">{item.reason}</p>
          <div className="safety-actions">
            {item.reportId && (
              <Link className="profile-tip__link" href={`/admin/reports/${item.reportId}`}>
                Відкрити скаргу →
              </Link>
            )}
            {item.subject && (
              <Link
                className="profile-tip__link"
                href={adminHref("history", { userId: item.subject.id })}
              >
                Історія та керування акаунтом →
              </Link>
            )}
          </div>
        </li>
      ))}
    </ol>
  );
}

export function ModerationLog({ page, userId }: { page: number; userId: string }) {
  const query = useApiQuery(`log/${page}/${userId}`, (signal) =>
    moderationLog(page, userId, signal),
  );
  const data = query.state.status === "ready" ? query.state.data : null;
  return (
    <section className="account-card safety-card">
      <h2>Журнал дій{data ? ` · ${data.total}` : ""}</h2>
      <p className="profile-hint">Автор, дата та причина кожної зміни зберігаються на сервері.</p>
      {userId && (
        <Link className="profile-tip__link" href={adminHref("history")}>
          Показати всі акаунти й зміни каталогу →
        </Link>
      )}
      {query.state.status === "loading" && <p role="status">Завантажуємо журнал…</p>}
      {query.state.status === "error" && (
        <SafetyFeedback
          error={query.state.error}
          path={adminHref("history")}
          onRetry={query.retry}
        />
      )}
      {data && (
        <>
          <ActionHistory items={data.items} />
          {!data.items.length && <p className="safety-empty">Записів на цій сторінці немає.</p>}
          <Pagination
            page={data.page}
            pages={Math.max(pageCount(data.total, data.pageSize), data.page)}
            hrefFor={(number) => adminHref("history", { userId, page: number })}
          />
        </>
      )}
    </section>
  );
}
