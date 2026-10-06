"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import { OnboardingError, onboardingRequest, type AuthSession } from "./OnboardingApi";

type Status = "loading" | "waiting" | "login" | "verified" | "invalid" | "error";

export function EmailVerification() {
  const router = useRouter();
  const [status, setStatus] = useState<Status>("loading");
  const [session, setSession] = useState<AuthSession | null>(null);
  const [attempt, setAttempt] = useState(0);
  const [pending, setPending] = useState(false);
  const [notice, setNotice] = useState("");
  const [error, setError] = useState("");
  const verification = useRef<Promise<unknown> | null>(null);
  const resending = useRef(false);

  useEffect(() => {
    const controller = new AbortController();
    async function load() {
      const token = new URLSearchParams(window.location.search).get("token");
      let confirmed = false;
      let invalid = false;
      if (token) {
        verification.current ??= onboardingRequest("auth/verify-email", {
          method: "POST",
          body: JSON.stringify({ token }),
        });
        try {
          await verification.current;
          confirmed = true;
        } catch (error) {
          if (controller.signal.aborted) return;
          if (!(error instanceof OnboardingError) || error.code !== "invalid_token") {
            verification.current = null;
            setStatus("error");
            return;
          }
          invalid = true;
        }
        if (controller.signal.aborted) return;
        const url = new URL(window.location.href);
        url.searchParams.delete("token");
        window.history.replaceState(null, "", url.pathname + url.search);
      }
      try {
        const current = await onboardingRequest<AuthSession>("auth/me", {
          signal: controller.signal,
          cache: "no-store",
        });
        if (controller.signal.aborted) return;
        if (current.user.emailVerified) {
          router.replace("/onboarding");
          return;
        }
        setSession(current);
        setStatus(invalid ? "invalid" : "waiting");
      } catch (error) {
        if (controller.signal.aborted) return;
        setSession(null);
        setStatus(
          error instanceof OnboardingError && error.status === 401
            ? confirmed
              ? "verified"
              : invalid
                ? "invalid"
                : "login"
            : "error",
        );
      }
    }
    void load();
    return () => controller.abort();
  }, [attempt, router]);

  function checkVerification() {
    setStatus("loading");
    setError("");
    setNotice("");
    setAttempt((value) => value + 1);
  }

  async function resend() {
    if (!session || resending.current) return;
    resending.current = true;
    setPending(true);
    setError("");
    setNotice("");
    try {
      await onboardingRequest("auth/resend-verification", { method: "POST" }, session.csrfToken);
      setStatus("waiting");
      setNotice("Лист надіслано повторно. Перевір вхідні та папку «Спам».");
    } catch (error) {
      if (error instanceof OnboardingError && error.code === "email_already_verified") {
        checkVerification();
      } else if (error instanceof OnboardingError && error.status === 401) {
        setSession(null);
        setStatus("login");
      } else {
        setError("Не вдалося надіслати лист. Спробуй ще раз пізніше.");
      }
    } finally {
      resending.current = false;
      setPending(false);
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
            disabled={pending}
            onClick={() => void resend()}
          >
            {pending ? "Надсилаємо лист…" : "Надіслати лист повторно"}
          </button>
        </>
      )}
      {status === "error" && (
        <button className="register-submit" type="button" onClick={checkVerification}>
          Спробувати ще раз
        </button>
      )}
      {status !== "loading" && (
        <p className="register-form__login">
          <Link href="/login?next=/onboarding">Перейти до входу</Link>
        </p>
      )}
    </div>
  );
}
