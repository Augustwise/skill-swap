"use client";

import Image from "next/image";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useRef, useState, type FormEvent } from "react";
import { AuthError, authRequest, requestError, type AuthSession } from "./AuthApi";
import { emailError, focusInvalid, loginDestination, normalizeEmail } from "./AuthValidation";
import type { Profile } from "./OnboardingApi";
type Mode = "login" | "recovery" | "recoverySent";

export function LoginForm({
  initialMode = "login",
  accountDeleted = false,
}: {
  initialMode?: "login" | "recovery";
  accountDeleted?: boolean;
}) {
  const router = useRouter();
  const [mode, setMode] = useState<Mode>(initialMode);
  const [email, setEmail] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [rememberMe, setRememberMe] = useState(true);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
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

  const switchMode = (nextMode: Mode) => {
    if (submitting.current) return;
    setError("");
    setFieldErrors({});
    setMode(nextMode);
    window.requestAnimationFrame(() => document.getElementById("login-title")?.focus());
  };

  async function handleLogin(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (submitting.current) return;
    const formElement = event.currentTarget;
    setError("");
    setFieldErrors({});

    const emailVal = normalizeEmail(email);
    const form = new FormData(event.currentTarget);
    const password = String(form.get("password") ?? "");

    const newFieldErrors: Record<string, string> = {};
    const invalidEmail = emailError(emailVal);
    if (invalidEmail) newFieldErrors.email = invalidEmail;

    if (!password) {
      newFieldErrors.password = "Введи пароль.";
    }

    if (Object.keys(newFieldErrors).length > 0) {
      setFieldErrors(newFieldErrors);
      focusInvalid(formElement, newFieldErrors);
      return;
    }

    submitting.current = true;
    setPending(true);
    try {
      const data = await authRequest<AuthSession>("auth/login", {
        method: "POST",
        body: JSON.stringify({ email: emailVal, password, rememberMe }),
      });
      formElement.reset();
      if (!data.user.emailVerified) {
        router.replace("/verify-email");
        return;
      }
      const { profile } = await authRequest<{ profile: Profile }>("me/profile");
      router.replace(loginDestination(window.location.search, profile.eligibleForMatching));
    } catch (error) {
      setError(
        error instanceof AuthError && error.status === 401
          ? "Невірна пошта або пароль."
          : error instanceof AuthError && error.retryAfter
            ? `${error.message} Можна повторити через ${Math.ceil(error.retryAfter)} с.`
            : requestError(error),
      );
    } finally {
      submitting.current = false;
      setPending(false);
    }
  }

  async function handleRecovery(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (submitting.current) return;
    setError("");
    setFieldErrors({});

    const emailVal = normalizeEmail(email);
    const newFieldErrors: Record<string, string> = {};
    const invalidEmail = emailError(emailVal);
    if (invalidEmail) newFieldErrors.email = invalidEmail;

    if (Object.keys(newFieldErrors).length > 0) {
      setFieldErrors(newFieldErrors);
      focusInvalid(event.currentTarget, newFieldErrors);
      return;
    }

    submitting.current = true;
    setPending(true);
    try {
      await authRequest("auth/forgot-password", {
        method: "POST",
        body: JSON.stringify({ email: emailVal }),
      });
      setEmail(emailVal);
      setMode("recoverySent");
      window.requestAnimationFrame(() => document.getElementById("login-title")?.focus());
    } catch (error) {
      setError(requestError(error));
    } finally {
      submitting.current = false;
      setPending(false);
    }
  }

  if (mode === "recoverySent") {
    return (
      <div className="register-form register-success" role="status">
        <h1 id="login-title" tabIndex={-1}>
          Перевір пошту
        </h1>
        <p>Якщо акаунт для {email} існує, ми надіслали лист для відновлення пароля.</p>
        <button className="register-submit" type="button" onClick={() => switchMode("login")}>
          Повернутися до входу
        </button>
      </div>
    );
  }

  if (mode === "recovery") {
    return (
      <form className="register-form" noValidate onSubmit={handleRecovery} aria-busy={pending}>
        <div className="register-form__heading">
          <h1 id="login-title" tabIndex={-1}>
            Відновити пароль
          </h1>
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
            aria-describedby={fieldErrors.email ? "recovery-email-error" : undefined}
            required
          />
          {fieldErrors.email && (
            <small id="recovery-email-error" className="register-field__error">
              {fieldErrors.email}
            </small>
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
        <button
          className="login-form__back"
          type="button"
          disabled={pending}
          onClick={() => switchMode("login")}
        >
          Повернутися до входу
        </button>
      </form>
    );
  }

  return (
    <form
      className="register-form login-form"
      noValidate
      onSubmit={handleLogin}
      aria-busy={pending}
    >
      <div className="register-form__heading">
        <h1 id="login-title" tabIndex={-1}>
          З поверненням
        </h1>
        <p>Увійди, щоб побачити свої нові збіги</p>
      </div>
      {accountDeleted && (
        <p className="login-form__notice" role="status">
          Акаунт видалено, профіль знеособлено. Усі сесії завершено.
        </p>
      )}
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
          aria-describedby={fieldErrors.email ? "login-email-error" : undefined}
          required
        />
        {fieldErrors.email && (
          <small id="login-email-error" className="register-field__error">
            {fieldErrors.email}
          </small>
        )}
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
          aria-describedby={fieldErrors.password ? "login-password-error" : undefined}
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
          <small id="login-password-error" className="register-field__error">
            {fieldErrors.password}
          </small>
        )}
      </div>
      <div className="login-form__options">
        <label className="login-form__remember">
          <input
            className="checkbox"
            type="checkbox"
            checked={rememberMe}
            onChange={(event) => setRememberMe(event.target.checked)}
          />
          <span>Запам&apos;ятати мене</span>
        </label>
        <button type="button" disabled={pending} onClick={() => switchMode("recovery")}>
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
        disabled={pending}
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
