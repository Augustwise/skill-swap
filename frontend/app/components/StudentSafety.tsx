"use client";

import Link from "next/link";
import { useState } from "react";
import { AppDialog } from "./AppDialog";
import { useCountersChanged } from "./AppShell";
import type { Person } from "./DiscoveryApi";
import { fullName } from "./DiscoveryFormat";
import {
  blockStudent,
  invalidFields,
  reportStudent,
  settingsHref,
  textError,
  unblockStudent,
} from "./SafetyApi";
import { FieldError, SafetyFeedback } from "./SafetyFeedback";
import { useSafetyMutation } from "./useSafetyMutation";

export function StudentSafety({
  student,
  csrfToken,
  path,
  blockedByMe = false,
  onChanged,
}: {
  student: Person;
  csrfToken: string;
  path: string;
  blockedByMe?: boolean;
  onChanged?: (notice: string) => void;
}) {
  const [dialog, setDialog] = useState<"block" | "report" | "unblock" | null>(null);
  const [notice, setNotice] = useState("");
  const [blocked, setBlocked] = useState(blockedByMe);
  const countersChanged = useCountersChanged();
  return (
    <div className="student-safety">
      <div className="safety-actions">
        <button
          type="button"
          className="dashboard-button"
          aria-haspopup="dialog"
          onClick={() => setDialog("report")}
        >
          Поскаржитися
        </button>
        <button
          type="button"
          className="dashboard-button dashboard-button--danger"
          aria-haspopup="dialog"
          onClick={() => setDialog(blocked ? "unblock" : "block")}
        >
          {blocked ? "Розблокувати" : "Заблокувати"}
        </button>
      </div>
      {notice && (
        <p className="safety-notice" role="status">
          {notice} <Link href={settingsHref(blocked ? "blocks" : "reports")}>До налаштувань</Link>
        </p>
      )}
      {dialog && (
        <StudentSafetyDialog
          key={dialog}
          action={dialog}
          student={student}
          path={path}
          csrfToken={csrfToken}
          onClose={() => setDialog(null)}
          onDone={(text) => {
            setNotice(text);
            if (dialog !== "report") {
              setBlocked(dialog === "block");
              countersChanged();
            }
            setDialog(null);
            if (dialog !== "report") onChanged?.(text);
          }}
        />
      )}
    </div>
  );
}

function StudentSafetyDialog({
  action,
  student,
  path,
  csrfToken,
  onClose,
  onDone,
}: {
  action: "block" | "report" | "unblock";
  student: Person;
  path: string;
  csrfToken: string;
  onClose: () => void;
  onDone: (notice: string) => void;
}) {
  const [reason, setReason] = useState("");
  const mutation = useSafetyMutation();
  const report = action === "report";
  const title = report
    ? "Скарга на студента"
    : action === "block"
      ? "Заблокувати студента?"
      : "Розблокувати студента?";
  return (
    <AppDialog
      title={title}
      badge={fullName(student)}
      canClose={!mutation.pending}
      onClose={onClose}
      description={
        report
          ? "Опиши ситуацію. Адміністратор розгляне скаргу, а її статус буде в налаштуваннях. Скарга сама по собі не блокує студента."
          : action === "block"
            ? "Нові запити й повідомлення між вами стануть недоступними. Усі спільні активні обміни та майбутні заняття буде скасовано, очікувані запити — закрито. Історія збережеться."
            : "Ви знову зможете знаходити одне одного й починати нові обміни, якщо це дозволяють налаштування приватності. Скасовані обміни не відновляться."
      }
    >
      <form
        className="safety-form"
        noValidate
        onSubmit={async (event) => {
          event.preventDefault();
          const form = event.currentTarget;
          if (
            report &&
            !mutation.validate(form, invalidFields({ reason: textError(reason, 1000) }))
          )
            return;
          if (report) {
            const result = await mutation.run(
              () => reportStudent(student.id, reason, csrfToken),
              form,
            );
            if (result.ok) onDone("Скаргу надіслано. Її статус доступний у розділі «Мої скарги».");
          } else if (action === "block") {
            const result = await mutation.run(() => blockStudent(student.id, csrfToken), form);
            if (result.ok)
              onDone(
                `Студента заблоковано. Скасовано обмінів: ${result.value.cancelledExchanges}; закрито запитів: ${result.value.closedRequests}.`,
              );
          } else {
            const result = await mutation.run(() => unblockStudent(student.id, csrfToken), form);
            if (result.ok) onDone("Студента розблоковано. Попередні обміни не відновлено.");
          }
        }}
      >
        {report && (
          <label className="safety-field">
            Причина скарги
            <textarea
              name="reason"
              value={reason}
              rows={5}
              disabled={mutation.pending}
              data-autofocus
              aria-invalid={Boolean(mutation.fields.reason)}
              aria-describedby="report-reason-error report-counter"
              onChange={(event) => setReason(event.target.value)}
            />
            <span className="profile-hint" id="report-counter">
              {Array.from(reason.trim()).length} / 1000
            </span>
            <FieldError id="report-reason-error" message={mutation.fields.reason} />
          </label>
        )}
        <SafetyFeedback error={mutation.error} path={path} />
        <div className="safety-actions safety-actions--end">
          <button
            className="dashboard-button"
            type="button"
            data-autofocus={!report || undefined}
            disabled={mutation.pending}
            onClick={onClose}
          >
            Скасувати
          </button>
          <button
            className={`dashboard-button ${action === "block" ? "dashboard-button--danger" : "dashboard-button--primary"}`}
            disabled={mutation.pending}
          >
            {mutation.pending
              ? "Зберігаємо…"
              : report
                ? "Надіслати скаргу"
                : action === "block"
                  ? "Підтвердити блокування"
                  : "Підтвердити розблокування"}
          </button>
        </div>
      </form>
    </AppDialog>
  );
}
