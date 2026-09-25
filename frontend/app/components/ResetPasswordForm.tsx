"use client";

import Image from "next/image";
import Link from "next/link";
import { useState, type FormEvent } from "react";

export function ResetPasswordForm({ token }: { token: string }) {
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const [complete, setComplete] = useState(false);
  const [showPassword, setShowPassword] = useState(false);
  const [showConfirmation, setShowConfirmation] = useState(false);

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
    setError("");
    setFieldErrors({});

    const form = new FormData(event.currentTarget);
    const password = String(form.get("password") ?? "");
    const confirmation = String(form.get("confirmation") ?? "");

    const newFieldErrors: Record<string, string> = {};
    if (!password) {
      newFieldErrors.password = "Введи новий пароль.";
    } else if (password.length < 12) {
      newFieldErrors.password = "Пароль має містити щонайменше 12 символів.";
    } else if (new TextEncoder().encode(password).length > 72) {
      newFieldErrors.password = "Пароль не може перевищувати 72 байти.";
    }

    if (!confirmation) {
      newFieldErrors.confirmation = "Повтори новий пароль.";
    } else if (password && confirmation && password !== confirmation) {
      newFieldErrors.confirmation = "Паролі не збігаються.";
    }

    if (Object.keys(newFieldErrors).length > 0) {
      setFieldErrors(newFieldErrors);
      const firstField = ["password", "confirmation"].find((f) => newFieldErrors[f]);
      if (firstField) {
        (event.currentTarget.elements.namedItem(firstField) as HTMLElement | null)?.focus();
      }
      return;
    }

    setPending(true);
    try {
      const response = await fetch("/api/v1/auth/reset-password", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ token, password }),
      });
      if (!response.ok) {
        setError(
          response.status === 400
            ? "Посилання недійсне або прострочене. Запроси нове через сторінку входу."
            : "Не вдалося змінити пароль. Спробуй ще раз.",
        );
        return;
      }
      setComplete(true);
    } catch {
      setError("Не вдалося зв’язатися із сервером. Перевір з’єднання і спробуй ще раз.");
    } finally {
      setPending(false);
    }
  }

  if (complete) {
    return (
      <div className="register-form register-success" role="status">
        <h1 id="reset-title">Пароль змінено</h1>
        <p>Тепер можеш увійти з новим паролем.</p>
        <Link className="register-submit register-success__link" href="/login">
          Увійти
        </Link>
      </div>
    );
  }

  return (
    <form className="register-form" noValidate onSubmit={handleSubmit}>
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
          minLength={12}
          aria-invalid={Boolean(fieldErrors.password)}
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
        {fieldErrors.password && (
          <small className="register-field__error">{fieldErrors.password}</small>
        )}
      </div>
      <div className="register-field register-field--password">
        <label htmlFor="reset-confirmation">Повтори пароль</label>
        <input
          id="reset-confirmation"
          name="confirmation"
          type={showConfirmation ? "text" : "password"}
          autoComplete="new-password"
          minLength={12}
          aria-invalid={Boolean(fieldErrors.confirmation)}
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
          <small className="register-field__error">{fieldErrors.confirmation}</small>
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
    </form>
  );
}
