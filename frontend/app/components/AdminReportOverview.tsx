"use client";

import Link from "next/link";
import { useState } from "react";
import { AppDialog } from "./AppDialog";
import { adminReport, decisions, resolveReport, type AdminReport, type Decision } from "./AdminApi";
import { safeAppPath } from "./AppRoutes";
import { fullName } from "./DiscoveryFormat";
import { formatDateTime } from "./ExchangeFormat";
import { ActionHistory } from "./ModerationLog";
import { ModerateAccount } from "./ModerateAccount";
import { reportStatuses, invalidFields, textError } from "./SafetyApi";
import { FieldError, SafetyFeedback } from "./SafetyFeedback";
import { useApiQuery } from "./useApiQuery";
import { useSafetyMutation } from "./useSafetyMutation";

export function AdminReportOverview({
  id,
  from,
  viewerId,
  csrfToken,
}: {
  id: string;
  from: string;
  viewerId: string;
  csrfToken: string;
}) {
  const path = `/admin/reports/${id}`;
  const query = useApiQuery(path, (signal) => adminReport(id, signal));
  const data = query.state.status === "ready" ? query.state.data : null;
  const [deciding, setDeciding] = useState(false);
  const [notice, setNotice] = useState("");
  return (
    <>
      <Link className="discovery-back" href={safeAppPath(from, "/admin?status=OPEN")}>
        ← До скарг
      </Link>
      {notice && (
        <p className="account-card profile-success" role="status">
          {notice}
        </p>
      )}
      {query.state.status === "loading" && (
        <p className="account-card" role="status">
          Завантажуємо скаргу…
        </p>
      )}
      {query.state.status === "error" && (
        <SafetyFeedback error={query.state.error} path={path} onRetry={query.retry} />
      )}
      {data && (
        <div className="profile-columns safety-columns">
          <div className="profile-sections">
            <section className="account-card safety-card">
              <div className="safety-row__heading">
                <h2>Скарга на {fullName(data.report.reportedUser)}</h2>
                <span className={`safety-badge safety-badge--${data.report.status.toLowerCase()}`}>
                  {reportStatuses[data.report.status]}
                </span>
              </div>
              <p className="profile-hint">
                Від {fullName(data.report.reporter)} · {formatDateTime(data.report.createdAt)}
              </p>
              <p className="safety-text">{data.report.reason}</p>
              {data.report.status === "OPEN" && (
                <div className="safety-actions">
                  <button
                    className="dashboard-button dashboard-button--primary"
                    type="button"
                    aria-haspopup="dialog"
                    onClick={() => setDeciding(true)}
                  >
                    Ухвалити рішення
                  </button>
                </div>
              )}
              {data.report.resolvedAt && (
                <p className="profile-hint">Розглянуто {formatDateTime(data.report.resolvedAt)}</p>
              )}
            </section>
            {data.report.decision && (
              <section className="account-card safety-card">
                <h2>Рішення за скаргою</h2>
                <ActionHistory items={[data.report.decision]} />
              </section>
            )}
            <section className="account-card safety-card">
              <h2>Історія модерації користувача</h2>
              <p className="profile-hint">
                До 20 останніх дій. Повний журнал доступний у розділі «Журнал дій».
              </p>
              {data.history.length ? (
                <ActionHistory items={data.history} />
              ) : (
                <p className="safety-empty">Попередніх дій немає.</p>
              )}
            </section>
          </div>
          <aside className="profile-rail">
            <ModerateAccount
              userId={data.report.reportedUser.id}
              viewerId={viewerId}
              name={fullName(data.report.reportedUser)}
              status={data.report.reportedUser.accountStatus}
              csrfToken={csrfToken}
              onChanged={(text) => {
                setNotice(text);
                query.retry();
              }}
            />
            <button type="button" className="dashboard-button" onClick={query.retry}>
              Оновити скаргу
            </button>
          </aside>
        </div>
      )}
      {deciding && data && (
        <ReportDecision
          report={data.report}
          viewerId={viewerId}
          csrfToken={csrfToken}
          onClose={() => setDeciding(false)}
          onDone={() => {
            setDeciding(false);
            setNotice("Рішення збережено. Автора, дату й причину записано в журнал.");
            query.retry();
          }}
          onRefresh={() => {
            setDeciding(false);
            query.retry();
          }}
        />
      )}
    </>
  );
}

function ReportDecision({
  report,
  viewerId,
  csrfToken,
  onClose,
  onDone,
  onRefresh,
}: {
  report: AdminReport;
  viewerId: string;
  csrfToken: string;
  onClose: () => void;
  onDone: () => void;
  onRefresh: () => void;
}) {
  const [decision, setDecision] = useState<Decision>("RESOLVE");
  const [reason, setReason] = useState("");
  const mutation = useSafetyMutation();
  return (
    <AppDialog
      title="Рішення за скаргою"
      badge={fullName(report.reportedUser)}
      canClose={!mutation.pending}
      onClose={onClose}
      description="Обери дію й поясни причину. Рішення та його автор залишаться в журналі модерації."
    >
      <form
        className="safety-form"
        noValidate
        onSubmit={async (event) => {
          event.preventDefault();
          const form = event.currentTarget;
          if (!mutation.validate(form, invalidFields({ reason: textError(reason, 1000) }))) return;
          const result = await mutation.run(
            () => resolveReport(report.id, decision, reason, csrfToken),
            form,
          );
          if (result.ok) onDone();
        }}
      >
        <label className="safety-field">
          Рішення
          <select
            name="decision"
            aria-label="Рішення"
            value={decision}
            disabled={mutation.pending}
            onChange={(event) => setDecision(event.target.value as Decision)}
          >
            {(Object.entries(decisions) as [Decision, string][]).map(([key, label]) => (
              <option
                key={key}
                value={key}
                disabled={key === "SUSPEND" && report.reportedUser.id === viewerId}
              >
                {label}
              </option>
            ))}
          </select>
        </label>
        {decision === "SUSPEND" && (
          <p className="safety-info">
            Вхід користувача буде заборонено, усі його сесії завершаться, активні обміни буде
            скасовано й очікувані запити закрито.
          </p>
        )}
        <label className="safety-field">
          Причина рішення
          <textarea
            name="reason"
            rows={4}
            value={reason}
            disabled={mutation.pending}
            aria-invalid={Boolean(mutation.fields.reason)}
            aria-describedby="decision-reason-error decision-count"
            onChange={(event) => setReason(event.target.value)}
          />
          <span id="decision-count" className="profile-hint">
            {Array.from(reason.trim()).length} / 1000
          </span>
          <FieldError id="decision-reason-error" message={mutation.fields.reason} />
        </label>
        <SafetyFeedback error={mutation.error} path={`/admin/reports/${report.id}`} />
        <div className="safety-actions safety-actions--end">
          <button
            type="button"
            className="dashboard-button"
            data-autofocus
            disabled={mutation.pending}
            onClick={onClose}
          >
            Скасувати
          </button>
          {mutation.error != null && (
            <button
              type="button"
              className="dashboard-button"
              disabled={mutation.pending}
              onClick={onRefresh}
            >
              Оновити дані
            </button>
          )}
          <button
            className={`dashboard-button ${decision === "SUSPEND" ? "dashboard-button--danger" : "dashboard-button--primary"}`}
            disabled={mutation.pending}
          >
            {mutation.pending ? "Зберігаємо…" : "Зберегти рішення"}
          </button>
        </div>
      </form>
    </AppDialog>
  );
}
