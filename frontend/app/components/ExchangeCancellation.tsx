"use client";

import { useId, useState, type FormEvent } from "react";
import { AppDialog } from "./AppDialog";
import { cancellationError } from "./CompletionApi";
import { CompletionFeedback } from "./CompletionFeedback";

export function ExchangeCancellation({
  pending,
  error,
  path,
  onClose,
  onSubmit,
}: {
  pending: boolean;
  error: unknown;
  path: string;
  onClose: () => void;
  onSubmit: (reason: string) => void;
}) {
  const id = useId();
  const [reason, setReason] = useState("");
  const [invalid, setInvalid] = useState("");

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (pending) return;
    const message = cancellationError(reason);
    setInvalid(message);
    if (message) {
      event.currentTarget.querySelector("textarea")?.focus();
      return;
    }
    onSubmit(reason);
  }

  return (
    <AppDialog
      title="Скасувати обмін?"
      description="Непроведені заняття буде скасовано. Проведені заняття, погоджені умови, чат та історія збережуться. Повернути обмін в активний стан не можна."
      canClose={!pending}
      onClose={onClose}
      className="session-dialog"
    >
      <form className="session-form" onSubmit={submit} noValidate>
        <label className="session-field" htmlFor={id}>
          Причина скасування
          <textarea
            id={id}
            rows={4}
            value={reason}
            onChange={(event) => {
              setReason(event.target.value);
              setInvalid("");
            }}
            disabled={pending}
            required
            data-autofocus
            aria-invalid={Boolean(invalid)}
            aria-describedby={`${id}-hint${invalid ? ` ${id}-error` : ""}`}
          />
        </label>
        <p className="profile-hint" id={`${id}-hint`}>
          {Array.from(reason.trim()).length} / 500 символів. Причину побачить партнер.
        </p>
        {invalid && (
          <p className="exchange-field__error" id={`${id}-error`} role="alert">
            {invalid}
          </p>
        )}
        <CompletionFeedback error={error} notice="" path={path} />
        <div className="session-actions">
          <button className="dashboard-button" type="button" disabled={pending} onClick={onClose}>
            Залишити обмін
          </button>
          <button className="dashboard-button completion-danger" type="submit" disabled={pending}>
            {pending ? "Скасовуємо…" : "Скасувати обмін"}
          </button>
        </div>
      </form>
    </AppDialog>
  );
}
