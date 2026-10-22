"use client";

import { useState } from "react";
import { AppDialog } from "./AppDialog";
import { moderateAccount } from "./AdminApi";
import { invalidFields, textError } from "./SafetyApi";
import { FieldError, SafetyFeedback } from "./SafetyFeedback";
import { useSafetyMutation } from "./useSafetyMutation";

export function ModerateAccount({
  userId,
  viewerId,
  name,
  status,
  csrfToken,
  onChanged,
}: {
  userId: string;
  viewerId: string;
  name?: string;
  status?: "ACTIVE" | "SUSPENDED";
  csrfToken: string;
  onChanged: (notice: string) => void;
}) {
  const [action, setAction] = useState<"suspend" | "restore" | null>(null);
  const [notice, setNotice] = useState("");
  return (
    <section className="account-card safety-card">
      <h2>Керування акаунтом</h2>
      <p className="safety-text">{name ?? userId}</p>
      {status && (
        <p className="profile-hint">Стан: {status === "ACTIVE" ? "Активний" : "Призупинений"}</p>
      )}
      {userId === viewerId ? (
        <p className="safety-info">
          Адміністратор не може призупинити або відновити власний акаунт.
        </p>
      ) : (
        <div className="safety-actions">
          {status !== "SUSPENDED" && (
            <button
              className="dashboard-button dashboard-button--danger"
              type="button"
              aria-haspopup="dialog"
              onClick={() => setAction("suspend")}
            >
              Призупинити акаунт
            </button>
          )}
          {status !== "ACTIVE" && (
            <button
              className="dashboard-button"
              type="button"
              aria-haspopup="dialog"
              onClick={() => setAction("restore")}
            >
              Відновити акаунт
            </button>
          )}
        </div>
      )}
      {notice && (
        <p className="profile-success" role="status">
          {notice}
        </p>
      )}
      {action && (
        <AccountActionDialog
          action={action}
          name={name ?? userId}
          userId={userId}
          csrfToken={csrfToken}
          onClose={() => setAction(null)}
          onDone={(text) => {
            setNotice(text);
            setAction(null);
            onChanged(text);
          }}
        />
      )}
    </section>
  );
}

function AccountActionDialog({
  action,
  name,
  userId,
  csrfToken,
  onClose,
  onDone,
}: {
  action: "suspend" | "restore";
  name: string;
  userId: string;
  csrfToken: string;
  onClose: () => void;
  onDone: (text: string) => void;
}) {
  const [reason, setReason] = useState("");
  const mutation = useSafetyMutation();
  return (
    <AppDialog
      title={action === "suspend" ? "Призупинити акаунт?" : "Відновити акаунт?"}
      badge={name}
      description={
        action === "suspend"
          ? "Усі сесії користувача буде завершено, новий вхід заборонено, активні обміни скасовано та очікувані запити закрито. Історія збережеться."
          : "Користувач знову зможе увійти. Скасовані обміни не відновляться."
      }
      canClose={!mutation.pending}
      onClose={onClose}
    >
      <form
        className="safety-form"
        noValidate
        onSubmit={async (event) => {
          event.preventDefault();
          const form = event.currentTarget;
          if (!mutation.validate(form, invalidFields({ reason: textError(reason, 1000) }))) return;
          const result = await mutation.run(
            () => moderateAccount(userId, action, reason, csrfToken),
            form,
          );
          if (result.ok)
            onDone(
              result.value.action
                ? action === "suspend"
                  ? "Акаунт призупинено. Дію записано в журнал."
                  : "Акаунт відновлено. Дію записано в журнал."
                : "Акаунт уже має потрібний стан. Нового запису в журналі не створено.",
            );
        }}
      >
        <label className="safety-field">
          Причина рішення
          <textarea
            name="reason"
            rows={4}
            value={reason}
            disabled={mutation.pending}
            aria-invalid={Boolean(mutation.fields.reason)}
            aria-describedby="account-reason-error account-reason-count"
            onChange={(event) => setReason(event.target.value)}
          />
          <span className="profile-hint" id="account-reason-count">
            {Array.from(reason.trim()).length} / 1000
          </span>
          <FieldError id="account-reason-error" message={mutation.fields.reason} />
        </label>
        <SafetyFeedback error={mutation.error} path="/admin?tab=history" />
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
          <button
            className={`dashboard-button ${action === "suspend" ? "dashboard-button--danger" : "dashboard-button--primary"}`}
            disabled={mutation.pending}
          >
            {mutation.pending ? "Зберігаємо…" : "Підтвердити рішення"}
          </button>
        </div>
      </form>
    </AppDialog>
  );
}
