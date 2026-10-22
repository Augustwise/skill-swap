"use client";

import Link from "next/link";
import { AuthError } from "./AuthApi";
import { loginHref } from "./AppRoutes";
import { safetyError } from "./SafetyApi";

export function SafetyFeedback({
  error,
  path,
  onRetry,
}: {
  error: unknown;
  path: string;
  onRetry?: () => void;
}) {
  if (!error) return null;
  const login = error instanceof AuthError && error.status === 401;
  const forbidden =
    error instanceof AuthError && ["admin_required", "report_not_found"].includes(error.code);
  return (
    <div className="safety-feedback" role="alert">
      <p>{safetyError(error)}</p>
      {error instanceof AuthError && error.code === "report_already_open" && (
        <Link href="/settings?tab=reports" className="dashboard-button">
          Переглянути мої скарги
        </Link>
      )}
      {login ? (
        <Link className="dashboard-button" href={loginHref(path)}>
          Увійти знову
        </Link>
      ) : onRetry && !forbidden ? (
        <button className="dashboard-button" type="button" onClick={onRetry}>
          Спробувати ще раз
        </button>
      ) : null}
    </div>
  );
}

export function FieldError({ id, message }: { id: string; message?: string }) {
  return message ? (
    <span className="safety-field-error" id={id}>
      {message}
    </span>
  ) : null;
}
