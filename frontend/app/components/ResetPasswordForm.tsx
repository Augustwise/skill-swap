"use client";

import Link from "next/link";
import { useState, type FormEvent } from "react";

export function ResetPasswordForm({ token }: { token: string }) {
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  const [complete, setComplete] = useState(false);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    const form = new FormData(event.currentTarget);
    const password = String(form.get("password") ?? "");
    const confirmation = String(form.get("confirmation") ?? "");
    if (password !== confirmation) {
      setError("Паролі не збігаються.");
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
    <form className="register-form" onSubmit={handleSubmit}>
      <div className="register-form__heading">
        <h1 id="reset-title">Новий пароль</h1>
        <p>Використай щонайменше 12 символів.</p>
      </div>
      <label className="register-field">
        <span>Новий пароль</span>
        <input
          name="password"
          type="password"
          autoComplete="new-password"
          minLength={12}
          required
        />
      </label>
      <label className="register-field">
        <span>Повтори пароль</span>
        <input
          name="confirmation"
          type="password"
          autoComplete="new-password"
          minLength={12}
          required
        />
      </label>
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
