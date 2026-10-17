"use client";

import Link from "next/link";
import { Logo } from "./Logo";
import { ProfileShell } from "./ProfileShell";
import { ProfileSummary } from "./ProfileSummary";
import { ProfileEditor } from "./ProfileEditor";
import { useProfile } from "./useProfile";

export function Profile() {
  const { profile, session, status, retry, setProfile } = useProfile();
  if (status !== "ready" || !profile || !session) {
    return (
      <main className="dashboard-message-page">
        <div className="dashboard-message" role={status === "loading" ? "status" : "alert"}>
          <Logo variant="app" />
          <h1>
            {status === "loading"
              ? "Завантажуємо профіль…"
              : status === "login"
                ? "Увійди, щоб продовжити"
                : "Не вдалося завантажити профіль"}
          </h1>
          <p>
            {status === "loading"
              ? "Готуємо твої збережені дані."
              : "Перевір з’єднання або увійди в акаунт знову."}
          </p>
          {status === "login" && (
            <Link
              className="dashboard-button dashboard-button--primary"
              href="/login?next=/profile"
            >
              Увійти
            </Link>
          )}
          {status === "error" && (
            <button
              className="dashboard-button dashboard-button--primary"
              type="button"
              onClick={retry}
            >
              Спробувати ще раз
            </button>
          )}
        </div>
      </main>
    );
  }
  return (
    <ProfileShell profile={profile} csrfToken={session.csrfToken}>
      <nav className="profile-tabs" aria-label="Розділи профілю">
        <a href="#profile-about" aria-current="page">
          Про мене
        </a>
      </nav>
      <div className="profile-columns">
        <div className="profile-sections" id="profile-about">
          <ProfileEditor profile={profile} csrfToken={session.csrfToken} onSaved={setProfile} />
          <section className="account-card">
            <h2>Пошта університету</h2>
            <p className="profile-email">{session.user.email}</p>
            <span className="profile-verified">✓ Підтверджено</span>
          </section>
        </div>
        <ProfileSummary profile={profile} />
      </div>
    </ProfileShell>
  );
}
