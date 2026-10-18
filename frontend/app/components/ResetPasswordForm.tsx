"use client";

import Image from "next/image";
import Link from "next/link";
import { useRef, useState, type FormEvent } from "react";
import { AuthError, authRequest, requestError } from "./AuthApi";
import { focusInvalid, passwordError } from "./AuthValidation";

export function ResetPasswordForm({ token }: { token: string }) {
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const [complete, setComplete] = useState(false);
  const [showPassword, setShowPassword] = useState(false);
  const [showConfirmation, setShowConfirmation] = useState(false);
  const [invalidLink, setInvalidLink] = useState(false);
  const submitting = useRef(false);

  const clearFieldError = (field: string) => {
    if (fieldErrors[field]) {
      setFieldErrors((prev) => {
        const next = { ...prev };
        delete next[field];
        return next;
      });
    }
  };

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (submitting.current || invalidLink) return;
    const formElement = event.currentTarget;
    setError("");
    setFieldErrors({});

    const form = new FormData(event.currentTarget);
    const password = String(form.get("password") ?? "");
    const confirmation = String(form.get("confirmation") ?? "");

    const newFieldErrors: Record<string, string> = {};
    const invalidPassword = passwordError(password);
    if (invalidPassword) newFieldErrors.password = invalidPassword;

    if (!confirmation) {
      newFieldErrors.confirmation = "Повтори новий пароль.";
    } else if (password && confirmation && password !== confirmation) {
      newFieldErrors.confirmation = "Паролі не збігаються.";
    }

    if (Object.keys(newFieldErrors).length > 0) {
      setFieldErrors(newFieldErrors);
      focusInvalid(formElement, newFieldErrors);
      return;
    }

    submitting.current = true;
    setPending(true);
    try {
      await authRequest("auth/reset-password", {
        method: "POST",
        body: JSON.stringify({ token, password }),
      });
      formElement.reset();
      window.history.replaceState(null, "", "/reset-password");
      setComplete(true);
      window.requestAnimationFrame(() => document.getElementById("reset-title")?.focus());
    } catch (error) {
      if (error instanceof AuthError && error.code === "invalid_token") {
        formElement.reset();
        window.history.replaceState(null, "", "/reset-password");
        setInvalidLink(true);
        window.requestAnimationFrame(() => document.getElementById("reset-title")?.focus());
      } else if (error instanceof AuthError && error.fields.password) {
        const fields = {
          password: "Використай щонайменше 12 символів і не більше 72 байтів UTF-8.",
        };
        setFieldErrors(fields);
        focusInvalid(formElement, fields);
      } else setError(requestError(error));
    } finally {
      submitting.current = false;
      setPending(false);
    }
  }

  if (invalidLink) return <ResetLinkNotice />;

  if (complete) {
    return (
      <div className="register-form register-success" role="status">
        <h1 id="reset-title" tabIndex={-1}>
          Пароль змінено
        </h1>
        <p>Усі попередні сесії завершено. Увійди з новим паролем.</p>
        <Link className="register-submit register-success__link" href="/login">
          Увійти
        </Link>
      </div>
    );
  }

  return (
    <form className="register-form" noValidate onSubmit={handleSubmit} aria-busy={pending}>
      <div className="register-form__heading">
        <h1 id="reset-title">Новий пароль</h1>
        <p>Використай щонайменше 12 символів.</p>
      </div>
      <div className="register-field register-field--password">
        <label htmlFor="reset-password">Новий пароль</label>
        <input
          id="reset-password"
          name="password"
          type={showPassword ? "text" : "password"}
          autoComplete="new-password"
          aria-invalid={Boolean(fieldErrors.password)}
          aria-describedby="reset-password-hint"
          onChange={() => clearFieldError("password")}
          required
        />
        <button
          type="button"
          className="register-field__eye"
          onClick={() => setShowPassword((value) => !value)}
          aria-label={showPassword ? "Приховати пароль" : "Показати пароль"}
          aria-pressed={showPassword}
        >
          <Image
            src={`/figma/register/${showPassword ? "eye-off" : "eye"}.svg`}
            alt=""
            width={17}
            height={17}
          />
        </button>
        <small
          id="reset-password-hint"
          className={fieldErrors.password ? "register-field__error" : "auth-hint"}
        >
          {fieldErrors.password || "Щонайменше 12 символів, не більше 72 байтів UTF-8."}
        </small>
      </div>
      <div className="register-field register-field--password">
        <label htmlFor="reset-confirmation">Повтори пароль</label>
        <input
          id="reset-confirmation"
          name="confirmation"
          type={showConfirmation ? "text" : "password"}
          autoComplete="new-password"
          aria-invalid={Boolean(fieldErrors.confirmation)}
          aria-describedby={fieldErrors.confirmation ? "reset-confirmation-error" : undefined}
          onChange={() => clearFieldError("confirmation")}
          required
        />
        <button
          type="button"
          className="register-field__eye"
          onClick={() => setShowConfirmation((value) => !value)}
          aria-label={showConfirmation ? "Приховати пароль" : "Показати пароль"}
          aria-pressed={showConfirmation}
        >
          <Image
            src={`/figma/register/${showConfirmation ? "eye-off" : "eye"}.svg`}
            alt=""
            width={17}
            height={17}
          />
        </button>
        {fieldErrors.confirmation && (
          <small id="reset-confirmation-error" className="register-field__error">
            {fieldErrors.confirmation}
          </small>
        )}
      </div>
      {error && (
        <p className="register-form__error" role="alert">
          {error}
        </p>
      )}
      <button className="register-submit" type="submit" disabled={pending}>
        {pending ? "Зберігаємо…" : "Зберегти пароль"}
      </button>
      <Link className="login-form__back" href="/login">
        Повернутися до входу
      </Link>
    </form>
  );
}

export function ResetLinkNotice() {
  return (
    <div className="register-form register-success" role="alert">
      <h1 id="reset-title" tabIndex={-1}>
        Посилання недійсне
      </h1>
      <p>
        Посилання вже використане, прострочене або пошкоджене. Запроси нове, щоб змінити пароль.
      </p>
      <Link className="register-submit register-success__link" href="/login?mode=recovery">
        Отримати нове посилання
      </Link>
    </div>
  );
}
