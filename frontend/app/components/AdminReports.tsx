"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { adminHref, adminReports } from "./AdminApi";
import { Pagination } from "./DiscoveryCards";
import { pageCount } from "./DiscoveryApi";
import { fullName } from "./DiscoveryFormat";
import { formatDateTime } from "./ExchangeFormat";
import { reportStatuses } from "./SafetyApi";
import { SafetyFeedback } from "./SafetyFeedback";
import { useApiQuery } from "./useApiQuery";

export function AdminReports({ status, page }: { status: string; page: number }) {
  const router = useRouter();
  const path = adminHref("reports", { status, page });
  const query = useApiQuery(path, (signal) => adminReports(status, page, signal));
  const data = query.state.status === "ready" ? query.state.data : null;
  return (
    <section className="account-card safety-card">
      <div className="safety-row__heading">
        <h2>Скарги студентів{data ? ` · ${data.total}` : ""}</h2>
        <label className="safety-field">
          Статус
          <select
            value={status}
            onChange={(event) => router.push(adminHref("reports", { status: event.target.value }))}
          >
            <option value="">Усі статуси</option>
            {Object.entries(reportStatuses).map(([value, label]) => (
              <option value={value} key={value}>
                {label}
              </option>
            ))}
          </select>
        </label>
      </div>
      {query.state.status === "loading" && <p role="status">Завантажуємо скарги…</p>}
      {query.state.status === "error" && (
        <SafetyFeedback error={query.state.error} path={path} onRetry={query.retry} />
      )}
      {data && (
        <>
          {!data.items.length && (
            <div className="safety-empty">
              <h3>Скарг на цій сторінці немає</h3>
              <p>Спробуй інший статус або повернися пізніше.</p>
            </div>
          )}
          <ul className="safety-list">
            {data.items.map((report) => (
              <li className="safety-row safety-row--stack" key={report.id}>
                <div className="safety-row__heading">
                  <h3>{fullName(report.reportedUser)}</h3>
                  <span className={`safety-badge safety-badge--${report.status.toLowerCase()}`}>
                    {reportStatuses[report.status]}
                  </span>
                </div>
                <p className="profile-hint">
                  Від {fullName(report.reporter)} · {formatDateTime(report.createdAt)}
                </p>
                <p className="safety-text">{report.reason}</p>
                <Link
                  className="dashboard-button"
                  href={`/admin/reports/${report.id}?from=${encodeURIComponent(path)}`}
                >
                  Переглянути скаргу →
                </Link>
              </li>
            ))}
          </ul>
          <Pagination
            page={data.page}
            pages={Math.max(pageCount(data.total, data.pageSize), data.page)}
            hrefFor={(number) => adminHref("reports", { status, page: number })}
          />
        </>
      )}
    </section>
  );
}
