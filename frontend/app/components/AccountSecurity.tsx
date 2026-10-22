"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { AppDialog } from "./AppDialog";
import { changePassword, deleteAccount, invalidFields, passwordFields } from "./SafetyApi";
import { FieldError, SafetyFeedback } from "./SafetyFeedback";
import { useSafetyMutation } from "./useSafetyMutation";

export function AccountSecurity({ csrfToken }: { csrfToken: string }) {
  const [deleting, setDeleting] = useState(false);
  return (
    <div className="profile-sections">
      <PasswordForm csrfToken={csrfToken} />
      <section className="account-card safety-card safety-card--danger">
        <h2>Видалення акаунта</h2>
        <p>
          Профіль буде знеособлено, вхід стане неможливим, усі сесії буде завершено. Активні обміни
          й майбутні заняття буде скасовано.
        </p>
        <p className="profile-hint">
          Історія для партнерів збережеться з автором «Видалений користувач», а твої повідомлення
          буде замінено на «Повідомлення видалено». Відновити акаунт не можна.
        </p>
        <div className="safety-actions">
          <button
            type="button"
            className="dashboard-button dashboard-button--danger"
            aria-haspopup="dialog"
            onClick={() => setDeleting(true)}
          >
            Видалити акаунт
          </button>
        </div>
      </section>
      {deleting && <DeleteAccountDialog csrfToken={csrfToken} onClose={() => setDeleting(false)} />}
    </div>
  );
}

function PasswordForm({ csrfToken }: { csrfToken: string }) {
  const [draft, setDraft] = useState({ currentPassword: "", newPassword: "", confirmation: "" });
  const [notice, setNotice] = useState("");
  const mutation = useSafetyMutation();
  const labels = {
    currentPassword: "Поточний пароль",
    newPassword: "Новий пароль",
    confirmation: "Повтори новий пароль",
  };
  return (
    <section className="account-card safety-card">
      <h2>Зміна пароля</h2>
      <p className="profile-hint" id="password-rules">
        Від 12 символів, не більше 72 байтів UTF-8. Після зміни всі інші сесії завершаться. Ця сесія
        залишиться активною.
      </p>
      <form
        className="safety-form"
        noValidate
        onSubmit={async (event) => {
          event.preventDefault();
          setNotice("");
          const form = event.currentTarget;
          if (
            !mutation.validate(
              form,
              passwordFields(draft.currentPassword, draft.newPassword, draft.confirmation),
            )
          )
            return;
          const result = await mutation.run(
            () => changePassword(draft.currentPassword, draft.newPassword, csrfToken),
            form,
          );
          if (result.ok) {
            setDraft({ currentPassword: "", newPassword: "", confirmation: "" });
            setNotice("Пароль змінено. Інші сесії завершено.");
          }
        }}
      >
        {(Object.keys(labels) as (keyof typeof labels)[]).map((name) => (
          <label key={name} className="safety-field">
            {labels[name]}
            <input
              name={name}
              type="password"
              autoComplete={name === "currentPassword" ? "current-password" : "new-password"}
              value={draft[name]}
              disabled={mutation.pending}
              aria-invalid={Boolean(mutation.fields[name])}
              aria-describedby={`password-rules ${name}-error`}
              onChange={(event) => {
                setDraft({ ...draft, [name]: event.target.value });
                setNotice("");
              }}
            />
            <FieldError id={`${name}-error`} message={mutation.fields[name]} />
          </label>
        ))}
        <SafetyFeedback error={mutation.error} path="/settings?tab=security" />
        {notice && (
          <p className="profile-success" role="status">
            {notice}
          </p>
        )}
        <div className="safety-actions">
          <button
            className="dashboard-button dashboard-button--primary"
            disabled={mutation.pending}
          >
            {mutation.pending ? "Змінюємо…" : "Змінити пароль"}
          </button>
        </div>
      </form>
    </section>
  );
}

function DeleteAccountDialog({ csrfToken, onClose }: { csrfToken: string; onClose: () => void }) {
  const [password, setPassword] = useState("");
  const [confirmed, setConfirmed] = useState(false);
  const mutation = useSafetyMutation();
  const router = useRouter();
  return (
    <AppDialog
      title="Остаточно видалити акаунт?"
      description="Цю дію не можна скасувати. Підтвердь видалення поточним паролем."
      canClose={!mutation.pending}
      onClose={onClose}
    >
      <form
        className="safety-form"
        noValidate
        onSubmit={async (event) => {
          event.preventDefault();
          const form = event.currentTarget;
          if (
            !mutation.validate(
              form,
              invalidFields({
                password: password ? "" : "Введи поточний пароль.",
                confirmed: confirmed ? "" : "Підтвердь, що розумієш наслідки.",
              }),
            )
          )
            return;
          const result = await mutation.run(() => deleteAccount(password, csrfToken), form);
          if (result.ok) {
            setPassword("");
            router.replace("/login?deleted=1");
          }
        }}
      >
        <label className="safety-field">
          Поточний пароль
          <input
            type="password"
            name="password"
            autoComplete="current-password"
            value={password}
            disabled={mutation.pending}
            aria-invalid={Boolean(mutation.fields.password)}
            aria-describedby="delete-password-error"
            onChange={(event) => setPassword(event.target.value)}
          />
          <FieldError id="delete-password-error" message={mutation.fields.password} />
        </label>
        <label className="safety-check">
          <input
            type="checkbox"
            name="confirmed"
            checked={confirmed}
            disabled={mutation.pending}
            aria-describedby="delete-confirmed-error"
            onChange={(event) => setConfirmed(event.target.checked)}
          />{" "}
          Я розумію, що акаунт буде видалено без можливості відновлення.
        </label>
        <FieldError id="delete-confirmed-error" message={mutation.fields.confirmed} />
        <SafetyFeedback error={mutation.error} path="/settings?tab=security" />
        <div className="safety-actions safety-actions--end">
          <button
            type="button"
            className="dashboard-button"
            data-autofocus
            disabled={mutation.pending}
            onClick={onClose}
          >
            Залишити акаунт
          </button>
          <button
            className="dashboard-button dashboard-button--danger"
            disabled={mutation.pending || !confirmed}
          >
            {mutation.pending ? "Видаляємо…" : "Підтвердити видалення"}
          </button>
        </div>
      </form>
    </AppDialog>
  );
}
