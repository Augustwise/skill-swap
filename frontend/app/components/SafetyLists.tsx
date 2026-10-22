"use client";

import { useState } from "react";
import { AppDialog } from "./AppDialog";
import { Pagination } from "./DiscoveryCards";
import { pageCount, type ResultPage } from "./DiscoveryApi";
import { fullName } from "./DiscoveryFormat";
import { formatDateTime } from "./ExchangeFormat";
import {
  blockedStudents,
  myReports,
  reportStatuses,
  settingsHref,
  unblockStudent,
  type Block,
  type Report,
} from "./SafetyApi";
import { SafetyFeedback } from "./SafetyFeedback";
import { useApiQuery } from "./useApiQuery";
import { useSafetyMutation } from "./useSafetyMutation";

export function SafetyLists({
  tab,
  page,
  csrfToken,
}: {
  tab: "blocks" | "reports";
  page: number;
  csrfToken: string;
}) {
  const query = useApiQuery<ResultPage<Block | Report>>(`${tab}/${page}`, (signal) =>
    tab === "blocks" ? blockedStudents(page, signal) : myReports(page, signal),
  );
  const [unblocking, setUnblocking] = useState<Block | null>(null);
  const [notice, setNotice] = useState("");
  const data = query.state.status === "ready" ? query.state.data : null;
  return (
    <section className="account-card safety-card">
      <h2>
        {tab === "blocks" ? "Заблоковані студенти" : "Мої скарги"}
        {data ? ` · ${data.total}` : ""}
      </h2>
      <p className="profile-hint">
        {tab === "blocks"
          ? "Особисті блокування. Розблокування дозволяє почати нову взаємодію, але не відновлює скасовані обміни."
          : "Тут можна перевірити статус надісланих скарг. Рішення й журнал модерації доступні адміністраторам."}
      </p>
      {notice && (
        <p className="profile-success" role="status">
          {notice}
        </p>
      )}
      {query.state.status === "loading" && <p role="status">Завантажуємо список…</p>}
      {query.state.status === "error" && (
        <SafetyFeedback
          error={query.state.error}
          path={settingsHref(tab, page)}
          onRetry={query.retry}
        />
      )}
      {data && (
        <>
          {!data.items.length && (
            <div className="safety-empty">
              <h3>
                {data.total
                  ? "На цій сторінці немає записів"
                  : tab === "blocks"
                    ? "Немає заблокованих студентів"
                    : "Ти ще не надсилав скарг"}
              </h3>
              <p>
                {tab === "blocks"
                  ? "Заблокувати студента можна в його профілі, запиті, обміні або чаті."
                  : "Надіслати скаргу можна в профілі студента, запиті, обміні або чаті."}
              </p>
            </div>
          )}
          <ul className="safety-list">
            {data.items.map((item) =>
              "user" in item ? (
                <li key={item.user.id} className="safety-row">
                  <div>
                    <h3>{fullName(item.user)}</h3>
                    <p className="profile-hint">Заблоковано {formatDateTime(item.createdAt)}</p>
                  </div>
                  <button
                    className="dashboard-button"
                    type="button"
                    aria-haspopup="dialog"
                    onClick={() => setUnblocking(item)}
                  >
                    Розблокувати
                  </button>
                </li>
              ) : (
                <li className="safety-row safety-row--stack" key={item.id}>
                  <div className="safety-row__heading">
                    <h3>{fullName(item.reportedUser)}</h3>
                    <span className={`safety-badge safety-badge--${item.status.toLowerCase()}`}>
                      {reportStatuses[item.status]}
                    </span>
                  </div>
                  <p className="safety-text">{item.reason}</p>
                  <p className="profile-hint">
                    Надіслано {formatDateTime(item.createdAt)}
                    {item.resolvedAt ? ` · Розглянуто ${formatDateTime(item.resolvedAt)}` : ""}
                  </p>
                </li>
              ),
            )}
          </ul>
          <Pagination
            page={data.page}
            pages={Math.max(pageCount(data.total, data.pageSize), data.page)}
            hrefFor={(number) => settingsHref(tab, number)}
          />
        </>
      )}
      {unblocking && (
        <UnblockDialog
          block={unblocking}
          csrfToken={csrfToken}
          path={settingsHref(tab, page)}
          onClose={() => setUnblocking(null)}
          onDone={() => {
            setUnblocking(null);
            setNotice("Студента розблоковано. Скасовані обміни не відновлено.");
            query.retry();
          }}
        />
      )}
    </section>
  );
}

function UnblockDialog({
  block,
  csrfToken,
  path,
  onClose,
  onDone,
}: {
  block: Block;
  csrfToken: string;
  path: string;
  onClose: () => void;
  onDone: () => void;
}) {
  const mutation = useSafetyMutation();
  return (
    <AppDialog
      title="Розблокувати студента?"
      badge={fullName(block.user)}
      description="Скасовані обміни й закриті запити не відновляться. Нова взаємодія залежатиме від налаштувань приватності обох студентів."
      canClose={!mutation.pending}
      onClose={onClose}
    >
      <SafetyFeedback error={mutation.error} path={path} />
      <div className="safety-actions safety-actions--end">
        <button
          className="dashboard-button"
          type="button"
          data-autofocus
          disabled={mutation.pending}
          onClick={onClose}
        >
          Скасувати
        </button>
        <button
          className="dashboard-button dashboard-button--primary"
          type="button"
          disabled={mutation.pending}
          onClick={async () => {
            const result = await mutation.run(() => unblockStudent(block.user.id, csrfToken));
            if (result.ok) onDone();
          }}
        >
          {mutation.pending ? "Зберігаємо…" : "Підтвердити розблокування"}
        </button>
      </div>
    </AppDialog>
  );
}
