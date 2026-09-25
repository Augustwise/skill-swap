"use client";

import Image from "next/image";
import Link from "next/link";
import { useState, type FormEvent } from "react";

type ApiProblem = { error?: { code?: string; fields?: Record<string, string> } };
type Mode = "login" | "recovery" | "recoverySent" | "signedIn";

export function LoginForm() {
  const [mode, setMode] = useState<Mode>("login");
  const [email, setEmail] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [rememberMe, setRememberMe] = useState(true);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});

  const clearFieldError = (field: string) => {
    if (fieldErrors[field]) {
      setFieldErrors((prev) => {
        const next = { ...prev };
        delete next[field];
        return next;
      });
    }
  };

  const switchMode = (nextMode: Mode) => {
    setError("");
    setFieldErrors({});
    setMode(nextMode);
  };

  async function handleLogin(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    setFieldErrors({});

    const emailVal = email.trim().toLowerCase();
    const form = new FormData(event.currentTarget);
    const password = String(form.get("password") ?? "");

    const newFieldErrors: Record<string, string> = {};
    if (!emailVal) {
      newFieldErrors.email = "Введи університетську пошту.";
    } else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(emailVal)) {
      newFieldErrors.email = "Введи коректну адресу пошти.";
    }

    if (!password) {
      newFieldErrors.password = "Введи пароль.";
    }

    if (Object.keys(newFieldErrors).length > 0) {
      setFieldErrors(newFieldErrors);
      const firstField = ["email", "password"].find((f) => newFieldErrors[f]);
      if (firstField) {
        (event.currentTarget.elements.namedItem(firstField) as HTMLElement | null)?.focus();
      }
      return;
    }

    setPending(true);
    try {
      const response = await fetch("/api/v1/auth/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        credentials: "same-origin",
        body: JSON.stringify({ email: emailVal, password, rememberMe }),
      });
      if (!response.ok) {
        const data: ApiProblem = await response.json();
        if (response.status === 401) setError("Невірна пошта або пароль.");
        else if (response.status === 429) setError("Забагато спроб входу. Спробуй пізніше.");
        else if (data.error?.code === "account_suspended") setError("Цей акаунт призупинено.");
        else if (data.error?.fields) setError("Перевір адресу пошти та пароль.");
        else setError("Не вдалося увійти. Спробуй ще раз.");
        return;
      }
      setMode("signedIn");
    } catch {
      setError("Не вдалося зв’язатися із сервером. Перевір з’єднання і спробуй ще раз.");
    } finally {
      setPending(false);
    }
  }

  async function handleRecovery(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    setFieldErrors({});

    const emailVal = email.trim().toLowerCase();
    const newFieldErrors: Record<string, string> = {};
    if (!emailVal) {
      newFieldErrors.email = "Введи університетську пошту.";
    } else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(emailVal)) {
      newFieldErrors.email = "Введи коректну адресу пошти.";
    }

    if (Object.keys(newFieldErrors).length > 0) {
      setFieldErrors(newFieldErrors);
      (event.currentTarget.elements.namedItem("email") as HTMLElement | null)?.focus();
      return;
    }

    setPending(true);
    try {
      const response = await fetch("/api/v1/auth/forgot-password", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email: emailVal }),
      });
      if (!response.ok) {
        setError("Не вдалося надіслати лист. Спробуй ще раз.");
        return;
      }
      setMode("recoverySent");
    } catch {
      setError("Не вдалося зв’язатися із сервером. Перевір з’єднання і спробуй ще раз.");
    } finally {
      setPending(false);
    }
  }

  if (mode === "signedIn") {
    return (
      <div className="register-form register-success" role="status">
        <h1 id="login-title">Вхід виконано</h1>
        <p>Ти увійшов в акаунт SkillSwap.</p>
        <Link className="register-submit register-success__link" href="/">
          На головну
        </Link>
      </div>
    );
  }

  if (mode === "recoverySent") {
    return (
      <div className="register-form register-success" role="status">
        <h1 id="login-title">Перевір пошту</h1>
        <p>Якщо акаунт для {email} існує, ми надіслали лист для відновлення пароля.</p>
        <button className="register-submit" type="button" onClick={() => switchMode("login")}>
          Повернутися до входу
        </button>
      </div>
    );
  }

  if (mode === "recovery") {
    return (
      <form className="register-form" noValidate onSubmit={handleRecovery}>
        <div className="register-form__heading">
          <h1 id="login-title">Відновити пароль</h1>
          <p>Вкажи університетську пошту, і ми надішлемо посилання.</p>
        </div>
        <label className="register-field">
          <span>Університетська пошта</span>
          <input
            name="email"
            type="email"
            placeholder="andrii.melnyk@edu.kpi.ua"
            autoComplete="email"
            maxLength={320}
            value={email}
            onChange={(event) => {
              setEmail(event.target.value);
              clearFieldError("email");
            }}
            aria-invalid={Boolean(fieldErrors.email)}
            required
          />
          {fieldErrors.email && (
            <small className="register-field__error">{fieldErrors.email}</small>
          )}
        </label>
        {error && (
          <p className="register-form__error" role="alert">
            {error}
          </p>
        )}
        <button className="register-submit" type="submit" disabled={pending}>
          {pending ? "Надсилаємо…" : "Надіслати посилання"}
        </button>
        <button className="login-form__back" type="button" onClick={() => switchMode("login")}>
          Повернутися до входу
        </button>
      </form>
    );
  }

  return (
    <form className="register-form login-form" noValidate onSubmit={handleLogin}>
      <div className="register-form__heading">
        <h1 id="login-title">З поверненням</h1>
        <p>Увійди, щоб побачити свої нові збіги</p>
      </div>
      <label className="register-field">
        <span>Університетська пошта</span>
        <input
          name="email"
          type="email"
          placeholder="andrii.melnyk@edu.kpi.ua"
          autoComplete="email"
          maxLength={320}
          value={email}
          onChange={(event) => {
            setEmail(event.target.value);
            clearFieldError("email");
          }}
          aria-invalid={Boolean(fieldErrors.email)}
          required
        />
        {fieldErrors.email && <small className="register-field__error">{fieldErrors.email}</small>}
      </label>
      <div className="register-field register-field--password">
        <label htmlFor="login-password">Пароль</label>
        <input
          id="login-password"
          name="password"
          type={showPassword ? "text" : "password"}
          placeholder="Введи пароль"
          autoComplete="current-password"
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
      <div className="login-form__options">
        <label className="login-form__remember">
          <input
            type="checkbox"
            checked={rememberMe}
            onChange={(event) => setRememberMe(event.target.checked)}
          />
          <span>Запам&apos;ятати мене</span>
        </label>
        <button type="button" onClick={() => switchMode("recovery")}>
          Забув пароль?
        </button>
      </div>
      {error && (
        <p className="register-form__error" role="alert">
          {error}
        </p>
      )}
      <button className="register-submit" type="submit" disabled={pending}>
        {pending ? "Входимо…" : "Увійти"}
      </button>
      <div className="login-form__divider" aria-hidden="true">
        <span>або</span>
      </div>
      <button
        className="login-form__university"
        type="button"
        onClick={() => setError("Вхід через акаунт університету поки недоступний.")}
      >
        <Image src="/figma/login/cap.svg" alt="" width={18} height={18} />
        Увійти через акаунт університету
      </button>
      <p className="register-form__login">
        Ще немає акаунта? <Link href="/register">Зареєструватися</Link>
      </p>
    </form>
  );
}
