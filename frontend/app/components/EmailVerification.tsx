"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import { AuthError, authRequest, requestError, type AuthSession } from "./AuthApi";

type Status = "loading" | "waiting" | "login" | "verified" | "invalid" | "error";

export function EmailVerification({
  token = "",
  invalidToken = false,
}: {
  token?: string;
  invalidToken?: boolean;
}) {
  const router = useRouter();
  const [status, setStatus] = useState<Status>("loading");
  const [session, setSession] = useState<AuthSession | null>(null);
  const [attempt, setAttempt] = useState(0);
  const [action, setAction] = useState<"resend" | "logout" | null>(null);
  const [cooldown, setCooldown] = useState(0);
  const [notice, setNotice] = useState("");
  const [error, setError] = useState("");
  const verification = useRef<Promise<unknown> | null>(null);
  const resending = useRef(false);
  const confirmed = useRef(false);
  const invalid = useRef(invalidToken);
  const pending = action !== null;

  useEffect(() => {
    if (!cooldown) return;
    const timer = window.setTimeout(() => setCooldown((value) => Math.max(0, value - 1)), 1000);
    return () => window.clearTimeout(timer);
  }, [cooldown]);

  useEffect(() => {
    const controller = new AbortController();
    async function load() {
      if (token && !confirmed.current && !invalid.current) {
        verification.current ??= authRequest("auth/verify-email", {
          method: "POST",
          body: JSON.stringify({ token }),
        });
        try {
          await verification.current;
          confirmed.current = true;
        } catch (error) {
          if (controller.signal.aborted) return;
          if (!(error instanceof AuthError) || error.code !== "invalid_token") {
            verification.current = null;
            setStatus("error");
            return;
          }
          invalid.current = true;
        }
        if (controller.signal.aborted) return;
        window.history.replaceState(null, "", "/verify-email");
      }
      if (invalidToken) window.history.replaceState(null, "", "/verify-email");
      try {
        const current = await authRequest<AuthSession>("auth/me", {
          signal: controller.signal,
          cache: "no-store",
        });
        if (controller.signal.aborted) return;
        if (current.user.emailVerified) {
          router.replace("/onboarding");
          return;
        }
        setSession(current);
        setStatus(invalid.current ? "invalid" : "waiting");
      } catch (error) {
        if (controller.signal.aborted) return;
        setSession(null);
        setStatus(
          error instanceof AuthError && error.status === 401
            ? confirmed.current
              ? "verified"
              : invalid.current
                ? "invalid"
                : "login"
            : "error",
        );
      }
    }
    void load();
    return () => controller.abort();
  }, [attempt, router, token, invalidToken]);

  function checkVerification() {
    setStatus("loading");
    setError("");
    setNotice("");
    setAttempt((value) => value + 1);
  }

  async function resend() {
    if (!session || resending.current || cooldown) return;
    resending.current = true;
    setAction("resend");
    setError("");
    setNotice("");
    try {
      const current = await authRequest<AuthSession>("auth/me");
      setSession(current);
      if (current.user.emailVerified) {
        router.replace("/onboarding");
        return;
      }
      await authRequest("auth/resend-verification", { method: "POST" }, current.csrfToken);
      setStatus("waiting");
      setCooldown(30);
      setNotice("Лист надіслано повторно. Перевір вхідні та папку «Спам».");
    } catch (error) {
      if (error instanceof AuthError && error.code === "email_already_verified") {
        checkVerification();
      } else if (error instanceof AuthError && error.status === 401) {
        setSession(null);
        setStatus("login");
      } else {
        setError(requestError(error));
      }
    } finally {
      resending.current = false;
      setAction(null);
    }
  }

  async function logout() {
    if (resending.current) return;
    resending.current = true;
    setAction("logout");
    setError("");
    try {
      const current = await authRequest<AuthSession>("auth/me");
      await authRequest("auth/logout", { method: "POST" }, current.csrfToken);
      router.replace("/login");
    } catch (error) {
      if (error instanceof AuthError && error.status === 401) router.replace("/login");
      else setError(requestError(error));
    } finally {
      resending.current = false;
      setAction(null);
    }
  }

  const title =
    status === "loading"
      ? "Перевіряємо підтвердження…"
      : status === "verified"
        ? "Пошту підтверджено"
        : status === "invalid"
          ? "Посилання не працює"
          : status === "error"
            ? "Не вдалося перевірити пошту"
            : "Підтвердь свою пошту";

  return (
    <div className="register-form register-success" aria-busy={status === "loading" || pending}>
      <span className="auth-status-label">Підтвердження акаунта</span>
      <h1 id="verification-title">{title}</h1>
      <p role="status">
        {status === "loading"
          ? "Зачекай, поки ми перевіримо статус твоєї пошти."
          : status === "verified"
            ? "Увійди в акаунт, щоб заповнити профіль і додати навички."
            : status === "invalid"
              ? session
                ? "Посилання недійсне, вже використане або термін його дії минув. Надішли новий лист для підтвердження."
                : "Посилання недійсне, вже використане або термін його дії минув. Увійди та надішли новий лист."
              : status === "error"
                ? "Перевір з’єднання із сервером і спробуй ще раз."
                : session
                  ? `Відкрий посилання в листі на ${session.user.email}. Онбординг буде доступний після підтвердження пошти. Перевір також папку «Спам».`
                  : "Відкрий посилання в листі для підтвердження пошти. Увійди в акаунт, щоб перевірити статус або надіслати лист повторно."}
      </p>
      {notice && <p role="status">{notice}</p>}
      {error && (
        <p className="register-form__error" role="alert">
          {error}
        </p>
      )}
      {(status === "waiting" || status === "invalid") && session && (
        <>
          <button
            className="register-submit"
            type="button"
            disabled={pending}
            onClick={checkVerification}
          >
            Я підтвердив(-ла) пошту
          </button>
          <button
            className="login-form__back"
            type="button"
            disabled={pending || cooldown > 0}
            onClick={() => void resend()}
          >
            {action === "resend"
              ? "Надсилаємо лист…"
              : cooldown
                ? `Повторно через ${cooldown} с.`
                : "Надіслати лист повторно"}
          </button>
        </>
      )}
      {status === "error" && (
        <button className="register-submit" type="button" onClick={checkVerification}>
          Спробувати ще раз
        </button>
      )}
      {session && status !== "loading" && status !== "error" ? (
        <button
          className="login-form__back"
          type="button"
          disabled={pending}
          onClick={() => void logout()}
        >
          {action === "logout" ? "Виходимо…" : "Вийти з акаунта"}
        </button>
      ) : (
        status !== "loading" && (
          <p className="register-form__login">
            <Link href="/login?next=/onboarding">Перейти до входу</Link>
          </p>
        )
      )}
    </div>
  );
}
